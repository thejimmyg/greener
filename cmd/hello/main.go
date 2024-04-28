package main

import (
	"context"
	"log"

	"github.com/thejimmyg/greener"
)

func main() {
	app := greener.NewDefaultApp(
		greener.NewDefaultServeConfigProviderFromEnvironment(),
		greener.NewDefaultLogger(log.Printf),
		greener.NewDefaultEmptyPageProvider([]greener.Injector{}),
	)
	app.Handle("/", func(s greener.Services) {
		app.Page("Hello", greener.Text("Hello <>!")).WriteHTMLTo(s.W())
	})
	app.Serve(context.Background())
}
