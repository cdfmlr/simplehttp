package simplehttp

import (
	"errors"
	"fmt"
	"io"
	"os"
	pathLib "path"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	IndexFile = "index.html"
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

// FileServer is a Handler that serves HTTP requests with the contents of the
// file system rooted at root. Supports HTTP/1.1 Range ([RFC 9110 Section 14]).
//
//	GET prefix/dir/index.html
//	=>  root/dir/index.html
//
// [RFC 9110 Section 14]: https://www.rfc-editor.org/rfc/rfc9110#name-range-requests
func FileServer(root string, prefix string) HandlerFunc {
	return func(c *Context) {
		path := strings.TrimPrefix(c.Request.Url, prefix)
		if strings.HasSuffix(path, "/") {
			path = pathLib.Join(path, IndexFile)
		}
		path = pathLib.Join(root, path)
		// fmt.Println("FileServer:", path, c.Request)

		serveFile(c, path)
	}
}

// serveFile writes the contents of the file to the c,
// with HTTP/1.1 Range supports.
func serveFile(c *Context, path string) {
	// open file
	f, err := os.Open(path)
	if err != nil {
		c.ResponseText(404, "Not Found")
		return
	}
	defer f.Close()

	// get file info
	stat, err := f.Stat()
	if errors.Is(err, os.ErrNotExist) {
		c.ResponseText(404, "Not Found")
		return
	}

	// check file stat
	if errors.Is(err, os.ErrPermission) {
		c.ResponseText(403, "Forbidden")
		return
	}
	if err != nil {
		c.ResponseText(500, "Internal Server Error")
		return
	}
	if stat.IsDir() {
		c.ResponseText(403, "not a file")
		return
	}

	// HTTP/1.1 Range support
	var start, end int64 = 0, stat.Size()
	rangeHeader, isRange := c.Request.Headers["Range"]
	if isRange {
		c.Response.Headers["Accept-Ranges"] = "bytes"
		s, e, err := parseRange(c, rangeHeader, stat.Size())
		if err != nil {
			return
		}
		start, end = s, e
	}
	//  else if end-start > 1024*1024 { // 1MB
	// 	// c.Response.SetStateLine()
	// 	c.Response.Headers["Accept-Ranges"] = "bytes"
	// }

	// stop,, 不知道为什么 Safari 要加这个才能正常工作
	if start == end {
		c.Response.SetStateLine(c.Request.Version, 200)
		c.Response.Headers["Content-Type"] = mimeType(path)
		c.Response.Headers["Content-Length"] = "0"
		return
	}

	if start < 0 || end > stat.Size() {
		c.ResponseText(416, "range not satisfiable")
		return
	}

	// set response headers
	if isRange {
		c.Response.SetStateLine(c.Request.Version, 206)
		c.Response.Headers["Content-Range"] = fmt.Sprintf("bytes %d-%d/%d", start, end, stat.Size())
		c.Response.Headers["Accept-Ranges"] = "bytes"
	} else {
		c.Response.SetStateLine(c.Request.Version, 200)
	}

	c.Response.Headers["Content-Length"] = fmt.Sprintf("%d", end-start)
	c.Response.Headers["Content-Type"] = mimeType(path)

	// write response body: file content
	_, _ = f.Seek(start, io.SeekStart)
	_, _ = io.CopyN(c.Response.Body, f, end-start)
}

// parseRange: bytes=0-100
func parseRange(c *Context, rangeHeader string, fileSize int64) (start, end int64, err error) {
	rangeRegexp := regexp.MustCompile(`bytes=(\d*)\-(\d*)`)

	if !rangeRegexp.MatchString(rangeHeader) {
		c.ResponseText(400, "bad request")
		return 0, 0, errors.New("bad request")
	}

	matches := rangeRegexp.FindStringSubmatch(rangeHeader)
	// fmt.Println(matches)
	if len(matches) < 3 { // ["bytes=0-1", "0", "1"]
		c.ResponseText(400, "bad request")
		return 0, 0, errors.New("bad request")
	}

	// let's parse end first, and then start, make it easier
	if matches[2] != "" {
		end, _ = strconv.ParseInt(matches[2], 10, 64)
	} else { // bytes=9500-  =>  The final 500 bytes (byte offsets 9500-9999, inclusive)
		end = fileSize - 1
	}
	end += 1 // end is exclusive

	if matches[1] != "" {
		start, _ = strconv.ParseInt(matches[1], 10, 64)
	} else { // bytes=-500  =>  The final 500 bytes (byte offsets 9500-9999, inclusive)
		start = fileSize - end
	}

	return start, end, nil
}

func mimeType(path string) string {
	switch pathLib.Ext(path) {
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".txt":
		return "text/plain"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

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
