package greener

import (
	"bytes"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"io/fs"
	"net/url"
	"sync"

	"golang.org/x/image/draw"
)

// ImageData holds the resized image and its ETag
type ImageData struct {
	Image image.Image
	Size  string
}

func resizeImage(src image.Image, size int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

func loadAndResizeImage(icon512Bytes []byte, icon512 image.Image, sizes []int) (map[int][]byte, error) {
	resizedImages := make(map[int][]byte)
	resizedImages[512] = icon512Bytes

	var wg sync.WaitGroup
	var mutex sync.Mutex                    // Mutex to protect map writes
	errChan := make(chan error, len(sizes)) // Buffered channel to store errors from goroutines

	for _, size := range sizes {
		wg.Add(1)
		go func(size int) {
			defer wg.Done()
			resized := resizeImage(icon512, size)
			buf := new(bytes.Buffer)
			if err := png.Encode(buf, resized); err != nil {
				errChan <- err
				return
			}
			mutex.Lock() // Lock the mutex before accessing the shared map
			resizedImages[size] = buf.Bytes()
			mutex.Unlock() // Unlock the mutex after the map is updated
		}(size)
	}

	wg.Wait()
	close(errChan) // Close the channel to signal no more values will be sent

	// Check if there were any errors
	for err := range errChan {
		if err != nil {
			return nil, err // Return the first error encountered
		}
	}

	return resizedImages, nil
}

// func StaticIconHandler(logger Logger, icon512 image.Image, etag string, sizes []int) http.HandlerFunc {
// 	resizedImages, err := loadAndResizeImage(icon512, etag, sizes)
// 	if err != nil {
// 		log.Fatalf("Failed to resize icons: %v\n", err)
// 	}
//
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		// Determine requested size from URL
// 		path := r.URL.Path
// 		var requestedSize int
// 		var sizeFound bool
//
// 		for size := range resizedImages {
// 			if strings.Contains(path, strconv.Itoa(size)+"x"+strconv.Itoa(size)) {
// 				requestedSize = size
// 				sizeFound = true
// 				break
// 			}
// 		}
//
// 		if !sizeFound {
// 			http.NotFound(w, r)
// 			return
// 		}
//
// 		// Check for ETag match to possibly return 304 Not Modified
// 		// logger.Logf("ETag: %s, %s", r.Header.Get("If-None-Match"), resizedImages[requestedSize].ETag)
// 		if r.Header.Get("If-None-Match") == resizedImages[requestedSize].ETag {
// 			w.WriteHeader(http.StatusNotModified)
// 			return
// 		}
//
// 		// Serve the requested size
// 		w.Header().Set("Content-Type", "image/png")
// 		w.Header().Set("ETag", resizedImages[requestedSize].ETag)
// 		err := png.Encode(w, resizedImages[requestedSize].Image)
// 		if err != nil {
// 			logger.Logf("Failed to encode image: %v", err)
// 			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
// 		}
// 	}
// }

type DefaultIconsInjector struct {
	Logger
	icon512      image.Image
	sizes        []int
	cacheSeconds int
	html         string
	paths        map[int]string
	chs          map[int]ContentHandler
}

func (d *DefaultIconsInjector) IconPaths() map[int]string {
	return d.paths
}

func (d *DefaultIconsInjector) Inject(mux HandlerRouter) (template.HTML, template.HTML) {
	for size := range d.chs {
		mux.Handle("/"+d.paths[size], d.chs[size])
	}
	return template.HTML(d.html), template.HTML("")
}

func NewDefaultIconsInjector(logger Logger, iconFS fs.FS, icon512Path string, sizes []int, cacheSeconds int) (*DefaultIconsInjector, error) {
	icon512Bytes, err := fs.ReadFile(iconFS, icon512Path)
	if err != nil {
		logger.Logf("Failed to open source image for favicon: %v", err)
		return nil, err
	}
	icon512, _, err := image.Decode(bytes.NewReader(icon512Bytes))
	if err != nil {
		logger.Logf("Failed to decode source image for favicon: %v", err)
		return nil, err
	}

	d := &DefaultIconsInjector{
		Logger:       logger,
		icon512:      icon512,
		sizes:        sizes,
		cacheSeconds: cacheSeconds,
		paths:        make(map[int]string),
		chs:          make(map[int]ContentHandler),
		html:         "",
	}
	d.Logf("Injecting route and HTML for png icons")
	resizedImages, err := loadAndResizeImage(icon512Bytes, icon512, d.sizes)
	if err != nil {
		logger.Logf("Failed to resize icons: %v\n", err)
		return nil, err
	}
	for _, size := range sizes {
		imageData := resizedImages[size]
		d.chs[size] = NewContentHandler(d.Logger, imageData, "image/png", "", d.cacheSeconds)
		path := fmt.Sprintf("icon-%dx%d-%s.png", size, size, d.chs[size].Hash())
		encodedPath := url.PathEscape(path)
		d.html += fmt.Sprintf(`
    <link rel="icon" type="image/png" sizes="%dx%d" href="/%s">`, size, size, encodedPath)
		if size == 180 {
			d.html += fmt.Sprintf(`
    <link rel="apple-touch-icon" sizes="180x180" href="/%s">`, encodedPath)
		}
		d.paths[size] = path
	}
	return d, nil
}
