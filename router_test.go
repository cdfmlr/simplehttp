package simplehttp

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const testPrefixRouter = 22530

func TestPrefixRouter(t *testing.T) {
	port := testPrefixRouter
	// server
	go func() {
		r := NewPrefixRouter("/")
		r.Use(Logger, Recovery)

		r.GET("/hello", func(c *Context) {
			c.ResponseText(200, "/hello")
		})

		r.POST("/hello", func(c *Context) {
			c.ResponseText(200, "POST")
		})

		r.GET("/hello/", func(c *Context) {
			c.ResponseText(200, "/hello/")
		})
		r.GET("/hello/world/", func(c *Context) {
			c.ResponseHTML(200, "/hello/world/")
		})

		r.HandleFunc(MethodAny, "/panic", func(c *Context) {
			panic("I'm panic!")
		})

		s := HttpServer{Handler: r}
		if err := s.ListenAndServe(fmt.Sprintf(":%d", port)); err != nil {
			panic(err)
		}
	}()

	// client

	cases := []struct {
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{"GET", "/", 404, ""},
		{"GET", "/whatever", 404, ""},

		{"GET", "/hello", 200, "/hello"},
		{"GET", "/hello?x=1", 200, "/hello"},

		{"POST", "/hello", 200, "POST"},

		{"GET", "/hello/", 200, "/hello/"},
		{"GET", "/hello/?x=1", 200, "/hello/"},

		{"GET", "/hello/world", 200, "/hello/"},
		{"GET", "/hello/world/", 200, "/hello/world/"},
		{"GET", "/hello/world/?x=1", 200, "/hello/world/"},
		{"GET", "/hello/world/foo", 200, "/hello/world/"},
		{"GET", "/hello/world/foo/bar?x=1", 200, "/hello/world/"},

		{"GET", "/panic", 500, "panic: I'm panic!"},
	}

	time.Sleep(1 * time.Second)

	for _, tt := range cases {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			testRouter(t, port, tt.method, tt.path, tt.expectedStatus, tt.expectedBody)
		})
	}
}

func testRouter(t *testing.T, port int, method string, path string, expectedStatus int, expectedBody string) {
	req, err := http.NewRequest(method,
		fmt.Sprintf("http://localhost:%d%s", port, path), nil)
	if err != nil {
		t.Error(err)
	}
	got, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
	}
	if got.StatusCode != expectedStatus {
		t.Errorf("expected status code %d, got %d",
			expectedStatus, got.StatusCode)
	}
	body, err := io.ReadAll(got.Body)
	if err != nil {
		t.Error("read body", err)
	}
	if string(body) != expectedBody {
		t.Errorf("expected %s, got %s", expectedBody, string(body))
	}
	t.Logf("✅ %v %v => (%v) %v",
		got.Request.Method, got.Request.URL, got.StatusCode, string(body))
}
