package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	host := flag.String("host", "", "host to proxy request to")
	hostPort := flag.Int("host-port", 443, "port to proxy request to")
	scheme := flag.String("scheme", "https", "scheme to use")
	port := flag.Int("port", 9090, "port to bind reverse proxy to")
	flag.Parse()

	// Signal handler
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	remote, err := url.Parse(fmt.Sprintf("%v://%v:%v", *scheme, *host, *hostPort))
	if err != nil {
		os.Exit(1)
	}

	// Setup proxy server
	proxy := httputil.NewSingleHostReverseProxy(remote)
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(proxyHandler(proxy, *host))
	srv := &http.Server{Addr: fmt.Sprintf(":%v", *port), Handler: router}

	// Start proxy server
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println(err)
		}
	}()
	fmt.Println("Started reverse proxy")

	// Blocks until singal is sent
	<-done

	// Shutdown proxy server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Stopped reverse proxy")
}

func proxyHandler(p *httputil.ReverseProxy, host string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Proxying traffic: %v", r)
		r.Host = host
		p.ServeHTTP(w, r)
	}
}
