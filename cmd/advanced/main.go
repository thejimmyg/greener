//go:generate go run ../generateetags/main.go ./www ./etags.json
//go:generate go run ../generategz/main.go ./www

package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/thejimmyg/greener"
)

// The go:embed syntax below is a special format that tells Go to copy all the
// matched files into the binary itself so that they can be accessed without
// needing the originals any more.

// We are going to set up one embedded filesystem for the www public files, and one for the icon.

//go:embed www/*
var wwwFS embed.FS

//go:embed wwwgz/*
var wwwgzFS embed.FS

//go:embed icon-512x512.png
var iconFileFS embed.FS

//go:embed etags.json
var etagsJson []byte

type HomeHandler struct {
	greener.EmptyPageProvider
	static *greener.CompressedFileHandler
}

func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		// If no other route is matched and the request is not for / then serve a static file
		h.static.ServeHTTP(w, r)
	} else {
		// Let's use our new WritePageProvider, instead of this version that uses app and s separately
		w.Write([]byte(h.Page("Hello", greener.Text("Hello <>!"), r.URL.Path)))
	}
}

type StartHandler struct {
	greener.EmptyPageProvider
}

func (h *StartHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(h.Page("Start", greener.Text("This is your app's start page."), r.URL.Path)))
}

func main() {
	// Setup
	wwwFSRoot, _ := fs.Sub(wwwFS, "www")       // Used for the static file serving
	wwwgzFSRoot, _ := fs.Sub(wwwgzFS, "wwwgz") // Used for compressed static file serving

	uiSupport := []greener.UISupport{greener.NewDefaultUISupport(
		"body {background: #ffc;}",
		`console.log("Hello from script");`,
		`console.log("Hello from service worker");`,
	)}
	themeColor := "#000000"
	appShortName := "Simple"
	logger := greener.NewDefaultLogger(log.Printf)
	// Both these would be longer for production though
	longCacheSeconds := 60 // In real life you might set this to a day, a month or a year perhaps
	shortCacheSeconds := 5 // Keep this fairly short because you want changes to propgagte quickly
	iconInjector, err := greener.NewDefaultIconsInjector(logger, iconFileFS, "icon-512x512.png", []int{16, 32, 144, 180, 192, 512}, longCacheSeconds)
	if err != nil {
		panic(err)
	}
	manifestInjector, err := greener.NewDefaultManifestInjector(logger, appShortName, themeColor, "/start", shortCacheSeconds, iconInjector.IconPaths(), []int{192, 512})
	if err != nil {
		panic(err)
	}
	injectors := []greener.Injector{
		greener.NewDefaultStyleInjector(logger, uiSupport, longCacheSeconds),
		greener.NewDefaultScriptInjector(logger, uiSupport, longCacheSeconds),
		greener.NewDefaultThemeColorInjector(logger, themeColor),
		greener.NewDefaultSEOInjector(logger, "A web app"),
		iconInjector,
		manifestInjector,
	}
	emptyPageProvider := greener.NewDefaultEmptyPageProvider(injectors)
	etags, err := greener.LoadEtagsJSON(etagsJson)
	if err != nil {
		panic(err)
	}
	static := greener.NewCompressedFileHandler(wwwFSRoot, wwwgzFSRoot, etags)

	// Routes
	mux := http.NewServeMux()
	emptyPageProvider.PerformInjections(mux)

	mux.Handle("/", &HomeHandler{EmptyPageProvider: emptyPageProvider, static: static})
	// This is loaded based on the injected manifest.json when the user opens your app in PWA mode
	mux.Handle("/start", &StartHandler{EmptyPageProvider: emptyPageProvider})

	// Serve
	err, ctx, _ := greener.AutoServe(logger, mux)
	if err != nil {
		panic(err)
	}
	<-ctx.Done()
}
