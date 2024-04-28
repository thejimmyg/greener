# Go Greener

Copy `greener` to `~/go/pkg/github.com/thejimmyg/greener` then run:

```
go run cmd/wiki/main.go
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
