package greener_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thejimmyg/greener"
)

// Logging
type Logger interface {
	Logf(string, ...any)
}
type Log struct{}

func (l *Log) Logf(m string, a ...any) {
	log.Printf(m, a...)
}

// Settings
type SettingGetter interface {
	Get(string, string) string
}
type Env struct{}

func (e *Env) Get(name string, ifEmpty string) string {
	value := os.Getenv(name)
	if value == "" {
		return ifEmpty
	}
	return value
}

func Get(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Making the GET request using the custom client
	response, err := client.Do(req)

	if err != nil {
		return "", err
	}
	defer response.Body.Close() // Ensure the response body is closed

	// Reading the response body
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	// Converting the response body to a string (assuming it's text-based)
	return string(responseBody), nil
}

func ServeUDS(logger Logger, uds string, srv *http.Server) {
	// UNIX Domain Socket support
	listener, err := net.Listen("unix", uds)
	if err != nil {
		logger.Logf("Error listening: %v", err)
		os.Exit(1)
	}
	logger.Logf("Server listening on Unix Domain Socket: %s", uds)
	if err := srv.Serve(listener); err != http.ErrServerClosed {
		logger.Logf("Server closed with error: %v", err)
		os.Exit(1)
	}
}
func ServeHTTP(logger Logger, addr string, srv *http.Server) {

	// HTTP support
	logger.Logf("Server listening on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Logf("Server closed with error: %v", err)
	}
}

type Hello struct{}

func (h *Hello) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, world!"))
}

type Health struct{}

func (h *Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

// Demonstrates how to set up and run a server
func Example_server() {
	// Settings
	settings := &Env{}
	host := settings.Get("HOST", "localhost")
	port := settings.Get("PORT", "8080")
	addr := fmt.Sprintf("%s:%s", host, port)
	uds := settings.Get("UDS", "")

	// Logs
	logger := &Log{}

	// Server
	mux := http.NewServeMux()
	mux.Handle("/health", &Health{})
	mux.Handle("/", &Hello{})
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-ctx.Done() // This blocks until the context is cancelled by calling stop() or sending a signal
		logger.Logf("Shutting down server...")
		if err := srv.Shutdown(context.Background()); err != nil {
			logger.Logf("Server shutdown failed: %s", err)
			os.Exit(1)
		}
	}()

	// Serve
	go func() {
		if uds != "" {
			ServeUDS(logger, uds, srv)
		} else {
			ServeHTTP(logger, addr, srv)
		}
	}()

	// Wait
	greener.PollForHealth("http://localhost:8080/health", 2*time.Second, 20*time.Millisecond)

	// Get response
	url := fmt.Sprintf("http://%s/", addr)
	resp, err := Get(ctx, url)
	if err != nil {
		logger.Logf("Error getting response: %v", err)
		os.Exit(1)
	}
	fmt.Print(resp)

	// Shutdown
	stop()

	// Output: Hello, world!
}
