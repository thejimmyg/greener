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
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		// fmt.Printf("Accpets gzip\n")
		// Check if the requested path exists in the wwwgz filesystem and is non-zero in size
		gzipStat, err := fs.Stat(h.wwwgzFS, requestPath)
		if err == nil && gzipStat.Size() > 0 {
			// fmt.Printf("Serving gzip\n")
			w.Header().Set("Content-Encoding", "gzip")
			h.wwwgzHandler.ServeHTTP(w, r)
			return
		} else {
			// fmt.Printf("Error: %v, path: %s\n", err, r.URL.Path)
		}
	}

	// Serve the original request from the www filesystem
	h.wwwHandler.ServeHTTP(w, r)
}
