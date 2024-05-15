package greener

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

// ETagEntry represents an entry in the etag.json file
type ETagEntry struct {
	MTime string `json:"mtime"`
	ETag  string `json:"etag"`
}

// CompressedFileHandler is a handler that serves compressed files.
type CompressedFileHandler struct {
	wwwHandler   http.Handler
	wwwgzHandler http.Handler
	wwwFS        fs.FS
	wwwgzFS      fs.FS
	etagMap      map[string]string
}

// NewCompressedFileHandler creates a new CompressedFileHandler.
func NewCompressedFileHandler(wwwFS, wwwgzFS fs.FS, etagMap map[string]string) *CompressedFileHandler {
	return &CompressedFileHandler{
		wwwHandler:   http.FileServer(http.FS(wwwFS)),
		wwwgzHandler: http.FileServer(http.FS(wwwgzFS)),
		wwwFS:        wwwFS,
		wwwgzFS:      wwwgzFS,
		etagMap:      etagMap,
	}
}

// LoadEtagsJSON parses the etag.json file and creates a map of paths to ETags.
func LoadEtagsJSON(data []byte) (map[string]string, error) {
	var etagFile struct {
		Entries map[string]ETagEntry `json:"entries"`
	}
	err := json.Unmarshal(data, &etagFile)
	if err != nil {
		return nil, err
	}

	etagMap := make(map[string]string)
	for path, entry := range etagFile.Entries {
		etagMap[path] = entry.ETag
	}

	return etagMap, nil
}

// ServeHTTP serves HTTP requests, checking for compressed versions of files.
func (h *CompressedFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestPath := r.URL.Path[1:]
	if etag, ok := h.etagMap[requestPath]; ok {
		etag = fmt.Sprintf("\"%s\"", etag)
		w.Header().Set("ETag", etag)
		if match := r.Header.Get("If-None-Match"); match != "" {
			if match = strings.TrimSpace(match); EtagMatch(match, etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	// Parse Accept-Encoding header
	acceptEncoding := r.Header.Get("Accept-Encoding")
	encodings := ParseEncodings(acceptEncoding)

	// Determine if an encoding is acceptable based on q-values
	isAcceptable := func(encoding string) bool {
		q, exists := encodings[encoding]
		return exists && q > 0
	}

	// Check if gzip is acceptable and the file exists in the wwwgz filesystem
	if isAcceptable("gzip") {
		gzipStat, err := fs.Stat(h.wwwgzFS, requestPath)
		if err == nil && gzipStat.Size() > 0 {
			w.Header().Set("Content-Encoding", "gzip")
			h.wwwgzHandler.ServeHTTP(w, r)
			return
		}
	}

	// Serve the original request from the www filesystem
	h.wwwHandler.ServeHTTP(w, r)
}
