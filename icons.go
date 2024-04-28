package greener

import (
	"bytes"
	"image"
	"image/png"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/Kodeworks/golang-image-ico"
	"golang.org/x/image/draw"
)

// ImageData holds the resized image and its ETag
type ImageData struct {
	Image image.Image
	ETag  string
}

func resizeImage(src image.Image, size int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

func loadAndResizeImage(icon512 *image.Image, etag string, sizes []int) (map[int]*ImageData, error) {
	resizedImages := make(map[int]*ImageData)
	resizedImages[512] = &ImageData{Image: *icon512, ETag: etag}
	var wg sync.WaitGroup
	for _, size := range sizes {
		wg.Add(1)
		go func(size int) {
			defer wg.Done()
			resized := resizeImage(*icon512, size)
			buf := new(bytes.Buffer)
			if err := png.Encode(buf, resized); err != nil {
				// Handle error properly
				return
			}
			etag := GenerateETag(buf.Bytes())

			resizedImages[size] = &ImageData{
				Image: resized,
				ETag:  etag,
			}
		}(size)
	}
	wg.Wait()

	return resizedImages, nil
}

func StaticIconHandler(logger Logger, icon512 *image.Image, etag string, sizes []int) http.HandlerFunc {
	resizedImages, err := loadAndResizeImage(icon512, etag, sizes)
	if err != nil {
		log.Fatalf("Failed to resize icons: %v\n", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Determine requested size from URL
		path := r.URL.Path
		var requestedSize int
		var sizeFound bool

		for size := range resizedImages {
			if strings.Contains(path, strconv.Itoa(size)+"x"+strconv.Itoa(size)) {
				requestedSize = size
				sizeFound = true
				break
			}
		}

		if !sizeFound {
			http.NotFound(w, r)
			return
		}

		// Check for ETag match to possibly return 304 Not Modified
		// logger.Logf("ETag: %s, %s", r.Header.Get("If-None-Match"), resizedImages[requestedSize].ETag)
		if r.Header.Get("If-None-Match") == resizedImages[requestedSize].ETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// Serve the requested size
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", resizedImages[requestedSize].ETag)
		err := png.Encode(w, resizedImages[requestedSize].Image)
		if err != nil {
			logger.Logf("Failed to encode image: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func StaticFaviconHandler(logger Logger, icon512 *image.Image) http.HandlerFunc {
	var favicon []byte // This will store the ICO data
	var faviconETag string

	// Resize the image to 16x16 pixels only
	icoImage := resizeImage(*icon512, 16)
	buf := new(bytes.Buffer)
	// Encode the single image into the ICO format
	if err := ico.Encode(buf, icoImage); err != nil {
		logger.Logf("Failed to encode favicon: %v", err)
		return nil
	}
	favicon = buf.Bytes()
	// Generate the ETag for the favicon data
	faviconETag = GenerateETag(favicon)

	return func(w http.ResponseWriter, r *http.Request) {
		// Check for ETag match to possibly return 304 Not Modified
		if r.Header.Get("If-None-Match") == faviconETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// Serve the favicon.ico file with ETag
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("ETag", faviconETag)
		_, err := w.Write(favicon)
		if err != nil {
			logger.Logf("Failed to serve favicon: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
