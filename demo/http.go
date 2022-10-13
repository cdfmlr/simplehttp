//go:build ignore

package main

import (
	"fmt"
	"io"
	"net/http"
)

func main() {
	//demoHTTP(":20220")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Header().Add("Content-Type", "text/plain")
		fmt.Fprintf(w, "%v %v %v\r\n", r.Method, r.URL.String(), "HTTP/1.1")
		for k, v := range r.Header {
			w.Header().Add(k, fmt.Sprint(v))
		}
		io.Copy(w, r.Body)
	})
	http.ListenAndServe(":20224", nil)
}
