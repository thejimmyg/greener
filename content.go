package greener

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

func GenerateETag(content []byte) string {
	h := md5.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func StaticContentHandler(logger Logger, content []byte, contentType string) http.HandlerFunc {
	serveGzip := true
	etag := GenerateETag(content)

	var gzippedContent bytes.Buffer
	gw := gzip.NewWriter(&gzippedContent)
	if _, err := gw.Write(content); err != nil {
		logger.Logf("failed to gzip content: %v", err)
		serveGzip = false
	}
	if err := gw.Close(); err != nil {
		logger.Logf("failed to close gzip writer for content: %v", err)
		serveGzip = false
	}
	if gzippedContent.Len() >= len(content) {
		serveGzip = false
	}

	return func(w http.ResponseWriter, r *http.Request) {

		// Check ETag to possibly return 304 Not Modified
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// Set common headers
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", contentType)

		if serveGzip && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			// Serve gzipped content
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Length", strconv.Itoa(gzippedContent.Len()))
			w.WriteHeader(http.StatusOK)
			w.Write(gzippedContent.Bytes())
		} else {
			// Serve original content
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.WriteHeader(http.StatusOK)
			w.Write(content)
		}
	}
}
