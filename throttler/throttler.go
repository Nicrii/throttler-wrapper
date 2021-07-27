package throttler

import (
	"net/http"
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
	queue           Queue
	count           int
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
		queue:           *NewQueue(limit),
	}

	go t.run()

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
	t.count = 0

	for i := 0; i < t.limit; i++ {
		reqCh := t.queue.Pop()
		if reqCh == nil {
			break
		}
		reqCh.Value <- struct{}{}
		t.count++
	}

	t.mutex.Unlock()
}
