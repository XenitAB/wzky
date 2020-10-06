// +build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

func main() {
	host := flag.String("host", "", "host to proxy request to")
	hostPort := flag.Int("host-port", 443, "port to proxy request to")
	scheme := flag.String("scheme", "https", "scheme to use")
	port := flag.Int("port", 9090, "port to bind reverse proxy to")
	serviceName := flag.String("service-name", "wzky", "name of windows service")
	isDebug := flag.Bool("debug", false, "enable debug mode")
	flag.Parse()

	remote, err := url.Parse(fmt.Sprintf("%v://%v:%v", *scheme, *host, *hostPort))
	if err != nil {
		os.Exit(1)
	}

	// Start windows service
	log, err := getLog(*serviceName, *isDebug)
	if err != nil {
		log.Error(1, fmt.Sprintf("%s service failed: %v", *serviceName, err))
		return
	}
	defer log.Close()

	log.Info(1, fmt.Sprintf("starting %s service", *serviceName))
	run := svc.Run
	if *isDebug {
		run = debug.Run
	}

	s := &service{
		log:    log,
		remote: remote,
		port:   *port,
	}
	err = run(*serviceName, s)
	if err != nil {
		log.Error(1, fmt.Sprintf("%s service failed: %v", *serviceName, err))
		return
	}

	log.Info(1, fmt.Sprintf("%s service stopped", *serviceName))
}

func getLog(name string, isDebug bool) (debug.Log, error) {
	if isDebug {
		return debug.New(name), nil
	}

	log, err := eventlog.Open(name)
	if err != nil {
		return nil, err
	}

	return log, nil
}

type service struct {
	log    debug.Log
	remote *url.URL
	port   int
}

func (s *service) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	// Setup proxy server
	proxy := httputil.NewSingleHostReverseProxy(s.remote)
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(proxyHandler(proxy, s.remote.Host))
	srv := &http.Server{Addr: fmt.Sprintf(":%v", s.port), Handler: router}

	// Start proxy server
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error(1, fmt.Sprintf("could not start server: %v", err))
		}
	}()
	s.log.Info(1, "started reverse proxy")

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			default:
				s.log.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}

	// Shutdown proxy server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()
	if err := srv.Shutdown(ctx); err != nil {
		s.log.Error(1, fmt.Sprintf("could not stop proxy server: %v", err))
		return
	}
	s.log.Info(1, "stopped reverse proxy")

	return
}

func proxyHandler(p *httputil.ReverseProxy, host string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Host = host
		p.ServeHTTP(w, r)
	}
}
