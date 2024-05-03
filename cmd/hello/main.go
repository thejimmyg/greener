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
	app.HandleWithServices("/", func(s greener.Services) {
		s.W().Write([]byte(app.Page("Hello", greener.Text("Hello <>!"))))
	})
	app.Serve(context.Background())
}
