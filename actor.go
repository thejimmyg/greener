package greener

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// trimBearer removes the "Bearer " prefix from an authorization header, case-insensitively, and trims any surrounding whitespace.
func trimBearer(authHeader string) string {
	// Define the prefix in a standard case.
	prefix := "bearer"
	// Convert the header to lowercase to ensure case-insensitive comparison.
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(authHeader)), prefix) {
		// Trim the prefix and any surrounding whitespace
		return strings.TrimSpace(strings.TrimSpace(authHeader)[len(prefix):])
	}
	return strings.TrimSpace(authHeader)
}

func PollForHealth(url string, timeout time.Duration, retryTimeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			// fmt.Println("Server is healthy and ready.")
			return nil
		}
		// if err != nil {
		// 	fmt.Printf("Failed to reach server: %v\n", err)
		// } else {
		// 	fmt.Printf("Server not ready, status code: %d\n", resp.StatusCode)
		// }
		time.Sleep(retryTimeout) // sleep before retrying
	}
	return fmt.Errorf("server at %s not ready within %v", url, timeout)
}

// handle is a generic function that abstracts the common pattern of decoding an HTTP request,
// processing it using a provided function that might return an error, and encoding the result back to HTTP response.
func ActorHandleCall[M any, T any](w http.ResponseWriter, r *http.Request, process func(context.Context, string, M) (T, error)) {
	var message M
	credentials := trimBearer(r.Header.Get("Authorization"))
	ctx := r.Context()
	timeoutHeader := r.Header.Get("X-Timeout-Ms")
	if timeoutHeader != "" {
		// Parse timeout from header
		timeoutMs, err := strconv.Atoi(timeoutHeader)
		if err != nil {
			http.Error(w, "Invalid timeout value", http.StatusBadRequest)
			return
		}
		c, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
		ctx = c
	}
	if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	response, err := process(ctx, credentials, message)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error processing request: %v", err), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// RemoteCall is a generic function that sends a request and expects a response, handling errors and HTTP communication.
func ActorRemoteCall[R any, T any](client *http.Client, serverURL, endpoint string, ctx context.Context, credentials string, requestData R) (T, error) {
	var responseData T
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return responseData, fmt.Errorf("error marshalling request data: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/"+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return responseData, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+credentials)
	deadline, ok := ctx.Deadline()
	if ok {
		timeout := time.Until(deadline)
		req.Header.Set("X-Timeout-Ms", fmt.Sprintf("%d", timeout.Milliseconds()))
	}
	response, err := client.Do(req)
	if err != nil {
		return responseData, fmt.Errorf("error sending request to '%s': %w", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return responseData, fmt.Errorf("received non-OK HTTP status from '%s': %s", endpoint, response.Status)
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return responseData, fmt.Errorf("error reading response body from '%s': %w", endpoint, err)
	}
	if err := json.Unmarshal(body, &responseData); err != nil {
		return responseData, fmt.Errorf("error unmarshalling response from '%s': %w", endpoint, err)
	}
	return responseData, nil
}
