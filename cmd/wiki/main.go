package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/thejimmyg/greener"
)

//go:embed www/*
var wwwFiles embed.FS

type WikiConfig struct{}

func NewWikiConfig() *WikiConfig {
	return &WikiConfig{}
}

// An example of injecting a component which needs both the WikiApp and WikiServices
type WritePageProvider interface {
	WritePage(title string, body greener.HTMLable)
}

type DefaultWritePageProvider struct {
	greener.ResponseWriterProvider
	greener.EmptyPageProvider
}

func (d *DefaultWritePageProvider) WritePage(title string, body greener.HTMLable) {
	d.Page(title, body).WriteHTMLTo(d.W())
}

func NewDefaultWritePageProvider(emptyPageProvider greener.EmptyPageProvider, responseWriterProvider greener.ResponseWriterProvider) *DefaultWritePageProvider {
	return &DefaultWritePageProvider{EmptyPageProvider: emptyPageProvider, ResponseWriterProvider: responseWriterProvider}
}

type WikiServices struct {
	greener.Services
	WritePageProvider // Here is the interface we are extending the serivces with
}

type WikiApp struct {
	greener.App
	*WikiConfig
}

func NewWikiApp(app greener.App, wikiConfig *WikiConfig) *WikiApp {
	return &WikiApp{
		App:        app,
		WikiConfig: wikiConfig,
	}
}

func (app *WikiApp) HandleWithWikiServices(path string, handler func(*WikiServices)) {
	app.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		services := app.NewServices(w, r)
		s := &WikiServices{Services: services} // We have to leave WritePageProvider nil temporarily
		writePageProvider := NewDefaultWritePageProvider(app, s)
		s.WritePageProvider = writePageProvider // Now we set it here.
		handler(s)
	})
}

func main() {
	// Setup
	wwwFS, _ := fs.Sub(wwwFiles, "www") // Used for the icon and the static file serving
	uiSupport := []greener.UISupport{greener.NewDefaultUISupport(
		"body {background: #ffc;}",
		`console.log("Hello from script");`,
		`console.log("Hello from service worker");`,
	)}
	themeColor := "#000000"
	appShortName := "Wiki"
	config := greener.NewDefaultServeConfigProviderFromEnvironment()
	logger := greener.NewDefaultLogger(log.Printf)
	injectors := []greener.Injector{
		greener.NewDefaultStyleInjector(logger, uiSupport),
		greener.NewDefaultScriptInjector(logger, uiSupport),
		greener.NewDefaultServiceWorkerInjector(logger, uiSupport),
		greener.NewDefaultThemeColorInjector(logger, themeColor),
		greener.NewDefaultIconsInjector(logger, wwwFS),
		greener.NewDefaultManifestInjector(logger, appShortName),
	}
	emptyPageProvider := greener.NewDefaultEmptyPageProvider(injectors)
	static := greener.NewCompressedFileHandler(http.FS(wwwFS))

	// Routes
	app := NewWikiApp(greener.NewDefaultApp(config, logger, emptyPageProvider), NewWikiConfig())
	app.HandleWithWikiServices("/", func(s *WikiServices) {
		if s.R().URL.Path != "/" {
			// If no other route is matched and the request is not for / then serve a static file
			static.ServeHTTP(s.W(), s.R())
		} else {
			// Let's use our new WritePageProvider, instead of this version that uses app and s separately
			// app.Page("Hello", greener.Text("Hello <>!")).WriteHTMLTo(s.W())
			s.WritePage("Hello", greener.Text("Hello <>!"))
		}
	})

	// Serve
	app.Serve(context.Background())
}
