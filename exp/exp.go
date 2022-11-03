package main

import (
	"fmt"
	"net/url"
	"os"
	"simplehttp"
)

const (
	Cert = "/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server/keys/cnlab.cert"
	Pkey = "/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server/keys/cnlab.prikey"
)

const (
	DoExpMininet = false

	MininetVmCert   = "/shared/cn/all-exps/05-http_server/keys/cnlab.cert"
	MininetVmPkey   = "/shared/cn/all-exps/05-http_server/keys/cnlab.prikey"
	MininetVmFsRoot = "/shared/cn/all-exps/05-http_server"

	MininetLocalCert   = Cert
	MininetLocalPkey   = Pkey
	MininetLocalFsRoot = "/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server"
)

type expMininetConfig struct {
	Cert   string
	Pkey   string
	FsRoot string
}

// expMininetEnv => {cert, pkey, fsroot} path
var expMininetConfigs = map[string]expMininetConfig{
	"local": {
		Cert:   MininetLocalCert,
		Pkey:   MininetLocalPkey,
		FsRoot: MininetLocalFsRoot,
	},
	"vm": {
		Cert:   MininetVmCert,
		Pkey:   MininetVmPkey,
		FsRoot: MininetVmFsRoot,
	},
}

func expEcho() {
	r := simplehttp.NewPrefixRouter("/")

	r.Use(simplehttp.Logger, simplehttp.Recovery)
	r.GET("/echo/", simplehttp.EchoHandler)

	s := simplehttp.HttpServer{Handler: r}

	go s.ListenAndServe(":20000")
	go s.ListenAndServeTLS(":20001", Cert, Pkey)

	select {}
}

func expFile() {
	r := simplehttp.NewPrefixRouter("/")

	r.Use(simplehttp.Logger, simplehttp.Recovery)

	r.GET("/static/", simplehttp.FileServer(
		"/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server",
		"/static",
	))

	s := simplehttp.HttpServer{Handler: r}

	go s.ListenAndServe(":20100")
	go s.ListenAndServeTLS(":20101", Cert, Pkey)

	select {}
}

func expMininet(config expMininetConfig) {
	srvHttp := simplehttp.HttpServer{ // 301 to https
		Handler: simplehttp.HandlerFunc(func(c *simplehttp.Context) {
			fmt.Println("301 to https: ",
				c.Request.Url, c.Request.Headers["Host"])
			c.Response.SetStateLine(c.Request.Version, 301)
			c.Response.Headers["Location"], _ = url.JoinPath(
				"https://", c.Request.Headers["Host"], c.Request.Url,
			)
		}),
	}
	srvHttps := simplehttp.HttpServer{ // static file server
		Handler: simplehttp.Chain(
			simplehttp.FileServer(config.FsRoot, "/"),
			simplehttp.Logger,
		),
		// Handler: simplehttp.FileServer(config.FsRoot, "/"),
	}

	go srvHttp.ListenAndServe(":80")
	go srvHttps.ListenAndServeTLS(":443", config.Cert, config.Pkey)

	select {}
}

func expPerf() {
	handler := simplehttp.FileServer(
		"/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server",
		"/static",
	)

	// blank handler without any middleware
	srvBlank := simplehttp.HttpServer{Handler: handler}

	r := simplehttp.NewPrefixRouter("/")
	r.Use(simplehttp.Logger, simplehttp.Recovery)
	r.GET("/static/", handler)

	// router + handler with middlewares
	srvDefalut := simplehttp.HttpServer{Handler: r}

	go srvBlank.ListenAndServe(":20200")
	go srvBlank.ListenAndServeTLS(":20201", Cert, Pkey)
	go srvDefalut.ListenAndServe(":20202")
	go srvDefalut.ListenAndServeTLS(":20203", Cert, Pkey)

	select {}
}

func main() {
	if DoExpMininet {
		expMininetEnv, ok := os.LookupEnv("EXP_MININET_ENV")
		if !ok {
			fmt.Println("EXP_MININET_ENV not set: use vm by default")
			expMininetEnv = "vm"
		}
		if config, ok := expMininetConfigs[expMininetEnv]; ok {
			fmt.Printf("DoExpMininet (env: %v) \n", expMininetEnv)
			expMininet(config)
		} else {
			fmt.Println("invalid EXP_MININET_ENV: ", expMininetEnv)
			os.Exit(1)
		}
		return
	}

	// go expEcho()
	// go expFile()
	go expPerf()

	select {}
}
