package main

import (
	"fmt"
	"simplehttp"
)

func testHttp() {
	//chain := simplehttp.Chain(simplehttp.EchoHandler, simplehttp.Logger, simplehttp.Recovery)
	chain := simplehttp.Chain(simplehttp.HandlerFunc(func(c *simplehttp.Context) {
		c.ResponseText(200, "ResponseText: OK from HTTP")
		//c.ResponseJSON(200, map[string]interface{}{
		//	"code":    200,
		//	"msg":     "ResponseJSON: OK",
		//	"request": c.Request,
		//})
	}), simplehttp.Logger, simplehttp.Recovery)

	s := simplehttp.HttpServer{
		Handler: chain,
	}

	fmt.Printf("[TestHttp] Listen and serve on %v\n", ":20223")
	err := s.ListenAndServe(":20223")
	if err != nil {
		panic(err)
	}
}

func testHttps() {
	chain := simplehttp.Chain(simplehttp.HandlerFunc(func(c *simplehttp.Context) {
		c.ResponseText(200, "ResponseText: OK from HTTPS")
	}), simplehttp.Logger, simplehttp.Recovery)

	s := simplehttp.HttpServer{
		Handler: chain,
	}

	fmt.Printf("[testHttps] Listen and serve on %v\n", ":20225")
	err := s.ListenAndServeTLS(":20225", "/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server/keys/cnlab.cert", "/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server/keys/cnlab.prikey")
	if err != nil {
		panic(err)
	}
}

func testRouter() {
	r := simplehttp.NewPrefixRouter("/")
	r.Use(simplehttp.Logger, simplehttp.Recovery)

	r.GET("/hello", func(c *simplehttp.Context) {
		c.ResponseJSON(200, map[string]interface{}{
			"code":      200,
			"want_path": "/hello",
			"got_path":  c.Request.Url,
		})
	})

	r.GET("/hello/", func(c *simplehttp.Context) {
		c.ResponseText(200, "Hello from"+c.Request.Url)
	})
	r.GET("/hello/world/", func(c *simplehttp.Context) {
		c.ResponseHTML(200, "<h1>Hello, World!</h1>\n path: /hello/world[/...]")
	})

	r.GET("/echo/", simplehttp.EchoHandler)

	r.GET("panic", func(c *simplehttp.Context) {
		panic("I'm panic!")
	})

	s := simplehttp.HttpServer{Handler: r}
	fmt.Printf("[testRouter] Listen and serve on %v\n", ":20226")
	if err := s.ListenAndServe(":20226"); err != nil {
		panic(err)
	}
}

func testFileServer() {
	r := simplehttp.NewPrefixRouter("/")
	r.Use(simplehttp.Logger, simplehttp.Recovery)

	r.GET("/static/", simplehttp.FileServer(
		"/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server",
		"/static",
	))

	s := simplehttp.HttpServer{Handler: r}
	fmt.Printf("[testFileServer] Listen and serve on %v\n", ":20227")
	if err := s.ListenAndServe(":20227"); err != nil {
		panic(err)
	}
}

func main() {
	// go testHttp()
	// go testHttps()
	// go testRouter()
	go testFileServer()

	select {}
}
