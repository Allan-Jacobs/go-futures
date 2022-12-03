# Go-Futures

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

An easy to use generic future implementation in Go.

### Install

```sh
get get github.com/Allan-Jacobs/go-futures@latest
```

### Example

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/Allan-Jacobs/go-futures/futures"
)

// HTTPGetAsync wraps http.Get into a future based api
func HTTPGetAsync(url string) futures.Future[*http.Response] {
	return futures.GoroutineFuture(func() (*http.Response, error) {
		return http.Get(url)
	})
}

func main() {
	// run futures simultaneously and await aggregated results
	responses, err := futures.All(
		HTTPGetAsync("https://go.dev"),
		HTTPGetAsync("https://pkg.dev"),
	).Await()
	if err != nil {
		fmt.Println("Error:", err)
	}
	for _, res := range responses {
		fmt.Println(res.Request.URL, res.Status)
	}
}
```

### License

go-futures is [MIT Licensed](https://github.com/Allan-Jacobs/go-futures/blob/master/LICENCE)
