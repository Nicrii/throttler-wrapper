package main

import (
	"bytes"
	"errors"
	"regexp"
	"strings"

	//"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type throttler struct {
	http.RoundTripper
	limit           int
	interval        time.Duration
	methods         []string
	allowedPrefixes []string
	exceptions      []string
	fastReturn      bool
	queue           Queue
	count           int
	mux             sync.Mutex
}

func NewThrottler(roundTripper http.RoundTripper, limit int, interval time.Duration, methods, urlPrefixes, exceptions []string, fastReturn bool) *throttler {
	t := &throttler{
		RoundTripper:    roundTripper,
		limit:           limit,
		interval:        interval,
		methods:         methods,
		allowedPrefixes: getRegex(urlPrefixes),
		exceptions:      getRegex(exceptions),
		fastReturn:      fastReturn,
		queue:           *NewQueue(limit),
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
func getRegex(strs []string) []string {
	result := make([]string, len(strs))

	for i, str := range strs {
		if len(str) == 0 {
			continue
		}

		var strsSplited []string
		var regex bytes.Buffer

		strsSplited = append(strsSplited, strings.Split(str, "*")...)
		regex.WriteString("^")

		for i, expr := range strsSplited {
			regex.WriteString(expr)

			if strsSplited[i] == "" || i+1 < len(strsSplited) && strsSplited[i+1] != "" {
				regex.WriteString(".*")
			}
		}

		regex.WriteString("$")
		result[i] = regex.String()
	}

	return result
}

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
			fmt.Println(req.URL.Path)
			if t.limit == 0 { //если не удовлетворяет условиям //проверка на префиксы и методы
				return t.RoundTripper.RoundTrip(req)
			}

			for _, exception := range t.exceptions {
				match, err := regexp.MatchString(exception, req.URL.String())
				if err != nil {
					return resp, err
				}
				if match {
					return t.RoundTripper.RoundTrip(req)
				}
			}

			for _, urlPrefix := range t.allowedPrefixes {
				match, err := regexp.MatchString(urlPrefix, req.URL.String())
				if err != nil {
					return resp, err
				}
				if !match {
					return t.RoundTripper.RoundTrip(req)
				}
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
		[]string{},                        // except servers status and network operations
		false,                             // wait on limit
	)

	var roundTrip Decorator = AppendDecorator(throttler)

	throttled := Decorate(throttler, []Decorator{roundTrip}...)

	client := http.Client{
		Transport: throttled,
	}

	_, _ = client.Get("http://apidomain.com/servers/1337/status?simple=true")
	//_, _ = client.Do(req)

	req, _ := http.NewRequest("PUT", "http://apidomain.com/network/", nil)
	_, _ = client.Do(req) // throttling might be used

	req, _ = http.NewRequest("PUT", "http://apidomain.com/servers/153/status", nil)
	_, _ = client.Do(req) // throttling might be used
	// ...

}
