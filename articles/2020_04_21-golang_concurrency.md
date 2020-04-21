# Concurrency Control in Golang

If we're using Python, concurrency control is simple, just like this:

```python
from concurrent.futures import ThreadPoolExecutor
import time


def loooooong_task(i):
    print("task %s sleeping..." % i)
    time.sleep(10)
    print("task %s done..." % i)


with ThreadPoolExecutor(max_workers=2) as executor:
    for i in range(10):
        executor.submit(loooooong_task, i)
```

But what can we do in Golang? maybe you want to use a [WaitGroup](https://golang.org/pkg/sync/#WaitGroup):

```go
package main

import (
	"sync"
)

type httpPkg struct{}

func (httpPkg) Get(url string) {}

var http httpPkg

func main() {
	var wg sync.WaitGroup
	var urls = []string{
		"http://www.golang.org/",
		"http://www.google.com/",
		"http://www.somestupidname.com/",
	}
	for _, url := range urls {
		// Increment the WaitGroup counter.
		wg.Add(1)
		// Launch a goroutine to fetch the URL.
		go func(url string) {
			// Decrement the counter when the goroutine completes.
			defer wg.Done()
			// Fetch the URL.
			http.Get(url)
		}(url)
	}
	// Wait for all HTTP fetches to complete.
	wg.Wait()
}
```

You can do by this to wait all goroutines, but you can't limit the concurrency with them. So what should I do?
I've met this problem when I'm writing [gotasks](https://github.com/jiajunhuang/gotasks), here is my resolution:

```go
package pool

type GoPool struct {
	MaxLimit int

	tokenChan chan struct{}
}

type GoPoolOption func(*GoPool)

func WithMaxLimit(max int) GoPoolOption {
	return func(gp *GoPool) {
		gp.MaxLimit = max
		gp.tokenChan = make(chan struct{}, gp.MaxLimit)

		for i := 0; i < gp.MaxLimit; i++ {
			gp.tokenChan <- struct{}{}
		}
	}
}

func NewGoPool(options ...GoPoolOption) *GoPool {
	p := &GoPool{}
	for _, o := range options {
		o(p)
	}

	return p
}

// Submit will wait a token, and then execute fn
func (gp *GoPool) Submit(fn func()) {
	token := <-gp.tokenChan // if there are no tokens, we'll block here

	go func() {
		fn()
		gp.tokenChan <- token
	}()
}

// Wait will wait all the tasks executed, and then return
func (gp *GoPool) Wait() {
	for i := 0; i < gp.MaxLimit; i++ {
		<-gp.tokenChan
	}

	close(gp.tokenChan)
}

func (gp *GoPool) size() int {
	return len(gp.tokenChan)
}
```

Usage is here:

```go
gopool := pool.NewGoPool(pool.WithMaxLimit(3))
defer gopool.Wait()

gopool.Submit(func() {/* your code here */})
```

Simple, right? But notice that:

- `gopool.Submit` will be blocked while there are no more tokens in `tokenChan`, but Go runtime will execute other goroutines instead of real blocking
- `gopool.Wait()` will wait for all goroutines you submit, only all of them are returned, it will return then

---

ref:

- https://github.com/jiajunhuang/gotasks/blob/master/pool/pool.go
