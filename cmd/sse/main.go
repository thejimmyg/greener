package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/thejimmyg/greener"
)

// The go:embed syntax below is a special format that tells Go to copy all the
// matched files into the binary itself so that they can be accessed without
// needing the originals any more.

// We are going to set up one embedded filesystem for the www public files, and one for the icon.

//go:embed icon-512x512.png
var iconFileFS embed.FS

type HomeHandler struct {
	greener.EmptyPageProvider
}

func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(h.Page("Hello", template.HTML(`
    <header>
        <h1>Server-Sent Events Example</h1>
    </header>
    <main>
        <form id="sseForm">
            <textarea id="inputText" placeholder="Type something..." rows="10"></textarea>
            <button type="submit">Submit</button>
        </form>
        <div id="results"></div>
    </main>
`))))
}

type StartHandler struct {
	greener.EmptyPageProvider
}

func (h *StartHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(h.Page("Start", greener.Text("This is your app's start page."))))
}

type SSEHandler struct {
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set necessary headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	// Removed the explicit Connection: keep-alive header

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for {
		// Write an event to the client
		fmt.Fprintf(w, "data: The time is %s\n\n", time.Now().String())

		// Flush the data to the client
		flusher.Flush()

		// Send events periodically (e.g., every 1 second)
		time.Sleep(1 * time.Second)
	}
}

func main() {
	// Setup
	uiSupport := []greener.UISupport{greener.NewDefaultUISupport(
		`:root {
    --header-color: #4CAF50;
    --main-color: #f0f0f0;
}

* {
    box-sizing: border-box;
}

body {
    margin: 0;
    font-family: Arial, sans-serif;
}

header {
    background-color: var(--header-color);
    color: white;
    padding: 1em;
    text-align: center;
}

main {
    background-color: var(--main-color);
    padding: 1em;
}

form {
    width: 100%;
    float: left;
    padding: 1em;
}

textarea {
    width: 100%;
    padding: 0.5em;
}

#results {
    clear: both;
    width: 100%;
    height: calc(100vh - 160px); /* Adjust based on the header and padding heights */
    padding: 1em;
    overflow-y: auto;
    background-color: white;
    border: 1px solid #ccc;
}
`,
		`
if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('service-worker.js').then((registration) => {
        console.log('Service Worker is registered.');

        navigator.serviceWorker.ready.then((registration) => {
            console.log('Service Worker is ready.');

            // Send the startSSE message to the active service worker
            if (registration.active) {
                console.log('Service Worker is active, sending startSSE message.');
                registration.active.postMessage('startSSE');
                console.log('startSSE message sent.');
            } else {
                registration.addEventListener('updatefound', () => {
                    console.log('Service Worker update found, waiting for statechange event to go to state activated.');
                    const newWorker = registration.installing;

                    if (newWorker) {
                        console.log('registering the statechange listener');
                        newWorker.addEventListener('statechange', () => {
                            console.log('Got state change:', newWorker.state);
                            if (newWorker.state === 'activated') {
                                console.log('Got state change activated, sending startSSE message');
                                newWorker.postMessage('startSSE');
                                console.log('Sent startSSE message');
                            }
                        });
                        console.log('registered the statechange listener');
                    } else {
                        console.error('No registration.installing value');
                    }
                });
            }
        }).catch((error) => {
            console.error('Service Worker ready check failed:', error);
        });
    }).catch((error) => {
        console.error('Service Worker registration failed:', error);
    });

    navigator.serviceWorker.addEventListener('message', (event) => {
        console.log('Received message:', event);
        const resultsDiv = document.getElementById('results');
        const escapedData = event.data.replace(/&/g, '&amp;')
                                      .replace(/</g, '&lt;')
                                      .replace(/>/g, '&gt;')
                                      .replace(/"/g, '&quot;')
                                      .replace(/'/g, '&#39;')
                                      .replace(/\n/g, '<br>');
        resultsDiv.innerHTML += escapedData + '<br>';
    });

    document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') {
            navigator.serviceWorker.ready.then((registration) => {
                if (registration.active) {
                    registration.active.postMessage('checkVisibility');
                }
            });
        }
    });
} else {
    console.error('Service Workers are not available in this browser tab. Perhaps they are unsupported, disabled or this is a non-localhost http URL?');
}

document.getElementById('sseForm').addEventListener('submit', function(event) {
    event.preventDefault();
    console.log('Submit form:', event);
});


`,
		`

let sseAbortController;

self.addEventListener('install', (event) => {
    console.log('Service Worker installing.');
    self.skipWaiting();
});

self.addEventListener('activate', (event) => {
    console.log('Service Worker activated.');
    self.clients.claim(); // Ensure control of all clients immediately
    checkVisibilityAndStartSSE();
});

self.addEventListener('message', (event) => {
    console.log('Service Worker received message:', event.data);
    if (event.data === 'startSSE' || event.data === 'checkVisibility') {
        checkVisibilityAndStartSSE();
    }
});

async function checkVisibilityAndStartSSE() {
    const clients = await self.clients.matchAll();
    const hasVisibleClient = clients.some(client => client.visibilityState === 'visible');
    console.log('Checking visibility. Visible client exists:', hasVisibleClient);

self.clients.matchAll().then(clients => clients.forEach(client => console.log(client.visibilityState)));

    if (hasVisibleClient && !sseAbortController) {
        startEventSource();
    } else if (!hasVisibleClient && sseAbortController) {
        sseAbortController.abort();
        sseAbortController = null;
        console.log('SSE connection closed due to no visible clients.');
    }
}


async function startEventSource() {
    sseAbortController = new AbortController();
    const signal = sseAbortController.signal;
    let timeoutId;

    try {

        const responsePromise = fetch('/events', { signal });
        timeoutId = setTimeout(() => sseAbortController.abort(), 5000); // 5-second timeout

        const response = await responsePromise;
        clearTimeout(timeoutId); // Clear the timeout if fetch succeeds

        if (!response.body) {
            console.error('ReadableStream not supported in this browser.');
            return;
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });

            const lines = buffer.split('\n');
            buffer = lines.pop(); // Keep the last partial line in buffer

            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    const data = line.slice('data: '.length);
                    console.log('SSE message received:', data);
                    broadcast(data);
                } else if (line.startsWith('event: ')) {
                    const eventType = line.slice('event: '.length);
                    console.log('SSE event received:', eventType);
                    // Handle different types of events if needed
                } else if (line === '') {
                    // A blank line means the end of the message
                    console.log('End of message');
                }
            }
        }

        console.log('SSE stream ended.');
        setTimeout(checkVisibilityAndStartSSE, 3000); // Retry after 3 seconds
    } catch (error) {
        if (timeoutId) clearTimeout(timeoutId); // Ensure timeout is cleared on error
        if (error.name !== 'AbortError') {
            console.error('SSE connection error. Retrying...', error);
            sseAbortController = null;
            setTimeout(checkVisibilityAndStartSSE, 3000); // Retry after 3 seconds if visibility conditions are met
        } else {
            console.log('SSE fetch aborted due to timeout');
            setTimeout(checkVisibilityAndStartSSE, 3000); // Retry after 3 seconds if visibility conditions are met
        }
    }
}


function broadcast(data) {
    self.clients.matchAll().then(clients => {
        clients.forEach(client => {
            console.log('Broadcasting message to client:', data);
            client.postMessage(data);
        });
    }).catch(error => {
        console.error('Error broadcasting message:', error);
    });
}
`,
	)}
	themeColor := "#000000"
	appShortName := "Simple"
	config := greener.NewDefaultServeConfigProviderFromEnvironment()
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

	mux.Handle("/", &HomeHandler{EmptyPageProvider: emptyPageProvider})
	// This is loaded based on the injected manifest.json when the user opens your app in PWA mode
	mux.Handle("/start", &StartHandler{EmptyPageProvider: emptyPageProvider})
	mux.Handle("/events", &SSEHandler{})

	// Serve
	greener.Serve(context.Background(), logger, mux, config)
}
