package greener

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type Logger interface {
	Logf(string, ...interface{})
	Errorf(string, ...interface{})
}

type DefaultLogger struct {
	logf func(string, ...interface{})
}

func (cl *DefaultLogger) Logf(m string, a ...interface{}) {
	cl.logf(m, a...)
}

func (cl *DefaultLogger) Errorf(m string, a ...interface{}) {
	cl.logf("ERROR: "+m, a...)
}

func NewDefaultLogger(logf func(string, ...interface{})) *DefaultLogger {
	return &DefaultLogger{logf: logf}
}

func AutoServe(logger Logger, mux *http.ServeMux) (context.Context, func()) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8000"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}
	host := os.Getenv("HOST")
	uds := os.Getenv("UDS")
	if uds == "" {
		mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		})
	}
	go Serve(ctx, logger, mux, host, port, uds)
	if uds == "" {
		healthURL := fmt.Sprintf("http://%s:%d/livez", host, port)
		if err = PollForHealth(healthURL, 2*time.Second, 20*time.Millisecond); err != nil {
			panic(err)
		}
	}
	return ctx, stop
}

func Serve(ctx context.Context, logger Logger, handler http.Handler, host string, port int, uds string) {
	addr := fmt.Sprintf("%s:%d", host, port)
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}
	if uds != "" {
		listener, err := net.Listen("unix", uds)
		if err != nil {
			logger.Logf("Error listening: %v", err)
			return
		}
		logger.Logf("Server listening on Unix Domain Socket: %s", uds)
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			logger.Logf("Server closed with error: %v", err)
			return
		}
	} else {
		logger.Logf("Server listening on %s", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Logf("Server closed with error: %v", err)
			return
		}
	}
}
