package throttler

import (
	"errors"
	"net/http"
	"regexp"
	"sync"
	"time"
)

type Throttler struct {
	http.RoundTripper
	limit           int
	interval        time.Duration
	methods         []string
	allowedPrefixes []string
	exceptions      []string
	fastReturn      bool
	ch              chan struct{}
	count           int
	waiting         int
	released        int
	mutex           sync.Mutex
}

func NewThrottler(roundTripper http.RoundTripper, limit int, interval time.Duration, methods, urlPrefixes, exceptions []string, fastReturn bool) *Throttler {

	t := &Throttler{
		RoundTripper:    roundTripper,
		limit:           limit,
		interval:        interval,
		methods:         methods,
		allowedPrefixes: getRegex(urlPrefixes),
		exceptions:      getRegex(exceptions),
		fastReturn:      fastReturn,
		ch:              make(chan struct{}, limit),
	}

	t.run()

	return t
}

func (t *Throttler) run() {
	go func() {
		ticker := time.NewTicker(t.interval)
		for range ticker.C {
			t.releaseRequests()
		}
	}()
}

func (t *Throttler) releaseRequests() {
	t.mutex.Lock()
	i := 0

	for ; i < t.count-t.released; i++ {
		<-t.ch
	}

	t.count, t.released = 0, 0

	for ; i < t.limit && t.waiting > 0; i++ {
		t.waiting--
		<-t.ch
		t.released++
		t.count++
	}

	t.mutex.Unlock()
}

func (t *Throttler) RoundTrip(req *http.Request) (resp *http.Response, err error) {
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
}
