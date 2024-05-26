package main

import (
	"github.com/thejimmyg/greener"
	"log"
	"net/http"
)

type HomeHandler struct {
	greener.Logger
}

func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Logf("Handling a request to %s by returning a message", r.URL.Path)
	w.Write([]byte("Hello, world!"))
}

func main() {
	logger := greener.NewDefaultLogger(log.Printf)
	mux := http.NewServeMux()
	mux.Handle("/", &HomeHandler{Logger: logger})
	err, ctx, _ := greener.AutoServe(logger, mux)
	if err != nil {
		panic(err)
	}
	<-ctx.Done()
}
