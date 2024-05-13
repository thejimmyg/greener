// We're using the internal API of the content handler for this test so we make these tests part of the greener package, not the greener_test package

package greener

// Content encoding should follow the following algorthm rather than just doing what the client wants:
//
// 1. Try to serve the smallest content that is explicitly listed and a q that isn’t 0 or invalid, not the clients preferred one.
// 2. Otherwise if a wildcard is supported, serve gzip if present and identity otherwise
// 3. If there is no Accept-Encoding header or if it is empty or if an error occurs then serve identity
//
// These tests will cover different combinations of Accept-Encoding headers and the presence/absence of the content types (identity, gzip, brotli).

// Test Case Structure
//
// For each test, the setup should include:
//
// * A contentHandler instance with predefined content for identity, gzip, and brotli.
// * A simulated HTTP request with a specific Accept-Encoding header.
// * A response writer to capture output.
//
// The tests will check:
//
// * Correct Content-Encoding is set in the response headers.
// * Correct content (byte slice) is written to the response.

import (
	"bytes"
	"io/ioutil"
	"net/http/httptest"
	"testing"
)

func TestAcceptEncodingHeader(t *testing.T) {
	tests := []struct {
		name                  string
		acceptEncodingMissing bool
		acceptEncoding        string
		expectedEncoding      string
		content               []byte
		gzipContent           []byte
		brotliContent         []byte
		expectedContent       []byte
	}{
		{
			name:                  "Gzip explicitly allowed, smallest and present",
			acceptEncodingMissing: false,
			acceptEncoding:        "gzip, br;q=0",
			expectedEncoding:      "gzip",
			content:               []byte("original content"),
			gzipContent:           []byte("gzip content"),
			brotliContent:         nil,
			expectedContent:       []byte("gzip content"),
		},
		{
			name:                  "Brotli explicitly allowed, smallest and present",
			acceptEncodingMissing: false,
			acceptEncoding:        "br, gzip;q=0",
			expectedEncoding:      "br",
			content:               []byte("original content"),
			gzipContent:           nil,
			brotliContent:         []byte("brotli content"),
			expectedContent:       []byte("brotli content"),
		},
		{
			name:                  "Both allowed, gzip smaller",
			acceptEncoding:        "gzip, br",
			acceptEncodingMissing: false,
			expectedEncoding:      "gzip",
			content:               []byte("original content ------"),
			gzipContent:           []byte("smaller gzip content"),
			brotliContent:         []byte("longer brotli content ---"),
			expectedContent:       []byte("smaller gzip content"),
		},
		{
			name:                  "Both allowed, brotli smaller",
			acceptEncodingMissing: false,
			acceptEncoding:        "gzip, br",
			expectedEncoding:      "br",
			content:               []byte("original content ------"),
			gzipContent:           []byte("longer gzip content ---"),
			brotliContent:         []byte("smaller brotli content"),
			expectedContent:       []byte("smaller brotli content"),
		},
		{
			name:                  "None allowed (q=0) both present",
			acceptEncodingMissing: false,
			acceptEncoding:        "gzip;q=0, br;q=0",
			expectedEncoding:      "",
			content:               []byte("original content ------"),
			gzipContent:           []byte("smaller gzip content"),
			brotliContent:         []byte("smaller brotli content"),
			expectedContent:       []byte("original content ------"),
		},
		{
			name:                  "Invalid q-values",
			acceptEncodingMissing: false,
			acceptEncoding:        "gzip;q=1.5, br;q=-0.5",
			expectedEncoding:      "",
			content:               []byte("original content"),
			gzipContent:           nil,
			brotliContent:         nil,
			expectedContent:       []byte("original content"),
		},
		{
			name:                  "Valid and invalid q-values mixed",
			acceptEncodingMissing: false,
			acceptEncoding:        "gzip;q=0.8, br;q=-0.5",
			expectedEncoding:      "gzip",
			content:               []byte("original content --------"),
			gzipContent:           []byte("longer gzip content ---"),
			brotliContent:         []byte("smaller brotli content"),
			expectedContent:       []byte("longer gzip content ---"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new handler with the test case setup, but don't use the constructor to avoid actual compression taking place.
			handler := &contentHandler{hash: "fakehash", contentType: "fake/content-type", content: tt.content, gzipContent: tt.gzipContent, brotliContent: tt.brotliContent, cacheSeconds: 5}

			// Simulate the HTTP request and response
			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			// Check the encoding
			contentEncoding := res.Header.Get("Content-Encoding")
			if contentEncoding != tt.expectedEncoding {
				t.Errorf("expected content encoding '%s', got '%s'", tt.expectedEncoding, contentEncoding)
			}

			// Check the content
			body, _ := ioutil.ReadAll(res.Body)
			if !bytes.Equal(body, tt.expectedContent) {
				t.Errorf("expected content '%s', got '%s'", string(tt.expectedContent), string(body))
			}
		})
	}
}

// Wildcard Tests
//
// Checking behavior when the wildcard is used in the Accept-Encoding header.
//
//     Wildcard allowed, gzip and brotli present:
//         Accept-Encoding: "*"
//         Expected encoding: gzip (choose gzip as default for wildcard)
//
//     Wildcard with q=0:
//         Accept-Encoding: "*;q=0"
//         Expected encoding: identity
//

func TestWildcard(t *testing.T) {
	tests := []struct {
		name                  string
		acceptEncodingMissing bool
		acceptEncoding        string
		expectedEncoding      string
		content               []byte
		gzipContent           []byte
		brotliContent         []byte
		expectedContent       []byte
	}{
		{
			name:                  "Wildcard allowed, gzip and brotli present",
			acceptEncoding:        "*",
			acceptEncodingMissing: false,
			expectedEncoding:      "gzip",
			content:               []byte("original content"),
			gzipContent:           []byte("gzip content"),
			brotliContent:         []byte("brotli content"),
			expectedContent:       []byte("gzip content"),
		},
		{
			name:                  "Wildcard with q=0",
			acceptEncodingMissing: false,
			acceptEncoding:        "*;q=0",
			expectedEncoding:      "",
			content:               []byte("original content"),
			gzipContent:           []byte("gzip content"),
			brotliContent:         []byte("brotli content"),
			expectedContent:       []byte("original content"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &contentHandler{hash: "fakehash", contentType: "fake/content-type", content: tt.content, gzipContent: tt.gzipContent, brotliContent: tt.brotliContent, cacheSeconds: 5}

			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			contentEncoding := res.Header.Get("Content-Encoding")
			if contentEncoding != tt.expectedEncoding {
				t.Errorf("expected content encoding '%s', got '%s'", tt.expectedEncoding, contentEncoding)
			}

			body, _ := ioutil.ReadAll(res.Body)
			if !bytes.Equal(body, tt.expectedContent) {
				t.Errorf("expected content '%s', got '%s'", string(tt.expectedContent), string(body))
			}
		})
	}
}

// Header Absence or Errors
//
// Cases when the header is absent, empty, or an error occurs in parsing.
//
//     No Accept-Encoding header:
//         No Accept-Encoding header
//         Expected encoding: identity
//
//     Empty Accept-Encoding header:
//         Accept-Encoding: ""
//         Expected encoding: identity
//
//     Header parsing error (simulated):
//         Corrupted Accept-Encoding header value
//         Expected encoding: identity

func TestErrorConditions(t *testing.T) {
	tests := []struct {
		name                  string
		acceptEncodingMissing bool
		acceptEncoding        string
		expectedEncoding      string
		content               []byte
		gzipContent           []byte
		brotliContent         []byte
		expectedContent       []byte
	}{
		{
			name:                  "No Accept-Encoding header",
			acceptEncodingMissing: true,
			acceptEncoding:        "",
			expectedEncoding:      "",
			content:               []byte("original content"),
			gzipContent:           []byte("gzip content"),
			brotliContent:         []byte("brotli content"),
			expectedContent:       []byte("original content"),
		},
		{
			name:                  "Empty Accept-Encoding header",
			acceptEncodingMissing: false,
			acceptEncoding:        "",
			expectedEncoding:      "",
			content:               []byte("original content"),
			gzipContent:           []byte("gzip content"),
			brotliContent:         []byte("brotli content"),
			expectedContent:       []byte("original content"),
		},
		{
			name:                  "Header parsing error invalid float",
			acceptEncodingMissing: false,
			acceptEncoding:        "invalid;q=not_a_number",
			expectedEncoding:      "",
			gzipContent:           []byte("gzip content"),
			brotliContent:         []byte("brotli content"),
			content:               []byte("original content"),
			expectedContent:       []byte("original content"),
		},
		{
			name:                  "Last duplicate wins 1",
			acceptEncodingMissing: false,
			acceptEncoding:        "br;q=0, br;q=1",
			expectedEncoding:      "br",
			gzipContent:           []byte("gzip content"),
			brotliContent:         []byte("brotli content"),
			content:               []byte("original content"),
			expectedContent:       []byte("brotli content"),
		},
		{
			name:                  "Last duplicate wins 0",
			acceptEncodingMissing: false,
			acceptEncoding:        "br;q=1, br;q=0",
			expectedEncoding:      "",
			gzipContent:           []byte("gzip content"),
			brotliContent:         []byte("brotli content"),
			content:               []byte("original content"),
			expectedContent:       []byte("original content"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &contentHandler{hash: "fakehash", contentType: "fake/content-type", content: tt.content, gzipContent: tt.gzipContent, brotliContent: tt.brotliContent, cacheSeconds: 5}

			req := httptest.NewRequest("GET", "http://example.com", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			contentEncoding := res.Header.Get("Content-Encoding")
			if contentEncoding != tt.expectedEncoding {
				t.Errorf("expected content encoding '%s', got '%s'", tt.expectedEncoding, contentEncoding)
			}

			body, _ := ioutil.ReadAll(res.Body)
			if !bytes.Equal(body, tt.expectedContent) {
				t.Errorf("expected content '%s', got '%s'", string(tt.expectedContent), string(body))
			}
		})
	}
}
