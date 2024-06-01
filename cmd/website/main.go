package main

import (
	"embed"
	"github.com/thejimmyg/greener"
	"io/fs"
	"log"
	"net/http"
)

//go:embed icon-512x512.png
var iconFileFS embed.FS

// Embed the markdown files located in the pages directory
//
//go:embed pages/*
var pageFiles embed.FS

func main() {

	pageMap := make(map[string]*greener.Page)
	markdown, err := fs.ReadFile(pageFiles, "pages/sitemap.md")
	if err != nil {
		log.Fatalf("Error reading sitemap markdown: %s", err)
	}

	rootSection, err := greener.ParseSitemap("/sitemap.html", markdown)
	if err != nil {
		log.Fatalf("Error parsing sitemap markdown: %s", err)
	}

	greener.BuildPageInSectionMap(rootSection, pageMap)

	greener.DumpSection(rootSection, 0)
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
	manifestInjector, err := greener.NewDefaultManifestInjector(logger, appShortName, themeColor, "start", shortCacheSeconds, iconInjector.IconPaths(), []int{192, 512})
	if err != nil {
		panic(err)
	}
	injectors := []greener.Injector{
		greener.NewDefaultStyleInjector(logger, greener.NavUISupport, longCacheSeconds),
		greener.NewDefaultScriptInjector(logger, greener.NavUISupport, longCacheSeconds),
		greener.NewDefaultThemeColorInjector(logger, themeColor),
		greener.NewDefaultSEOInjector(logger, "A web app"),
		iconInjector,
		manifestInjector,
	}
	emptyPageProvider := greener.NewDefaultEmptyPageProvider(injectors)

	// Routes
	mux := http.NewServeMux()
	emptyPageProvider.PerformInjections(mux)

	mux.Handle("/", &greener.PageHandler{RootSection: rootSection, PagesFS: pageFiles, EmptyPageProvider: emptyPageProvider, PageMap: pageMap})
	// This is loaded based on the injected manifest.json when the user opens your app in PWA mode
	// mux.Handle("/sitemap", &SitemapHandler{EmptyPageProvider: emptyPageProvider})

	// Serve
	err, ctx, _ := greener.AutoServe(logger, mux)
	if err != nil {
		panic(err)
	}
	<-ctx.Done()
}
