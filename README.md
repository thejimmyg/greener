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

Simply import it at the top of one of the `.go` files in your module:

```
import (
	"github.com/thejimmyg/greener"
)
```

Then run:

```
go mod tidy
```

Alternatively you can install it with:

```
go get github.com/thejimmyg/greener
```

Or simply copy the git repo to your Go directory (usually `$HOME/go`) in the right place: `$HOME/go/pkg/github.com/thejimmyg/greener`.


## Examples

* [Actor](https://pkg.go.dev/github.com/thejimmyg/greener#example-package-Actor) ([src](example_actor_test.go))
* [Template](https://pkg.go.dev/github.com/thejimmyg/greener#example-package-Template) ([src](example_template_test.go))
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
