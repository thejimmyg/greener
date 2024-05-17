package greener

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
)

// Logger interface
type Logger interface {
	Logf(string, ...interface{})
	Errorf(string, ...interface{})
}

// ServeConfigProvider interface for server configuration handling
type ServeConfigProvider interface {
	Host() string
	Port() int
	UDS() string
}

// DefaultLogger implements Logger.
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

// DefaultServeConfigProvider implments ServeConfigProvider for returning the configuration needed for serving the app
type DefaultServeConfigProvider struct {
	host string
	port int
	uds  string
}

func (dscp *DefaultServeConfigProvider) Host() string {
	return dscp.host
}
func (dscp *DefaultServeConfigProvider) Port() int {
	return dscp.port
}
func (dscp *DefaultServeConfigProvider) UDS() string {
	return dscp.uds
}

func NewDefaultServeConfigProviderFromEnvironment() *DefaultServeConfigProvider {
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8000"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}
	return &DefaultServeConfigProvider{host: os.Getenv("HOST"), port: port, uds: os.Getenv("UDS")}
}

func Serve(ctx context.Context, logger Logger, handler http.Handler, config ServeConfigProvider) {
	addr := fmt.Sprintf("%s:%d", config.Host(), config.Port())

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}
	// Listen for the context cancellation in a separate goroutine
	go func() {
		<-ctx.Done() // This blocks until the context is cancelled

		logger.Logf("Shutting down server...")
		if err := srv.Shutdown(context.Background()); err != nil {
			logger.Logf("Server shutdown failed: %v", err)
		}
	}()
	if config.UDS() != "" {
		listener, err := net.Listen("unix", config.UDS())
		if err != nil {
			logger.Logf("Error listening: %v", err)
			return
		}
		logger.Logf("Server listening on Unix Domain Socket: %s", config.UDS())
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
