package main

import (
	"log"
	"net"
	"net/http"

	"net/http/fcgi"
	// "github.com/hillu/go-fcgi-breakage/fcgi"
)

func main() {
	l, err := net.Listen("tcp", "localhost:8081")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	fcgi.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("-> %s %s (%s)", r.Method, r.URL, r.Proto)
		http.DefaultServeMux.ServeHTTP(w, r)
		log.Print("<-")
	}))
}
