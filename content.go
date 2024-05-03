package greener

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/andybalholm/brotli"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// ContentHandler handles brotli and gzip compression of content as well as generating a hash so that smallet content possible can be served to the client based on the request Content-Encoding. The response is set with 1 year cache max age with the intention that the URL the handler is registered with includes the content hash so that if the content changes, so would the URL.
type ContentHandler interface {
	Hash() string
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type contentHandler struct {
	hash          string
	contentType   string
	content       []byte
	gzipContent   []byte
	brotliContent []byte
	logger        Logger
}

// NewContentHandler returns a struct containing a hash of the content as well as gzip and brotli compressed content encodings. It implements http.Handler for serving the most appropriate content encoding based on the request.
func NewContentHandler(logger Logger, content []byte, contentType, salt string) ContentHandler {
	// Hash the content with salt
	hash := hashContentWithSalt(content, salt) // New hashing function

	// Compress the content
	originalBytes := []byte(content)
	gzipContent, brotliContent, err := compressContent(originalBytes)
	if err != nil {
		logger.Logf("Failed to compress the content: %v", err)
	}

	// Check if compressed versions are actually shorter
	if gzipContent == nil || len(gzipContent) >= len(originalBytes) {
		gzipContent = nil
	}
	if brotliContent == nil || len(brotliContent) >= len(originalBytes) {
		brotliContent = nil
	}

	return &contentHandler{
		hash:          hash,
		contentType:   contentType,
		content:       originalBytes,
		gzipContent:   gzipContent,
		brotliContent: brotliContent,
	}
}

func (c *contentHandler) Hash() string {
	return c.hash
}

func hashContentWithSalt(content []byte, salt string) string {
	hmac := hmac.New(sha256.New, []byte(salt))
	hmac.Write(content)
	sum := hmac.Sum(nil)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(sum)
}

func compressContent(content []byte) ([]byte, []byte, error) {
	var gzipBuffer, brotliBuffer bytes.Buffer
	var wg sync.WaitGroup
	var errGzip, errBrotli error

	wg.Add(1)
	go func() {
		defer wg.Done()
		gzipWriter := gzip.NewWriter(&gzipBuffer)
		if _, err := gzipWriter.Write(content); err != nil {
			errGzip = err
			return
		}
		errGzip = gzipWriter.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		brotliWriter := brotli.NewWriterLevel(&brotliBuffer, brotli.BestCompression)
		if _, err := brotliWriter.Write(content); err != nil {
			errBrotli = err
			return
		}
		errBrotli = brotliWriter.Close()
	}()

	wg.Wait()

	if errGzip != nil && errBrotli != nil {
		return nil, nil, fmt.Errorf("Both Gzip and Brotli compression failed: %v\n%v", errGzip, errBrotli)
	} else if errGzip == nil && errBrotli == nil {
		return gzipBuffer.Bytes(), brotliBuffer.Bytes(), nil
	} else if errBrotli != nil {
		return gzipBuffer.Bytes(), nil, errBrotli
	} else {
		return nil, brotliBuffer.Bytes(), errGzip
	}
}

func (c *contentHandler) brotliBest(supportsBrotli, supportsGzip bool, acceptEncoding string) bool {
	if supportsBrotli && c.brotliContent != nil && (c.gzipContent == nil || len(c.brotliContent) < len(c.gzipContent)) {
		return true
	}
	return false
}

func (c *contentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var contentBytes []byte
	var contentEncoding string
	var contentLength string

	acceptEncoding := r.Header.Get("Accept-Encoding")
	supportsBrotli := strings.Contains(acceptEncoding, "br")
	supportsGzip := strings.Contains(acceptEncoding, "gzip")

	if c.brotliBest(supportsBrotli, supportsGzip, acceptEncoding) {
		contentBytes = c.brotliContent
		contentEncoding = "br"
		contentLength = strconv.Itoa(len(c.brotliContent))
	} else if supportsGzip && c.gzipContent != nil {
		contentBytes = c.gzipContent
		contentEncoding = "gzip"
		contentLength = strconv.Itoa(len(c.gzipContent))
	} else {
		contentBytes = c.content
		contentLength = strconv.Itoa(len(c.content))
	}

	w.Header().Set("Content-Type", c.contentType)
	w.Header().Set("Content-Length", contentLength)
	w.Header().Set("Cache-Control", "public, max-age=31536000") // One year max-age
	if contentEncoding != "" {
		w.Header().Set("Content-Encoding", contentEncoding)
	}

	w.Write(contentBytes)
}

// StaticContentHandler prepares and serves the most appropriate content-encoding using an etag and can return 304 not modified responses as needed.

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
