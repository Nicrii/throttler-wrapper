package main

import (
	"errors"
	//"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type throttler struct {
	http.RoundTripper
	limit       int
	interval    time.Duration
	methods     []string
	urlPrefixes []string
	exceptions  []string
	fastReturn  bool
	queue       Queue
	count       int
	mux         sync.Mutex
}

func NewThrottler(roundTripper http.RoundTripper, limit int, interval time.Duration, methods, urlPrefixes, exceptions []string, fastReturn bool) *throttler {
	t := &throttler{
		RoundTripper: roundTripper,
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

//
//func (t *throttler) RoundTrip(request *http.Request) (*http.Response, error) {
//
//}

//func (t *throttler)RoundTrip(req *http.Request) (resp *http.Response, err error) {
//	return throttler.RoundTrip()
//}

//func (t *throttler) RoundTrip(req *http.Request) (resp *http.Response, err error) { //?????????
//	if t.limit == 0 {                                                               //если не удовлетворяет условиям //проверка на префиксы и методы
//		return t.RoundTripper.RoundTrip(req)
//	}
//	if len(t.methods) != 0 {
//		shouldReturn := true
//		for _, m := range t.methods {
//			if m == req.Method {
//				shouldReturn = false
//				break
//			}
//		}
//		if shouldReturn {
//			return t.RoundTripper.RoundTrip(req)
//		}
//	}
//
//	if t.count < t.limit { ///сделать потоко безопасно
//		t.mux.Lock()
//		t.count++
//		t.mux.Unlock()
//		return t.RoundTripper.RoundTrip(req)
//	} else {
//		if t.fastReturn {
//			if resp.Body != nil {
//				resp.Body.Close()
//			}
//			return resp, errors.New("Request limit exceeded\n")
//		}
//		ch := make(chan struct{})
//		t.mux.Lock()
//		t.queue.Push(&Node{Value: ch}) //сделать потоко безопасно
//		t.mux.Unlock()
//		<-ch
//		return t.RoundTripper.RoundTrip(req)
//	}
//}

type Decorated http.RoundTripper
type RoundTripFunc func(*http.Request) (*http.Response, error) // this is the type of functions you want to decorate
type Decorator func(tripper http.RoundTripper) http.RoundTripper

func Decorate(c Decorated, ds ...Decorator) Decorated {
	decorated := c
	for _, decorator := range ds {
		decorated = decorator(decorated)
	}
	return decorated
}

func (r RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return r(req)
}

func AppendDecorator(t *throttler) Decorator {
	return func(tripper http.RoundTripper) http.RoundTripper {
		return RoundTripFunc(func(req *http.Request) (resp *http.Response, err error) {
			if t.limit == 0 { //если не удовлетворяет условиям //проверка на префиксы и методы
				return t.RoundTripper.RoundTrip(req)
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
					return t.RoundTripper.RoundTrip(req)
				}
			}

			if t.count < t.limit { //не дошло до лимита, выводится без очереди
				fmt.Println("запрос без очереди")
				t.mux.Lock()
				t.count++
				t.mux.Unlock()
				return t.RoundTripper.RoundTrip(req)
			} else {
				if t.fastReturn {
					return resp, errors.New("Request limit exceeded\n")
				}
				ch := make(chan struct{})
				t.mux.Lock()
				t.queue.Push(&Node{Value: ch}) //сделать потоко безопасно
				t.mux.Unlock()
				<-ch
				fmt.Println("запрос в очереди")
				return t.RoundTripper.RoundTrip(req)
			}
		})
	}
}

func main() {
	throttler := NewThrottler(
		http.DefaultTransport,
		5,
		time.Second*10,                    // 60 rpm
		[]string{"POST", "PUT", "DELETE"}, // limit only POST, PUT, DELETE requests
		nil,                               // use for all URLs
		[]string{"/servers/*/status", "/network/"}, // except servers status and network operations
		false, // wait on limit
	)

	var roundTrip Decorator = AppendDecorator(throttler)

	throttled := Decorate(throttler, []Decorator{roundTrip}...)

	client := http.Client{
		Transport: throttled,
	}

	// ...
	for i := 0; i < 12; i++ {
		go func() {
			req, _ := http.NewRequest("PUT", "http://apidomain.com/images/reload", nil)
			_, _ = client.Do(req)
		}()
	}
	_, _ = client.Get("http://apidomain.com/network/routes") // no throttling
	// ...
	req, _ := http.NewRequest("PUT", "http://apidomain.com/images/reload", nil)
	_, _ = client.Do(req) // throttling might be used
	// ...
	_, _ = client.Get("http://apidomain.com/servers/1337/status?simple=true") // no throttling
}
