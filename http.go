package simplehttp

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http" // for http.StatusText only: 都是硬编码，重写一遍太蠢了
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DebugPanicResponse response a 500 status with the error message
	// when a panic occurs and handler is not handling it.
	// This is for debugging purpose.
	// In production, set it to false, which makes it response 500 without any
	// message, avoiding leaking sensitive information.
	DebugPanicResponse = true

	// ReadRequestLineTimeout in milliseconds
	ReadRequestLineTimeout = 3000 * time.Millisecond
	// ReadRequestBodyTimeout in milliseconds
	ReadRequestBodyTimeout = 3000 * time.Millisecond
)

// region Request

// Request is an HTTP request.
// The zero value is NOT valid, call NewRequest() to get a valid response.
type Request struct {
	Method  string
	Url     string
	Version string

	Headers map[string]string
	Body    io.Reader
}

func NewRequest() *Request {
	return &Request{
		Headers: make(map[string]string),
	}
}

// Parse HTTP Request from a tcp conn
func (r *Request) Parse(conn io.Reader) error {
	scanner := bufio.NewScanner(conn)
	scanner.Split(bufio.ScanBytes)

	// parse the request line
	line, err := scanBytesToLine(scanner, ReadRequestLineTimeout)
	if err != nil {
		return err
	}
	_, err = fmt.Sscanf(line, "%s %s %s", &r.Method, &r.Url, &r.Version)
	if err != nil {
		return err
	}

	// parse the headers
	for {
		line, err := scanBytesToLine(scanner, ReadRequestLineTimeout)
		if err != nil {
			return err
		}

		if line == "" { // empty line, end of headers
			break
		}

		if key, value, err := lineToKV(line); err != nil {
			return err
		} else {
			r.Headers[key] = value
		}
	}

	// read the body
	// TODO: lazy read: a wrapper of scanner => io.Reader

	buf := &bytes.Buffer{}
	r.Body = buf

	// The question is hwo to determine the body length.
	// TODO: Transfer-Encoding supports (HTTP/1.1)
	// Reference to RFC 9112 (HTTP/1.1) Section 6.

	l, ok := r.Headers["Content-Length"]
	if !ok { // no body
		return nil
	}

	length, err := strconv.Atoi(l)
	if err != nil {
		return err
	}

	return scanBytesToBuffer(scanner, buf, length, ReadRequestBodyTimeout)
}

// scanBytesToLine scan a http line from the scanner (split by bufio.ScanBytes).
// For Request.Parse use only.
func scanBytesToLine(scanner *bufio.Scanner, timeout time.Duration) (string, error) {
	chStr := make(chan string)
	chErr := make(chan error)

	go func() {
		s := strings.Builder{}
		for scanner.Scan() {
			if scanner.Err() != nil {
				chErr <- scanner.Err()
				return
			}

			b := scanner.Bytes()
			if b[0] == '\r' {
				if scanner.Scan() && scanner.Bytes()[0] == '\n' { // CRLF
					break
				} else {
					s.Write(b)
					s.Write(scanner.Bytes())
					continue
				}
			}
			s.Write(b)
		}
		chStr <- s.String()
	}()

	select {
	case line := <-chStr:
		return line, nil
	case err := <-chErr:
		return "", err
	case <-time.After(timeout):
		return "", errors.New("scanBytesToLine: timeout")
	}
}

// scanBytesToBuffer copy bytes from scanner to buffer.
// For Request.Parse use only.
func scanBytesToBuffer(scanner *bufio.Scanner, buf *bytes.Buffer, length int, timeout time.Duration) error {
	chDone := make(chan struct{})
	chErr := make(chan error)

	go func() {
		for buf.Len() < length && scanner.Scan() {
			if scanner.Err() != nil {
				chErr <- scanner.Err()
				return
			}
			buf.Write(scanner.Bytes())
			if scanner.Err() != nil {
				break
			}
		}
		chDone <- struct{}{}
	}()

	select {
	case <-chDone:
		return nil
	case err := <-chErr:
		return err
	case <-time.After(timeout):
		return errors.New("scanBytesToBuffer: timeout")
	}
}

// lineToKV parse a line to key-value pair.
// For Request.Parse use only.
func lineToKV(line string) (string, string, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", errors.New("invalid header line")
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	return key, value, nil
}

// endregion Request

// region Response

// Lener is the interface that wraps the Len method.
type Lener interface {
	Len() int
}

type ResponseWriter interface {
	io.Writer
	io.StringWriter
	Lener
	io.Reader // it's actually a ReadWriter but let's "hide" the Read() method to avoid misuse
}

// Response is the HTTP response.
// The zero value is NOT valid, call NewResponse() to get a valid response.
type Response struct {
	Version string
	Status  int
	Reason  string

	Headers map[string]string
	Body    ResponseWriter
}

func NewResponse() *Response {
	return &Response{
		Headers: make(map[string]string),
		Body:    &bytes.Buffer{},
	}
}

// write response to conn
// TODO: error handling
func (r *Response) write(conn io.Writer) error {
	// write status line
	_, err := fmt.Fprintf(conn, "%s %d %s\r\n", r.Version, r.Status, r.Reason)

	// let's calculate the real content length
	r.Headers["Content-Length"] = fmt.Sprintf("%d", r.Body.Len())

	// write headers
	for k, v := range r.Headers {
		_, err = fmt.Fprintf(conn, "%s: %s\r\n", k, v)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(conn, "\r\n")

	// write body
	_, err = io.Copy(conn, r.Body)

	return err

	// An example response that works:
	//_, _ = conn.write([]byte("HTTP/1.0 200 OK\r\n"))
	//_, _ = conn.write([]byte("Content-Length: 2\r\n"))
	//_, _ = conn.write([]byte("Content-Type:text/html:charset=UTF-8\r\n\r\n"))
	//_, _ = conn.write([]byte("OK"))
}

// SetStateLine set the state line of the response.
// e.g. HTTP/1.1 200 OK
// The status reason is inferred from the status code.
func (r *Response) SetStateLine(version string, status int) {
	r.Version = version
	r.Status = status
	r.Reason = http.StatusText(status)
}

// endregion Response

// region Context

// Context of a http transaction (request -> handle -> response).
//
//	Context = Request + Response
//
// NOTE: simplehttp.Context is not context.Context.
type Context struct {
	Request  *Request
	Response *Response

	values              sync.Map
	handlers            []Handler
	currentHandlerIndex int
}

func NewContext(request *Request, response *Response) *Context {
	return &Context{
		Request:             request,
		Response:            response,
		values:              sync.Map{},
		handlers:            []Handler{},
		currentHandlerIndex: -1,
	}
}

// Set a value in the context.
func (c *Context) Set(key string, value interface{}) {
	c.values.Store(key, value)
}

// Get a value from the context.
func (c *Context) Get(key string) (value interface{}, ok bool) {
	return c.values.Load(key)
}

// Delete a value from the context.
func (c *Context) Delete(key string) {
	c.values.Delete(key)
}

// ResponseText makes a response with status and plain text as body.
func (c *Context) ResponseText(status int, text string) {
	c.Response.SetStateLine(c.Request.Version, status)
	c.Response.Headers["Content-Type"] = "text/plain; charset=utf-8"
	_, _ = c.Response.Body.Write([]byte(text))
}

// ResponseHTML makes a response with status and html as body.
func (c *Context) ResponseHTML(status int, html string) {
	c.Response.SetStateLine(c.Request.Version, status)
	c.Response.Headers["Content-Type"] = "text/html; charset=utf-8"
	_, _ = c.Response.Body.Write([]byte(html))
}

// ResponseJSON makes a response with status and JSON as body.
func (c *Context) ResponseJSON(status int, v interface{}) {
	c.Response.SetStateLine(c.Request.Version, status)
	c.Response.Headers["Content-Type"] = "application/json; charset=utf-8"
	enc := json.NewEncoder(c.Response.Body)
	_ = enc.Encode(v)
}

// Chain makes a handler chain, returns a handler,
// call which will start the chain.
//
// NOTE: This is obviously not efficient, just for fun.
// For any scenario, use setChain() and Next() instead.
//
// I prefer to rename this to "HandlerWithMiddleware",
// but it's too long to be cool.
func Chain(h Handler, middlewares ...Handler) Handler {
	return HandlerFunc(func(c *Context) { // the INIT_MIDDLEWARE
		for _, m := range middlewares {
			c.handlers = append(c.handlers, m)
		}
		c.handlers = append(middlewares, h)
		c.Next()
	})
}

// setChain to a Context, call Next() method to start the chain.
func (c *Context) setChain(chain []Handler) {
	c.handlers = chain
}

// Next call the next handler (middleware) in the chain.
// A middleware should call Next() exactly once to continue the chain.
// the last handler, i.e. the real handler, should not call Next().
func (c *Context) Next() {
	c.currentHandlerIndex++
	for c.currentHandlerIndex < len(c.handlers) {
		// loop in case of a middleware not calling Next()
		c.handlers[c.currentHandlerIndex].ServeHTTP(c)
		c.currentHandlerIndex++
	}
}

// endregion Context

// region Handler

type Handler interface {
	ServeHTTP(c *Context)
}

// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as HTTP handlers. If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler that calls f.
type HandlerFunc func(c *Context)

func (f HandlerFunc) ServeHTTP(c *Context) {
	f(c)
}

// endregion Handler

// region Server

// TODO: HTTP/1.1 keep-alive

type Server interface {
	SetHandler(handler Handler)
	ListenAndServe(addr string) error
	// ListenAndServeTLS : 现代 HTTP 服务器支持 SSL 难道还不是标配嘛。。
	ListenAndServeTLS(addr, certFile, keyFile string) error
}

// puts Http & Https server together, sharing the handler:
//    go httpServer.ListenAndServe(addr)
//    go httpServer.ListenAndServeTLS(addr, certFile, keyFile)

type HttpServer struct {
	Handler Handler
}

func (s *HttpServer) SetHandler(handler Handler) {
	s.Handler = handler
}

// handleConn parse a request, create the context,
// handle it with s.Handler, write response back, and close conn.
// NOTE: handleConn is not a Handler.
func (s *HttpServer) handleConn(conn net.Conn) {
	// TODO: HTTP/1.1 keep-alive
	defer conn.Close()

	// data flow: request -> context -> handler -> response

	request := NewRequest()

	response := NewResponse()
	defer response.write(conn)

	defer func() { // something wrong and not handled by the handler
		if err := recover(); err != nil {
			// let's try to response a 500, but it's not guaranteed
			response.Version = "HTTP/1.0"
			response.Status = 500
			response.Reason = "Internal Server Error"
			if DebugPanicResponse {
				_, _ = response.Body.Write([]byte(
					fmt.Sprintf("panic: %v", err)))
			}

			_ = response.write(conn)

			// and throw the panic again
			panic(err)
		}
	}()

	ctx := NewContext(request, response)

	// parse request
	if err := request.Parse(conn); err != nil {
		response.Version = "HTTP/1.0"
		response.Status = 400
		response.Reason = "Bad Request"
	}

	// handle request
	s.Handler.ServeHTTP(ctx)

	// deferred: write response and close conn
}

// ListenAndServe listen addr, and serve HTTP: handle conn with s.Handler
func (s *HttpServer) ListenAndServe(addr string) error {
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	for {
		conn, _ := listen.Accept()
		go s.handleConn(conn)
	}
}

// ListenAndServeTLS listen addr, and serve HTTPS: handle conn with s.Handler
func (s *HttpServer) ListenAndServeTLS(addr, certFile, keyFile string) error {
	cer, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	listen, err := tls.Listen("tcp", addr, config)
	if err != nil {
		return err
	}

	for {
		conn, _ := listen.Accept()
		go s.handleConn(conn)
	}
}

// endregion Server
