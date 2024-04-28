package greener

import (
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
)

type deferredResponseWriter struct {
	http.ResponseWriter
	body        io.ReadWriter
	status      int
	wroteHeader bool
}

func (drw *deferredResponseWriter) WriteHeader(status int) {
	drw.status = status
	if status != http.StatusNotFound {
		drw.ResponseWriter.WriteHeader(status)
		drw.wroteHeader = true
	}
}

func (drw *deferredResponseWriter) Write(p []byte) (int, error) {
	if drw.status != http.StatusNotFound && !drw.wroteHeader {
		drw.WriteHeader(http.StatusOK) // Assume OK if no other status has been written.
	}
	if drw.status == http.StatusNotFound {
		return len(p), nil // Do nothing, just pretend to write for 404.
	}
	return drw.ResponseWriter.Write(p)
}

func NewCompressedFileHandler(root http.FileSystem) http.Handler {
	fileServer := http.FileServer(root)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			// Normalize path to ensure it has a trailing slash for directories
			urlPath := path.Clean(r.URL.Path)
			if !strings.HasSuffix(urlPath, "/") {
				urlPath += "/"
			}

			// Check for index.html.gz in directories
			d, err := root.Open(urlPath)
			if err == nil {
				defer d.Close()
				if stat, err := d.Stat(); err == nil && stat.IsDir() {
					index := "index.html.gz"
					if _, err := root.Open(path.Join(urlPath, index)); err == nil {
						w.Header().Set("Content-Type", "text/html")
						w.Header().Set("Content-Encoding", "gzip")
						r.URL.Path = path.Join(urlPath, index)
						fileServer.ServeHTTP(w, r)
						return
					}
				}
			}
			// Check for .gz version of the file
			gzipPath := r.URL.Path + ".gz"
			if _, err := root.Open(gzipPath); err == nil {
				setGzipContentType(w, root, r.URL.Path)
				w.Header().Set("Content-Encoding", "gzip")
				r.URL.Path = gzipPath
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Serve the original request
		fileServer.ServeHTTP(w, r)
	})
}

// setGzipContentType sets the Content-Type header based on the file extension,
// falling back to http.DetectContentType if the extension is not recognized.
func setGzipContentType(w http.ResponseWriter, fs http.FileSystem, filePath string) {
	// Attempt to determine Content-Type from file extension first
	ext := strings.ToLower(path.Ext(filePath))
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
		return
	}

	// Fallback to detecting Content-Type by reading file content
	// if MIME type is not determined by file extension
	f, err := fs.Open(filePath)
	if err != nil {
		// Unable to open the file; skip setting Content-Type
		return
	}
	defer f.Close()

	// Read a small slice of the file to determine the content type
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return // File is empty or unable to read; skip setting Content-Type
	}

	// Use http.DetectContentType to get the MIME type
	contentType := http.DetectContentType(buf)
	w.Header().Set("Content-Type", contentType)
}
