package main

import (
	"fmt"
	"simplehttp"
)

func main() {
	chain := simplehttp.Chain(simplehttp.EchoHandler, simplehttp.Logger, simplehttp.Recovery)
	//chain := simplehttp.Chain(simplehttp.HandlerFunc(func(c *simplehttp.Context) {
	//	c.ResponseText(200, "ResponseText: OK")
	//	//c.ResponseJSON(200, map[string]interface{}{
	//	//	"code":    200,
	//	//	"msg":     "ResponseJSON: OK",
	//	//	"request": c.Request,
	//	//})
	//}), simplehttp.Logger, simplehttp.Recovery)

	s := simplehttp.HttpServer{
		Handler: chain,
	}

	fmt.Printf("Listen and serve on %v\n", ":20223")
	s.ListenAndServe(":20223")
	//s.ListenAndServeTLS(":20225", "/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server/keys/cnlab.cert", "/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server/keys/cnlab.prikey")
}
