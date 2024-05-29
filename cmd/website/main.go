package main

import (
	"embed"
	"github.com/thejimmyg/greener"
	"log"
	"net/http"
)

//go:embed icon-512x512.png
var iconFileFS embed.FS

func main() {
	dumpSection(rootSection, 0)
	// Setup
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

	// Routes
	mux := http.NewServeMux()
	emptyPageProvider.PerformInjections(mux)

	mux.Handle("/", &PageHandler{EmptyPageProvider: emptyPageProvider})
	// This is loaded based on the injected manifest.json when the user opens your app in PWA mode
	// mux.Handle("/sitemap", &SitemapHandler{EmptyPageProvider: emptyPageProvider})

	// Serve
	err, ctx, _ := greener.AutoServe(logger, mux)
	if err != nil {
		panic(err)
	}
	<-ctx.Done()
}
