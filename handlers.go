package simplehttp

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"
)

// region Handler: echo

// EchoHandler is a simple handler that echoes the request
var EchoHandler HandlerFunc = echoHandler

func echoHandler(c *Context) {
	c.Response.Version = c.Request.Version
	c.Response.Status = 200
	c.Response.Reason = "OK"
	c.Response.Headers["Content-Type"] = "text/plain"

	// body:

	// write request line
	_, _ = fmt.Fprintf(c.Response.Body,
		"%v %v %v\r\n", c.Request.Method, c.Request.Url, c.Request.Version)

	// write request headers
	for k, v := range c.Request.Headers {
		_, _ = fmt.Fprintf(c.Response.Body, "%v: %v\r\n", k, v)
	}
	_, _ = c.Response.Body.Write([]byte("\r\n"))

	// write request body
	body, _ := io.ReadAll(c.Request.Body)
	_, _ = c.Response.Body.Write(body)
}

// endregion Handler: echo

// region Handler: FileServer

// TODO: FileServer is a Handler

// endregion  Handler: FileServer

// region Handler: CGIServer

// TODO: CGIServer is a Handler

// endregion  Handler: CGIServer

// region Middleware: Recover

// Recovery is a middleware that recovers from panic
// and writes a 500 response
var Recovery HandlerFunc = recovery

func recovery(c *Context) {
	defer func() {
		if err := recover(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "recovered from panic:", err)
			debug.PrintStack()

			c.Response.Version = c.Request.Version // "HTTP/1.0"
			c.Response.Status = 500
			c.Response.Reason = "Internal Server Error"
			_, _ = c.Response.Body.Write([]byte(
				fmt.Sprintf("panic: %v", err)))
		}
	}()

	c.Next()
}

// endregion Middleware: Recover

// region Middleware: Logger

// Logger is a middleware that logs the request
var Logger HandlerFunc = logger

func logger(c *Context) {
	st := time.Now()
	c.Next()
	et := time.Now()

	fmt.Printf("%v: %v %v => %v (%v)\n",
		st.Format(time.RFC3339Nano), c.Request.Method, c.Request.Url, c.Response.Status, et.Sub(st))
}

// endregion Middleware: Logger
