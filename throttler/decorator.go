package throttler

import (
	"errors"
	"net/http"
	"regexp"
)

type Decorated http.RoundTripper
type RoundTripFunc func(*http.Request) (*http.Response, error)
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

func AppendDecorator(t *Throttler) Decorator {
	return func(tripper http.RoundTripper) http.RoundTripper {
		return RoundTripFunc(func(req *http.Request) (resp *http.Response, err error) {
			if t.limit == 0 {
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

			for _, exception := range t.exceptions {
				match, err := regexp.MatchString(exception, req.URL.Path)
				if err != nil {
					return resp, err
				}
				if match {
					return t.RoundTripper.RoundTrip(req)
				}
			}

			for _, urlPrefix := range t.allowedPrefixes {
				match, err := regexp.MatchString(urlPrefix, req.URL.Path)
				if err != nil {
					return resp, err
				}
				if !match {
					return t.RoundTripper.RoundTrip(req)
				}
			}

			if t.count < t.limit {
				t.mutex.Lock()
				t.count++
				t.mutex.Unlock()
			} else if t.fastReturn {
				return resp, errors.New("Request limit exceeded\n")
			} else {
				t.mutex.Lock()
				t.waiting++
				t.mutex.Unlock()
			}

			t.ch <- struct{}{}
			return t.RoundTripper.RoundTrip(req)
		})
	}
}
