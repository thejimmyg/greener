# Go Greener

Copy `greener` to `~/go/pkg/github.com/thejimmyg/greener` then run:

```
go run cmd/hello/main.go
go run cmd/simple/main.go
```

Benchmark:

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
