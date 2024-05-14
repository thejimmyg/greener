package greener

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

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
	return fmt.Errorf("server not ready within %v", timeout)
}

// handle is a generic function that abstracts the common pattern of decoding an HTTP request,
// processing it using a provided function that might return an error, and encoding the result back to HTTP response.
func ActorHandleCall[R any, T any](w http.ResponseWriter, r *http.Request, process func(R) (T, error)) {
	var request R
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	response, err := process(request)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error processing request: %v", err), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// RemoteCall is a generic function that sends a request and expects a response, handling errors and HTTP communication.
func ActorRemoteCall[R any, T any](client *http.Client, serverURL, endpoint string, requestData R) (T, error) {
	var responseData T

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return responseData, fmt.Errorf("error marshalling request data: %w", err)
	}

	response, err := client.Post(serverURL+"/"+endpoint, "application/json", bytes.NewBuffer(jsonData))
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
