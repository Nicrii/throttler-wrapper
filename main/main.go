package main

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type throttler struct {
	roundTripper http.RoundTripper
	limit        int
	interval     time.Duration
	methods      []string
	urlPrefixes  []string
	exceptions   []string
	fastReturn   bool
	queue        Queue
	count        int
	mux          sync.Mutex
}

func NewThrottler(roundTripper http.RoundTripper, limit int, interval time.Duration, methods, urlPrefixes, exceptions []string, fastReturn bool) *throttler {
	t := &throttler{
		roundTripper: roundTripper,
		limit:        limit,
		interval:     interval,
		methods:      methods,
		urlPrefixes:  urlPrefixes,
		exceptions:   exceptions,
		fastReturn:   fastReturn,
		queue:        *NewQueue(limit),
	}
	go t.run()
	return t
}

func (t *throttler) run() {
	go func() {
		ticker := time.NewTicker(t.interval)
		for range ticker.C {
			t.releaseRequests()
		}
	}()
}

func (t *throttler) releaseRequests() {
	fmt.Printf("начан вывод из очереди\n")
	t.mux.Lock()
	t.count = 0
	for i := 0; i < t.limit; i++ {
		reqCh := t.queue.Pop()
		if reqCh == nil {
			break
		}
		reqCh.Value <- struct{}{}
		t.count++
	}
	t.mux.Unlock()
}

func (t *throttler) RoundTrip(req *http.Request) (resp *http.Response, err error) { //?????????
	if t.limit == 0 { //если не удовлетворяет условиям //проверка на префиксы и методы
		return t.roundTripper.RoundTrip(req)
	}
	if len(t.methods) != 0 {
		shouldReturn := true
		for _, m := range t.methods {
			if m == req.Method {
				shouldReturn = false
				break
			}
		}
		if shouldReturn {
			return t.roundTripper.RoundTrip(req)
		}
	}

	if t.count < t.limit { ///сделать потоко безопасно
		t.mux.Lock()
		t.count++
		t.mux.Unlock()
		return resp, err
	} else {
		if t.fastReturn {
			resp.Body.Close()
			return resp, errors.New("Request limit exceeded\n")
		}
		ch := make(chan struct{})
		t.mux.Lock()
		t.queue.Push(&Node{Value: ch}) //сделать потоко безопасно
		t.mux.Unlock()
		<-ch
		return resp, err
	}

	return resp, err
}

func main() {
	throttled := NewThrottler(
		http.DefaultTransport,
		60,
		time.Minute,                       // 60 rpm
		[]string{"POST", "PUT", "DELETE"}, // limit only POST, PUT, DELETE requests
		nil,                               // use for all URLs
		[]string{"/servers/*/status", "/network/"}, // except servers status and network operations
		false, // wait on limit
	)
	client := http.Client{
		Transport: throttled,
	}

	// ...
	resp, err := client.Get("http://apidomain.com/network/routes") // no throttling
	// ...
	req, _ := http.NewRequest("PUT", "http://apidomain.com/images/reload", nil)
	resp, err = client.Do(req) // throttling might be used
	// ...
	resp, err = client.Get("http://apidomain.com/servers/1337/status?simple=true") // no throttling
}
