# GoGreener

The IT industry uses a lot of energy and produces a lot of CO~2~. A business
will survive as long it makes more money than it spends. This means that as
businesses grow, the commercial pressure is to grow sales more than it is to
decrease costs. This means that frequently scaling challenges are solved by
giving more money to hosting companies rather than investing in the benchmarks,
tests and refactors necessary to write efficent, hight performance code.

The hypothesis of this project is that a vast amount of that IT spend and CO~2~
production is largely unnecessary, but that the problem is hard to address once
it has happened, but if that teams are able to write efficient,
high-performance code as easily as slower code then the problem won't occur as
often and we'll all be able to make a dent in IT related carbon emissions.

**How To be Efficeint**

The most obvious way of having efficient websites, web applications and cloud
services it so avoid doing unnecessary work.

The project makes a few technology choices which are slightly different from
the current status quo but nethertheless still completely appropriate for the
vast majority of use cases.

Here are the highlights:

* Written in Go - Go is super fast and excellent at making use of all available
  processors, all while not being much more complex than Python for the use
  cases I'm targetting. Yes, C and Rust are both technically a tiny but faster but they are
  vastly harder for the kind of person that is used to Python, Ruby, or Node to
  write well.

* Use SQLite for the database back-end and bake it into the application itself -
  SQLite is the most widely deployed, tested and relied upon database in the
  world and there is a pure Go implementation too. One feature is that it
  really only allows one thread to be writing at once so wasn't traditionally
  thought of as appropriate for server use cases. By having all funcitonality
  in one binary and then having separate read and write APIs within your code
  it is trivial to implement a goroutine that queues up the writes so that only
  one part of your code is writing at once. This turns the 'limitation' into a
  feature and makes SQLite used this way blazingly fast.

* Elegantly simple API for managing UI components, progressive web apps,
  application state, config and request services.


**Some Numbers**

On my Intel i7 laptop with an NVMe drive:

* Serve 420,000 'Hello World' dynamically generated HTML pages (with stlyes,
  scripts, manifests etc) each second with 128 concurrent requests and no
  errors

* Write 240,000 times per second to an SQLite database reliably.

Compare that with a traditional Python application like Django backed by
PostgreSQL which will probably manage a few hundred to thousand requests per
second and handle a few hundred writes per second.

You can see that by adopting this architecture it is unlikely you will ever
need to scale. It also becomes completely appropriate to deploy on a shared
hosting account too.

SQLite can be combined with LiteStream or LiteFS to allow streaming backups or
live replicas if that is necessary, so you can scale out in tradional ways too.


## Install

There are a few ways of getting started:

1. Copy the git repo to your Go directory (usually `$HOME/go`) in the right place: `$HOME/go/pkg/github.com/thejimmyg/greener`.

2. Import the package at the top of one of the `.go` files in your module:

   ```
   import (
   	"github.com/thejimmyg/greener"
   )
   ```

   Then run:

   ```
   go mod tidy
   ```

3. Install it with:

   ```
   go get github.com/thejimmyg/greener
   ```

With all there approaches the git repo ends up in `$HOME/go/pkg/github.com/thejimmyg/greener` where you can use it.

## Starting your own project

If you want to create a new project that uses `greener` I'd recommend you create your own repo in GitHub and then clone it into `$HOME/go/pkg/github.com/<username>/<repo>` and then run this in the root of the repo to create `go.mod`:

```
go mod init github.com/<username>/<repo>
```

Check in `go.mod` (and `go.sum` if it exists) and then push.


If you try to do anything even slightly different from this you'll need to properly understand go workspaces, modules and pacakges and there is quite a lot to learn. If you just stick to the above everything wull 'just' work.

In some environments like gitpod you will have one place for Go and another place for your git repo. In those cases you can create a symlink from the correct place in your Go strucutre to your git repo.

e.g.

```
ln -s "$HOME/go/pkg/github.com/<username>/<repo>" path/to/git/repo
```

Obviously replace `<username>` and `<repo>` with your own values when following these instructions or trying the examples.



## Being efficient on the web

The best way to be efficient on the web is to make sure your app doesn't make requests it doesn't need to. The main approaches are:


* Bundle files together so that the browser doesn't need to make lots of separate requests for small pieces of content
* Serve a file at a path that includes a hash of its content and cache it for a long time like a year so that browsers will only need to load it once a year. If the content changes, the hash and hence the path changes so the browser will then fetch the new version
* Where you need the path to be at a fixed location use e-tag caching so that when the browser requests the file again, the server can tell it that it hasn't changed rather than sending it again.
* Detect which compression algorithms the browser supports and send compressed content where possible.
  * For static files served from a filesystem, you can pre-compress a gzipped version and then serve that if possible
  * For static content served from memory in the app you can dynamically compress it and serve the best version

Greener can help with each of these steps.

* The `UISupport` interface embeds `StyleProvider`, `ScriptProvider` and `ServiceWorkerProvider`. The `NewDefaultUISupport()` method allows you to specify `style`, `script` and `serviceWorker` content. Then `DefaultStyleInjector` and `DefaultScriptInjector` can be passed all the `UISupport`s in order to assemble a single `style.css`, `script.js` and `service-worker.js` and then to serve them either at a fixed location with etag caching and content compression (`NewContentHandler`).


Take a look at [`./cmd/advanced/main.go`](cmd/advanced/main.go) to see the injectors that use `NewContentHandler` and `StaticContentHandler` in action.

In NewContentHandler, when serving with a cache time, etags are still supported so that if the cache has expired and the content hasn't changed the server doesn't need to send it again. The `/service-worker.js` and `/manifest.json` paths are not served with a hash in their paths because browsers might re-visit the URL occasionally and won't expect it to be missing.

Injectors:

* Script (and service worker)
* Style (legacy and modern)
* Manifest
* SEO
* Icon
* Legacy favicon

The top ones should all generate brotli, gzip and original and set a forever cache returning the base64 sha512 sum of the contents, perhaps salted with a particular string.

Icon should be loaded from a specific embedded file system for just that icon and again served with a forever cache from the sha512 e.g. icons/512x512/sfdaojiafihoasdfhoasfd.png

The injectors themselves use `template.HTML` from `html/template` and combine it with code from `html.go`.

If you use the Manifest injector it will create a manifest that will load from the URL path `/start` so make sure you implement that route in your app.


## GenerateGz

There is a [`generategz`](cmd/generategz/main.go) tool that will scan a directly and pre-compress files to .gz adding a `.gz` extension. If the compressed file is actually bigger it is deleted.

You use it like this:

```
go run cmd/generategz/main.go cmd/advanced/www icons
Walking 'cmd/advanced/www' ...
Compressing 'cmd/advanced/www/file-to-compress.txt' ...
Compressing 'cmd/advanced/www/humans.txt' ...
Gzipped version is larger, so we'll delete it again
Ignoring 'cmd/advanced/www/icons/favicon-512x512.png'
```

In this case `cmd/advanced/www` is the directory where pre-compressed versions should be added and `icons` is a directory relative to that one of files to skip. You can add multiple directories to skip by adding more arguments.

Once you have pre-compressed assets in this way you can change the file serving you use from this:

```
wwwFS, _ := fs.Sub(wwwFiles, "www") // Used for the icon and the static file serving
static := greener.NewCompressedFileHandler(http.FS(wwwFS))
...
static.ServeHTTP(s.W(), s.R())
```

to:

```
greener.NewCompressedFileHandler(wwwFS)
```

In this second version, if a `.gz` version is present and the client supports it, it will be served. Otherwise the original is served. In both cases etag caching is used.


## Examples

Here are some examples.

**TIP: These are in Go's example test format. If you view the code from the generated docs, it will be correct, but if you view it from source you'll need to change the package name to `main` and the `Example_()` function name to `main()` yourself.**

* [Actor](https://pkg.go.dev/github.com/thejimmyg/greener#example-package-Actor) ([src](example_actor_test.go))
* [Template](https://pkg.go.dev/github.com/thejimmyg/greener#example-package-Template) ([src](example_template_test.go))
* [Template HTML](https://pkg.go.dev/github.com/thejimmyg/greener#example-package-Template_html) ([src](example_template_html_test.go)) (this one uses `html/template` for comparison)
* [Server](https://pkg.go.dev/github.com/thejimmyg/greener#example-package-Server) ([src](example_server_test.go))

You can also run the hello world and advanced examples yourself with:

```
go run cmd/hello/main.go
go run cmd/advanced/main.go
```

Benchmarking the Web Hello example:

```sh
wrk -t 8 -c 128 http://localhost:8000/
```
```
Running 10s test @ http://localhost:8000/
  8 threads and 128 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   428.71us    1.80ms 110.85ms   99.73%
    Req/Sec    55.58k     5.15k   85.01k    76.65%
  4452080 requests in 10.10s, 3.68GB read
Requests/sec: 440814.53
Transfer/sec:    372.89MB
```

## Batch DB

The `BatchDB` is a low level interface used by `KV` and `FTS`. It offers lightning fast SQLite access by batching writes and carefully optimising settings. This means that writes from different parts of your application actually happen in the same transaction under the hood so if one fails, all will fail. Also, there could be a couple of milliseconds delay on each individual write, in return for better throughput. These are good tradeoffs for `KV` and `FTS` where SQL calls are never expected to result in an error. It would be less good for application code where you have to be very careful that errors are correctly returned, otherwise other parts of the code using the same batch DB might silently fail.

It comes with a very simple API:

```
type DBHandler interface {
	ExecContext(context.Context, string, args ...any) (sql.Result, error)
	QueryContext(context.Context, string, args ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, args ...any) *sql.Row
}

type DBModifier interface {
	Write(func (DB) error) error
}
```

Example:

```
ctx := context.Background()
db := NewDB()

// Read only queries
db.ExecContext(ctx)
db.QueryContext(ctx)
db.QueryRowContext(ctx)


err := db.Write(func (db DBHandler) error) {
	// Batch read/write queries guaranteed to all be in the same transaction
	// The read/write db object here shadows the read-only outer db one, preventing access
	if err := db.QueryRowContext(ctx); err != nil {
		// Returning an error causes the transaction to abort and all other goroutines sharing the transaction to fail too, so you should only return errors if you want this and all other changes that might have been made by other goroutines to abort and be rolled back too
		return err
	}
	// Returning nil causes the changes to be scheduled for commit
	return nil
}
// This isn't guaranteed to be the exact error you returned in the function passed to `db.Write()`, it could be a transaction error triggered by a different goroutine that was sharing the transaction.
if err != nil {
	fmt.Printf("Batch failed to write. All goroutines that shared it are also aborted.\n")
}
```

The default implementation uses the pure Go SQLite driver, but you can switch to the C version by using ading `-tags=sqlitec` to the usual go commands, e.g.:

```
go test -tags='sqlitec sqlite_fts5'
```

If you want to see an indication of the throughput, run the tests with:

```sh
go test -v | grep 'Completed batch inserting'
```
```
Completed batch inserting 10000 greetings in 142.976767ms, 69941.433212 greetings per second
```

You'll get better throughput if you insert around 1,000,000 with a higher concurrency, but you can play with the values to see what works for you.


## KV

There is a Key Value store implementation built on top of the DB.


## Search

There is a full text search implementation build on top of the DB.


## Release

These tools should be installed:

```
go install github.com/client9/misspell/cmd/misspell@latest
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
go install github.com/gordonklaus/ineffassign@latest
go install golang.org/x/lint/golint@latest
go install golang.org/x/pkgsite/cmd/pkgsite@latest
```

and then these tests run:

```
gofmt -s -l .
go vet ./...
~/go/bin/golint ./...
~/go/bin/gocyclo .
~/go/bin/ineffassign .
~/go/bin/misspell .
```

Then run this and visit [http://localhost:8080](http://localhost:8080) to check the docs:

```
~/go/bin/pkgsite
```

Finally you can tag the remote branch with the version:

```
git push origin main
git tag v0.1.0
git push origin v0.1.0
```

## Issues

* Not all etags are properly quoted
