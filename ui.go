package greener

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
)

// UISupport
type StyleProvider interface {
	Style() string
}

type ScriptProvider interface {
	Script() string
}

type ServiceWorkerProvider interface {
	ServiceWorker() string
}

type UISupport interface {
	StyleProvider
	ScriptProvider
	ServiceWorkerProvider
}

// Injector
type Injector interface {
	InjectHandlers(HandlerRouter)
	InjectHTML(string, *HTMLBuilder, *HTMLBuilder)
}

type HandlerRouter interface {
	Handle(string, http.Handler)
}

type CaptureResponseWriter struct {
	body *bytes.Buffer
}

func (crw *CaptureResponseWriter) Write(b []byte) (int, error) {
	return crw.body.Write(b)
}

func (crw *CaptureResponseWriter) Header() http.Header {
	return http.Header{}
}

func (crw *CaptureResponseWriter) WriteHeader(statusCode int) {
	// Not needed for simple capture
}

type CaptureMux struct {
	Responses map[string]string
}

func NewCaptureMux() *CaptureMux {
	return &CaptureMux{
		Responses: make(map[string]string),
	}
}

func (cm *CaptureMux) Handle(path string, handler http.Handler) {
	// Create a buffer to capture the response
	crw := &CaptureResponseWriter{
		body: new(bytes.Buffer),
	}

	// Simulate a request to handle
	handler.ServeHTTP(crw, &http.Request{})

	// Store the response associated with the path
	cm.Responses[path] = crw.body.String()
}

// EmptyPageProvider
type EmptyPageProvider interface {
	PerformInjections(HandlerRouter)
	Page(title string, body template.HTML, currentPath string) template.HTML
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
	link         string
}

func (d *DefaultStyleInjector) InjectHTML(currentPath string, headExtra *HTMLBuilder, bodyExtra *HTMLBuilder) {
	rel := RelativePath(currentPath, "/")
	headExtra.Printf(d.link, Text(rel))
}

func (d *DefaultStyleInjector) InjectHandlers(mux HandlerRouter) {
	var buffer bytes.Buffer
	for _, uis := range d.uiSupports {
		buffer.WriteString(uis.Style())
	}
	style := buffer.Bytes()
	if style != nil {
		d.Logf("Injecting route and HTML for styles")
		ch := NewContentHandler(d.Logger, style, "text/css", "", d.cacheSeconds)
		if mux != nil {
			mux.Handle("/style-"+ch.Hash()+".css", ch)
		}
		d.link = fmt.Sprintf(`
    <link rel="stylesheet" href="%%sstyle-%s.css">`, Text(url.PathEscape(ch.Hash())))
	} else {
		d.Logf("No styles specified")
	}
}
func NewDefaultStyleInjector(logger Logger, uiSupports []UISupport, cacheSeconds int) *DefaultStyleInjector {
	return &DefaultStyleInjector{Logger: logger, uiSupports: uiSupports, cacheSeconds: cacheSeconds}
}

type DefaultScriptInjector struct {
	Logger
	uiSupports   []UISupport
	cacheSeconds int
	script       string
}

func (d *DefaultScriptInjector) InjectHTML(currentPath string, headExtra *HTMLBuilder, bodyExtra *HTMLBuilder) {
	rel := RelativePath(currentPath, "/")
	bodyExtra.Printf(d.script, Text(rel))
}

func (d *DefaultScriptInjector) InjectHandlers(mux HandlerRouter) {
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
		if mux != nil {
			mux.Handle("/service-worker.js", ch)
		}
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
		if mux != nil {
			mux.Handle("/script-"+ch.Hash()+".js", ch)
		}
		d.script = fmt.Sprintf(`
    <script src="%%sscript-%s.js"></script>`, Text(url.PathEscape(ch.Hash())))
	} else {
		d.Logf("No scripts specified")
	}
}

func NewDefaultScriptInjector(logger Logger, uiSupports []UISupport, cacheSeconds int) *DefaultScriptInjector {
	return &DefaultScriptInjector{Logger: logger, uiSupports: uiSupports, cacheSeconds: cacheSeconds}
}

type DefaultThemeColorInjector struct {
	Logger
	themeColor string
	html       template.HTML
}

func (d *DefaultThemeColorInjector) InjectHTML(currentPath string, headExtra *HTMLBuilder, bodyExtra *HTMLBuilder) {
	headExtra.WriteHTML(d.html)
}
func (d *DefaultThemeColorInjector) InjectHandlers(mux HandlerRouter) {
	d.Logf("Injecting HTML for theme color")
	d.html = HTMLPrintf(`
    <meta name="msapplication-TileColor" content="%s">
    <meta name="theme-color" content="%s">`, Text(d.themeColor), Text(d.themeColor))
}

func NewDefaultThemeColorInjector(logger Logger, themeColor string) *DefaultThemeColorInjector {
	return &DefaultThemeColorInjector{Logger: logger, themeColor: themeColor}
}

type DefaultSEOInjector struct {
	Logger
	description string
	html        template.HTML
}

func (d *DefaultSEOInjector) InjectHTML(currentPath string, headExtra *HTMLBuilder, bodyExtra *HTMLBuilder) {
	headExtra.WriteHTML(d.html)
}

func (d *DefaultSEOInjector) InjectHandlers(mux HandlerRouter) {
	d.Logf("Adding HTML for SEO meta description")
	d.html = HTMLPrintf(`
    <meta name="description" content="%s">`, Text(d.description))
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
	manifest     string
}

type icon struct {
	Src   string `json:"src"`
	Sizes string `json:"sizes"`
	Type  string `json:"type"`
}

func (d *DefaultManifestInjector) InjectHTML(currentPath string, headExtra *HTMLBuilder, bodyExtra *HTMLBuilder) {
	rel := RelativePath(currentPath, "/")
	headExtra.Printf(d.manifest, Text(rel))
}
func (d *DefaultManifestInjector) InjectHandlers(mux HandlerRouter) {
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
	if mux != nil {
		mux.Handle("/manifest.json", ch)
	}
	d.manifest = fmt.Sprintf(`
    <link rel="manifest" href="%%smanifest.json">`)
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

func (d *DefaultEmptyPageProvider) Page(title string, body template.HTML, currentPage string) template.HTML {
	headExtra := &HTMLBuilder{}
	bodyExtra := &HTMLBuilder{}
	for _, injector := range d.injectors {
		injector.InjectHTML(currentPage, headExtra, bodyExtra)
	}
	return HTMLPrintf(`<!DOCTYPE html>
<html lang="en-GB">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>%s
  </head>
  <body>
%s%s
  </body>
</html>`, Text(title), headExtra.HTML(), body, bodyExtra.HTML())
}

func (d *DefaultEmptyPageProvider) PerformInjections(mux HandlerRouter) {
	for _, injector := range d.injectors {
		injector.InjectHandlers(mux)
	}
}

func NewDefaultEmptyPageProvider(injectors []Injector) *DefaultEmptyPageProvider {
	return &DefaultEmptyPageProvider{injectors: injectors}
}

func RelativePath(currentPath, target string) string {
	rel, _ := filepath.Rel(filepath.Dir(currentPath), target)
	if rel == "." {
		rel = ""
	} else {
		rel = rel + "/"
	}
	// fmt.Printf("%s %s\n", currentPath, rel)
	return rel
}
