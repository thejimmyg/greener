package greener

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// DefaultLogger implements Logger.
type DefaultLogger struct {
	logf func(string, ...interface{})
}

func (cl *DefaultLogger) Logf(m string, a ...interface{}) {
	cl.logf(m, a...)
}

func (cl *DefaultLogger) Errorf(m string, a ...interface{}) {
	cl.logf("ERROR: "+m, a...)
}

func NewDefaultLogger(logf func(string, ...interface{})) *DefaultLogger {
	return &DefaultLogger{logf: logf}
}

// DefaultResponseWriterProvider implements ResponseWriterProvider
type DefaultResponseWriterProvider struct {
	w http.ResponseWriter
}

func (d *DefaultResponseWriterProvider) W() http.ResponseWriter {
	return d.w
}

func NewDefaultResponseWriterProvider(w http.ResponseWriter) *DefaultResponseWriterProvider {
	return &DefaultResponseWriterProvider{w: w}
}

// DefaultRequestProvider implements RequestProvider
type DefaultRequestProvider struct {
	r *http.Request
}

func (d *DefaultRequestProvider) R() *http.Request {
	return d.r
}

func NewDefaultRequestProvider(r *http.Request) *DefaultRequestProvider {
	return &DefaultRequestProvider{r: r}
}

// DefaultServeConfigProvider implments ServeConfigProvider for returning the configuration needed for serving the app
type DefaultServeConfigProvider struct {
	host string
	port int
	uds  string
}

func (dscp *DefaultServeConfigProvider) Host() string {
	return dscp.host
}
func (dscp *DefaultServeConfigProvider) Port() int {
	return dscp.port
}
func (dscp *DefaultServeConfigProvider) UDS() string {
	return dscp.uds
}

func NewDefaultServeConfigProviderFromEnvironment() *DefaultServeConfigProvider {
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8000"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}
	return &DefaultServeConfigProvider{host: os.Getenv("HOST"), port: port, uds: os.Getenv("UDS")}
}

// DefaultServices embeds other interfaces for providing services to a request handler
type DefaultServices struct {
	ServeConfigProvider
	Logger
	ResponseWriterProvider
	RequestProvider
}

func NewDefaultServices(
	serveConfigProvider ServeConfigProvider,
	logger Logger,
	responseWriterProvider ResponseWriterProvider,
	requestProvider RequestProvider,
) Services {
	return &DefaultServices{
		ServeConfigProvider:    serveConfigProvider,
		Logger:                 logger,
		ResponseWriterProvider: responseWriterProvider,
		RequestProvider:        requestProvider,
	}
}

// DefaultStyleProvider implements StyleProvider
type DefaultStyleProvider struct {
	style string
}

func (dsp *DefaultStyleProvider) Style() string {
	return dsp.style
}

// DefaultScriptProvider implements ScriptProvider
type DefaultScriptProvider struct {
	script string
}

func (dsp *DefaultScriptProvider) Script() string {
	return dsp.script
}

// DefaultServiceWorkerProvider implements ServiceWorkerProvider
type DefaultServiceWorkerProvider struct {
	serviceWorker string
}

func (dsp *DefaultServiceWorkerProvider) ServiceWorker() string {
	return dsp.serviceWorker
}

// DefaultUISupport implements UISupport by embedding StyleProvider ScriptProvider and ServiceWorkerProvider
type DefaultUISupport struct {
	StyleProvider
	ScriptProvider
	ServiceWorkerProvider
}

// NewDefaultUISupport creates a DefaultUISupport from strings representing the style, the script and the serviceworker fragments for the component. Each can be "" to indicate the conponent doesn't need them.
func NewDefaultUISupport(style, script, serviceWorker string) *DefaultUISupport {
	return &DefaultUISupport{
		StyleProvider:         &DefaultStyleProvider{style: style},
		ScriptProvider:        &DefaultScriptProvider{script: script},
		ServiceWorkerProvider: &DefaultServiceWorkerProvider{serviceWorker: serviceWorker},
	}
}

type DefaultStyleInjector struct {
	Logger
	uiSupports   []UISupport
	cacheSeconds int
}

func (d *DefaultStyleInjector) Inject(app App) (template.HTML, template.HTML) {
	var buffer bytes.Buffer
	for _, uis := range d.uiSupports {
		buffer.WriteString(uis.Style())
	}
	style := buffer.Bytes()
	if style != nil {
		d.Logf("Injecting route and HTML for styles")
		ch := NewContentHandler(d.Logger, style, "text/css", "", d.cacheSeconds)
		app.Handle("/style-"+ch.Hash()+".css", ch)
		return HTMLPrintf(`
    <link rel="stylesheet" href="/style-%s.css">`, Text(url.PathEscape(ch.Hash()))), template.HTML("")
	} else {
		d.Logf("No styles specified")
		return template.HTML(""), template.HTML("")
	}
}
func NewDefaultStyleInjector(logger Logger, uiSupports []UISupport, cacheSeconds int) *DefaultStyleInjector {
	return &DefaultStyleInjector{Logger: logger, uiSupports: uiSupports, cacheSeconds: cacheSeconds}
}

type DefaultScriptInjector struct {
	Logger
	uiSupports   []UISupport
	cacheSeconds int
}

func (d *DefaultScriptInjector) Inject(app App) (template.HTML, template.HTML) {
	// Handle service worker first
	var swBuffer bytes.Buffer
	for _, sp := range d.uiSupports {
		swBuffer.WriteString(sp.ServiceWorker())
	}
	serviceWorker := swBuffer.Bytes()
	if serviceWorker != nil {
		d.Logf("Injecting route for /service-worker.js")
		// No cache for this one
		ch := NewContentHandler(d.Logger, serviceWorker, "text/javascript; charset=utf-8", "", 0)
		app.Handle("/service-worker.js", ch)
	} else {
		d.Logf("No service workers specified")
	}

	var buffer bytes.Buffer
	for _, uis := range d.uiSupports {
		buffer.WriteString(uis.Script())
	}
	if serviceWorker != nil {
		buffer.WriteString(`
/* Service Worker */

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/service-worker.js').then(registration => {
      console.log('ServiceWorker registration successful with scope: ', registration.scope);
    }, err => {
      console.log('ServiceWorker registration failed: ', err);
    });
  });
}
`)
	}
	script := buffer.Bytes()
	if script != nil {
		d.Logf("Injecting route and HTML for script")
		ch := NewContentHandler(d.Logger, script, "text/javascript; charset=utf-8", "", d.cacheSeconds)
		app.Handle("/script-"+ch.Hash()+".js", ch)
		return template.HTML(""), HTMLPrintf(`
    <script src="/script-%s.js"></script>`, Text(url.PathEscape(ch.Hash())))
	} else {
		d.Logf("No scripts specified")
		return template.HTML(""), template.HTML("")
	}
}

func NewDefaultScriptInjector(logger Logger, uiSupports []UISupport, cacheSeconds int) *DefaultScriptInjector {
	return &DefaultScriptInjector{Logger: logger, uiSupports: uiSupports, cacheSeconds: cacheSeconds}
}

type DefaultThemeColorInjector struct {
	Logger
	themeColor string
}

func (d *DefaultThemeColorInjector) Inject(app App) (template.HTML, template.HTML) {
	d.Logf("Injecting HTML for theme color")
	return HTMLPrintf(`
    <meta name="msapplication-TileColor" content="%s">
    <meta name="theme-color" content="%s">`, Text(d.themeColor), Text(d.themeColor)), template.HTML("")
}

func NewDefaultThemeColorInjector(logger Logger, themeColor string) *DefaultThemeColorInjector {
	return &DefaultThemeColorInjector{Logger: logger, themeColor: themeColor}
}

type DefaultLegacyFaviconInjector struct {
	Logger
	iconFS      fs.FS
	icon512Path string
}

func (d *DefaultLegacyFaviconInjector) Inject(app App) (template.HTML, template.HTML) {
	d.Logf("Injecting route and HTML for legacy /favicon.ico to HTML")
	fileIcon512, err := fs.ReadFile(d.iconFS, d.icon512Path)
	if err != nil {
		d.Logf("Failed to open source image for favicon: %v", err)
		return template.HTML(""), template.HTML("")
	}
	icon512, _, err := image.Decode(bytes.NewReader(fileIcon512))
	if err != nil {
		d.Logf("Failed to decode source image for favicon: %v", err)
		return template.HTML(""), template.HTML("")
	}
	app.HandleFunc("/favicon.ico", StaticFaviconHandler(d.Logger, icon512))
	return template.HTML(`
    <link rel="shortcut icon" href="/favicon.ico">`), template.HTML("")
}

func NewDefaultLegacyFaviconInjector(logger Logger, iconFS fs.FS, icon512Path string) *DefaultLegacyFaviconInjector {
	return &DefaultLegacyFaviconInjector{Logger: logger, iconFS: iconFS, icon512Path: icon512Path}
}

type DefaultSEOInjector struct {
	Logger
	description string
}

func (d *DefaultSEOInjector) Inject(app App) (template.HTML, template.HTML) {
	d.Logf("Adding HTML for SEO meta description")
	return HTMLPrintf(`
    <meta name="description" content="%s">`, Text(d.description)), template.HTML("")
}

func NewDefaultSEOInjector(logger Logger, description string) *DefaultSEOInjector {
	return &DefaultSEOInjector{Logger: logger, description: description}
}

type DefaultManifestInjector struct {
	Logger
	appShortName string
	themeColor   string
	cacheSeconds int
	startURL     string
	icons        []icon
}

type icon struct {
	Src   string `json:"src"`
	Sizes string `json:"sizes"`
	Type  string `json:"type"`
}

func (d *DefaultManifestInjector) Inject(app App) (template.HTML, template.HTML) {
	manifestData := struct {
		Name       string `json:"name"`
		ShortName  string `json:"short_name"`
		StartURL   string `json:"start_url"`
		Display    string `json:"display"`
		ThemeColor string `json:"theme_color"`
		Icons      []icon `json:"icons"`
	}{
		Name:       d.appShortName,
		ShortName:  d.appShortName,
		StartURL:   d.startURL,
		Display:    "standalone",
		ThemeColor: d.themeColor,
		Icons:      d.icons,
	}
	manifest, err := json.MarshalIndent(manifestData, "", "  ")
	if err != nil {
		d.Logf("JSON marshalling failed: %s", err)
		panic("Could not generate JSON for the manifest. Perhaps a problem with the config?")
	}
	d.Logf("Adding route for manifest")
	ch := NewContentHandler(d.Logger, manifest, "application/json", "", d.cacheSeconds)
	app.Handle("/manifest.json", ch)
	return template.HTML(`
    <link rel="manifest" href="/manifest.json">`), template.HTML("")
}

func NewDefaultManifestInjector(logger Logger, appShortName string, themeColor string, startURL string, cacheSeconds int, iconPaths map[int]string, sizes []int) (*DefaultManifestInjector, error) {
	var icons []icon

	for _, size := range sizes {
		path, exists := iconPaths[size]
		if !exists {
			// Handle the case where no path is found for a given size
			return nil, fmt.Errorf("no path found for size: %d", size)
		}

		icons = append(icons, icon{
			Src:   "/" + path,
			Sizes: fmt.Sprintf("%dx%d", size, size),
			Type:  "image/png",
		})
	}

	return &DefaultManifestInjector{Logger: logger, appShortName: appShortName, themeColor: themeColor, cacheSeconds: cacheSeconds, icons: icons, startURL: startURL}, nil

}

// Injectors prepares an HTML page string (to be used with HTMLPrintf) from a slice of Injector.
type DefaultEmptyPageProvider struct {
	page      string
	injectors []Injector
}

func (d *DefaultEmptyPageProvider) Page(title string, body template.HTML) template.HTML {
	return HTMLPrintf(d.page, Text(title), body)
}

func (d *DefaultEmptyPageProvider) PerformInjections(app App) {
	headExtra := ""
	bodyExtra := ""
	for _, injector := range d.injectors {
		h, b := injector.Inject(app)
		headExtra += strings.Replace(string(h), "%", "%%", -1)
		bodyExtra += strings.Replace(string(b), "%", "%%", -1)
	}
	d.page = `<!DOCTYPE html>
<html lang="en-GB">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>` + headExtra + `
  </head>
  <body>
%s` + bodyExtra + `
  </body>
</html>`
}

func NewDefaultEmptyPageProvider(injectors []Injector) *DefaultEmptyPageProvider {
	return &DefaultEmptyPageProvider{injectors: injectors}
}

// DefaultApp implements Server in such a way that the style, script and service worker content are only generated once. If there is any service worker then code is added to the script to register the service worker. Handlers for /script.js, /style.css and /service-worker.js are all added if needed. The server will serve from either a host and port or a UNIX domain socket based on the ServeConfigProvider.
type DefaultApp struct {
	ServeConfigProvider
	Logger
	EmptyPageProvider
	mux *http.ServeMux
}

func NewDefaultApp(serveConfigProvider ServeConfigProvider, logger Logger, emptyPageProvider EmptyPageProvider) *DefaultApp {
	app := &DefaultApp{
		ServeConfigProvider: serveConfigProvider,
		Logger:              logger,
		EmptyPageProvider:   emptyPageProvider,
		mux:                 http.NewServeMux(),
	}
	emptyPageProvider.PerformInjections(app)
	return app
}

func (app *DefaultApp) NewServices(w http.ResponseWriter, r *http.Request) Services {
	return NewDefaultServices(app.ServeConfigProvider, app.Logger, NewDefaultResponseWriterProvider(w), NewDefaultRequestProvider(r))
}

func (app *DefaultApp) Serve(ctx context.Context) {
	addr := fmt.Sprintf("%s:%d", app.Host(), app.Port())

	srv := &http.Server{
		Addr:    addr,
		Handler: app.Handler(),
	}
	// Listen for the context cancellation in a separate goroutine
	go func() {
		<-ctx.Done() // This blocks until the context is cancelled

		app.Logf("Shutting down server...")
		if err := srv.Shutdown(context.Background()); err != nil {
			app.Logf("Server shutdown failed: %v", err)
		}
	}()
	if app.UDS() != "" {
		listener, err := net.Listen("unix", app.UDS())
		if err != nil {
			app.Logf("Error listening: %v", err)
			return
		}
		app.Logf("Server listening on Unix Domain Socket: %s", app.UDS())
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			app.Logf("Server closed with error: %v", err)
			return
		}
	} else {
		app.Logf("Server listening on %s", addr)

		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			app.Logf("Server closed with error: %v", err)
			return
		}
	}
}

func (app *DefaultApp) Handler() http.Handler {
	return app.mux
}

func (app *DefaultApp) HandleFunc(path string, handler http.HandlerFunc) {
	app.mux.HandleFunc(path, handler)
}

func (app *DefaultApp) Handle(path string, handler http.Handler) {
	app.mux.Handle(path, handler)
}

func (app *DefaultApp) HandleWithServices(path string, handler func(Services)) {
	app.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		handler(app.NewServices(w, r))
	})
}
