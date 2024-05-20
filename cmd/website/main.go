package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"

	"github.com/thejimmyg/greener"
)

//go:embed icon-512x512.png
var iconFileFS embed.FS

type PageHandler struct {
	greener.EmptyPageProvider
}

func (h *PageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p, ok := pageMap[r.URL.Path]; ok {
		if err := p.ConvertMarkdownToHTML(); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		var breadcrumbs template.HTML
		if p.section != rootSection {
			breadcrumbs = generateBreadcrumbs(p)
		}
		sectionNav := generateSectionNav(p, false, "section", r.URL.Path)
		renderTemplate(w, h.Page, p.Title, breadcrumbs, sectionNav, p.HTML)

	} else {
		http.Error(w, "404 Not Found", http.StatusNotFound)
	}
}

type SitemapHandler struct {
	greener.EmptyPageProvider
}

func (h *SitemapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p, ok := pageMap[r.URL.Path]; ok {
		sitemapHTML := generateSitemapHTML(rootSection, 1, r.URL.Path) // Generate the sitemap content
		var breadcrumbs template.HTML
		if p.section != rootSection {
			breadcrumbs = generateBreadcrumbs(p)
		}
		sectionNav := generateSectionNav(p, false, "section", r.URL.Path)
		renderTemplate(w, h.Page, "Site Map", breadcrumbs, sectionNav, greener.HTMLPrintf("<h1>Sitemap</h1> %s", sitemapHTML))
	} else {
		http.Error(w, "404 Not Found", http.StatusNotFound)
	}
}

// w.Write([]byte(h.Page("Start", greener.Text("This is your app's start page."))))

func main() {
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
	mux.Handle("/sitemap", &SitemapHandler{EmptyPageProvider: emptyPageProvider})

	// Serve
	ctx, _ := greener.AutoServe(logger, mux)
	<-ctx.Done()
}
