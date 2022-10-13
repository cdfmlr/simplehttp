//go:build ignore

package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
)

func demoHandler(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		if line == "" {
			break
		}
	}
	fmt.Println("---")

	_, _ = conn.Write([]byte("HTTP/1.0 200 OK\r\n"))
	_, _ = conn.Write([]byte("Content-Length: 2\r\n"))
	_, _ = conn.Write([]byte("Content-Type:text/html:charset=UTF-8\r\n\r\n"))
	_, _ = conn.Write([]byte("OK"))
}

func demoHTTP(addr string) {
	listen, _ := net.Listen("tcp", addr)
	for {
		conn, _ := listen.Accept()
		go demoHandler(conn)
	}
}

func demoHTTPS(addr string, certFile, keyFile string) {
	cer, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		fmt.Println(err)
		return
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	listen, _ := tls.Listen("tcp", addr, config)
	for {
		conn, _ := listen.Accept()
		go demoHandler(conn)
	}
}

func main() {
	//demoHTTP(":20220")

	keysDir := "/Users/c/Learning/ucas2022fall/cn/all-exps/05-http_server/keys/"
	demoHTTPS(":20221", keysDir+"cnlab.cert", keysDir+"cnlab.prikey")
}
