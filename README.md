<p align="center"><a href="#readme"><img src="https://gh.kaos.st/go-redy.svg"/></a></p>

<p align="center">
  <a href="https://godoc.org/pkg.re/essentialkaos/redy.v4"><img src="https://godoc.org/pkg.re/essentialkaos/redy.v4?status.svg"></a>
  <a href="https://goreportcard.com/report/github.com/essentialkaos/redy"><img src="https://goreportcard.com/badge/github.com/essentialkaos/redy"></a>
  <a href="https://codebeat.co/projects/github-com-essentialkaos-redy-master"><img alt="codebeat badge" src="https://codebeat.co/badges/1398d17c-e335-43c7-92d7-3aa484b2454c" /></a>
  <a href="https://travis-ci.org/essentialkaos/redy"><img src="https://travis-ci.org/essentialkaos/redy.svg"></a>
  <a href='https://coveralls.io/github/essentialkaos/redy'><img src='https://coveralls.io/repos/github/essentialkaos/redy/badge.svg' alt='Coverage Status' /></a>
  <a href="https://github.com/essentialkaos/redy/blob/master/LICENSE"><img src="https://gh.kaos.st/mit.svg"></a>
</p>

<p align="center"><a href="#installation">Installation</a> • <a href="#usage-example">Usage example</a> • <a href="#build-status">Build Status</a> • <a href="#license">License</a></p>

<br/>

`redy` is a tiny Redis client based on [radix.v2](https://github.com/mediocregopher/radix.v2) code base.

### Installation

Before the initial install, allow git to use redirects for [pkg.re](https://github.com/essentialkaos/pkgre) service (_the reason why you should do this described [here](https://github.com/essentialkaos/pkgre#git-support)_):

```
git config --global http.https://pkg.re.followRedirects true
```

Make sure you have a working Go 1.10+ workspace (_[instructions](https://golang.org/doc/install)_), then:

```
go get pkg.re/essentialkaos/redy.v4
```

For update to latest stable release, do:

```
go get -u pkg.re/essentialkaos/redy.v4
```

### Usage example
```go
package main

import (
  "fmt"
  "time"

  "pkg.re/essentialkaos/redy.v4"
)

func main() {
  rc := redy.Client{
    Network:     "tcp",
    Addr:        "127.0.0.1:6379",
    DialTimeout: 15 * time.Second,
  }

  err := rc.Connect()

  if err != nil {
    fmt.Printf("Connection error: %v\n", err)
    return
  }

  r := rc.Cmd("SET", "ABC", 1)

  if r.Err != nil {
    fmt.Printf("Command error: %v\n", r.Err)
    return
  }

  r = rc.Cmd("GET", "ABC")

  if r.Err != nil {
    fmt.Printf("Command error: %v\n", r.Err)
    return
  }

  val, err := r.Int()

  if err != nil {
    fmt.Printf("Parsing error: %v\n", err)
    return
  }

  fmt.Printf("ABC -> %d\n", val)
}
```

### Build Status

| Branch     | Status |
|------------|--------|
| `master` (_Stable_) | [![Build Status](https://travis-ci.org/essentialkaos/redy.svg?branch=master)](https://travis-ci.org/essentialkaos/redy) |
| `develop` (_Unstable_) | [![Build Status](https://travis-ci.org/essentialkaos/redy.svg?branch=develop)](https://travis-ci.org/essentialkaos/redy) |

### License

[MIT](LICENSE)

<p align="center"><a href="https://essentialkaos.com"><img src="https://gh.kaos.st/ekgh.svg"/></a></p>
