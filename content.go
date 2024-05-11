package greener

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	cacheSeconds  int
}

// NewContentHandler returns a struct containing a hash of the content as well as gzip and brotli compressed content encodings. It implements http.Handler for serving the most appropriate content encoding based on the request.
func NewContentHandler(logger Logger, content []byte, contentType, salt string, cacheSeconds int) ContentHandler {
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
		cacheSeconds:  cacheSeconds,
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
	if c.cacheSeconds > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", c.cacheSeconds))
	}
	if contentEncoding != "" {
		w.Header().Set("Content-Encoding", contentEncoding)
	}

	// Set ETag header
	etag := "\"" + c.Hash() + "\"" // Adding quotes around the ETag as per the HTTP ETag format
	w.Header().Set("ETag", etag)
	// Check if the client has sent the If-None-Match header and compare the ETag
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match = strings.TrimSpace(match); etagMatch(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// Send the content as the ETag did not match or was not provided
	w.Write(contentBytes)
}

func etagMatch(header, etag string) bool {
	etags := strings.Split(header, ",")
	for _, e := range etags {
		trimmedEtag := strings.TrimSpace(e)
		// Remove surrounding quotes for a standardized comparison
		if len(trimmedEtag) >= 2 && trimmedEtag[0] == '"' && trimmedEtag[len(trimmedEtag)-1] == '"' {
			trimmedEtag = trimmedEtag[1 : len(trimmedEtag)-1]
		}
		// Compare without quotes
		if trimmedEtag == strings.Trim(etag, "\"") {
			return true
		}
	}
	return false
}
