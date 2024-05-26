package greener_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/thejimmyg/greener"
)

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

type Hello struct{}

func (h *Hello) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, world!\n"))
}

type Health struct{}

func (h *Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

// Demonstrates how to set up and run a server
func Example_server() {
	// Logs
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
	logger := greener.NewDefaultLogger(log.Printf)

	// Server
	mux := http.NewServeMux()
	mux.Handle("/", &Hello{})

	// Set up a context that can be cancelled by SIGTERM or SIGINT, serve
	// based on HOST, PORT, ORIGIN and UDS, add a /livez path, wait for
	// /livez to return 200 if possible based on the environment variables
	// chosen
	err, ctx, stop := greener.AutoServe(logger, mux)
	if err != nil {
		logger.Logf("ERROR: %v\n", err)
	}

	// Get response
	resp, err := Get(ctx, "http://localhost:8000/")
	if err != nil {
		logger.Logf("Error getting response: %v", err)
		os.Exit(1)
	}
	fmt.Print(resp)

	// Shutdown
	stop()
	<-ctx.Done()
	logger.Logf("Shut down server.")
	// Output: Server listening on localhost:8000
	// Hello, world!
	// Shut down server.
}
