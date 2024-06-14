<p align="center"><a href="#readme"><img src=".github/images/card.svg"/></a></p>

<p align="center">
  <a href="https://kaos.sh/g/redy.v4"><img src=".github/images/godoc.svg"/></a>
  <a href="https://kaos.sh/r/redy"><img src="https://kaos.sh/r/redy.svg" alt="GoReportCard" /></a>
  <a href="https://kaos.sh/b/redy"><img src="https://kaos.sh/b/1398d17c-e335-43c7-92d7-3aa484b2454c.svg" alt="Codebeat badge" /></a>
  <a href="https://kaos.sh/w/redy/ci"><img src="https://kaos.sh/w/redy/ci.svg" alt="GitHub Actions CI Status" /></a>
  <a href="https://kaos.sh/w/redy/codeql"><img src="https://kaos.sh/w/redy/codeql.svg" alt="GitHub Actions CodeQL Status" /></a>
  <a href="https://kaos.sh/c/redy"><img src="https://kaos.sh/c/redy.svg" alt="Coverage Status" /></a>
  <a href="#license"><img src=".github/images/license.svg"/></a>
</p>

<p align="center"><a href="#usage-example">Usage example</a> • <a href="#ci-status">CI Status</a> • <a href="#license">License</a></p>

<br/>

`redy` is a tiny Redis client based on [radix.v2](https://github.com/mediocregopher/radix.v2) code base.

### Usage example
```go
package main

import (
  "fmt"
  "time"

  "github.com/essentialkaos/redy/v4"
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

### CI Status

| Branch     | Status |
|------------|--------|
| `master` | [![CI](https://kaos.sh/w/redy/ci.svg?branch=master)](https://kaos.sh/w/redy/ci?query=branch:master) |
| `develop` | [![CI](https://kaos.sh/w/redy/ci.svg?branch=develop)](https://kaos.sh/w/redy/ci?query=branch:develop) |

### License

[MIT](LICENSE)

<p align="center"><a href="https://essentialkaos.com"><img src="https://gh.kaos.st/ekgh.svg"/></a></p>
