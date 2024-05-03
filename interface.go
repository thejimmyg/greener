package greener

import (
	"context"
	"html/template"
	"net/http"
)

// Injector
type Injector interface {
	Inject(App) (template.HTML, template.HTML)
}

// EmptyPageProvider
type EmptyPageProvider interface {
	PerformInjections(App)
	Page(title string, body template.HTML) template.HTML
}

// Logger interface
type Logger interface {
	Logf(string, ...interface{})
	Errorf(string, ...interface{})
}

// ResponseWriterProvider interface
type ResponseWriterProvider interface {
	W() http.ResponseWriter
}

// RequestProvider interface
type RequestProvider interface {
	R() *http.Request
}

// ServeConfigProvider interface for server configuration handling
type ServeConfigProvider interface {
	Host() string
	Port() int
	UDS() string
}

// Server interface
type HandleFuncProvider interface {
	HandleFunc(string, http.HandlerFunc)
}
type HandleProvider interface {
	Handle(string, http.Handler)
}
type HandleWithServicesProvider interface {
	HandleWithServices(string, func(Services))
}
type Server interface {
	ServeConfigProvider
	Serve(context.Context)
	Handler() http.Handler
}

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

type NewServicesProvider interface {
	NewServices(http.ResponseWriter, *http.Request) Services
}

type App interface {
	Logger
	Server
	HandleProvider
	HandleFuncProvider
	HandleWithServicesProvider
	EmptyPageProvider
	NewServicesProvider
}

type Services interface {
	Logger
	ResponseWriterProvider
	RequestProvider
}
