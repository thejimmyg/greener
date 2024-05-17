package main

import (
	"bufio"
	"fmt"
	"net/http"
)

func main() {
	// Make a request to the SSE endpoint
	resp, err := http.Get("http://localhost:8080/events")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	// Check if the server supports SSE
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		fmt.Println("Server does not support SSE")
		return
	}

	// Read the events from the response body
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error:", err)
	}
}
