package main

import (
	"container/list"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type throttler struct {
	window FixedWindowInterval
	queue  list.List
	f      <-chan struct{}
	limit  int
	count  int
	mux    sync.Mutex
}

func NewThrottler(limit int, interval int) *throttler {
	var window FixedWindowInterval
	f := make(chan struct{}, limit)
	window.interval = time.Second * 4
	t := &throttler{
		limit:  limit,
		window: window,
		f:      f,
	}
	go t.run()
	return t
}

type FixedWindowInterval struct {
	startTime time.Time
	endTime   time.Time
	interval  time.Duration
}

func (w *FixedWindowInterval) setWindowTime() {
	w.startTime = time.Now().UTC()
	w.endTime = time.Now().UTC().Add(w.interval)
}

func (t *throttler) run() {
	go func() {
		ticker := time.NewTicker(t.window.interval)
		t.window.setWindowTime()
		for range ticker.C {
			t.releaseRequests()
			t.window.setWindowTime()
		}
	}()
}

func (t *throttler) releaseRequests() {
	fmt.Printf("начан вывод из очереди\n")

	t.mux.Lock()
	length := t.queue.Len()
	t.count = 0
	for i := 0; i < t.limit && i < length; i++ {
		t.queue.Front().Value

		t.queue = t.queue[1:] //сделать потокобезопасно
		t.count++
	}
	t.mux.Unlock()

}

func (t *throttler) RoundTrip(name int) (resp *http.Response, err error) { //?????????
	if t.limit == 0 { //если не удовлетворяет условиям //проверка на префиксы и методы
		return resp, err
	}

	if t.count < t.limit { ///сделать потоко безопасно
		t.mux.Lock()
		t.count++
		t.mux.Unlock()
		fmt.Printf("Поток выведен без очереди %d\n", name)
		return resp, err
	} else {
		ch := make(chan struct{})
		t.mux.Lock()
		t.queue = append(t.queue, ch) //сделать потоко безопасно
		t.mux.Unlock()
		<-ch
		fmt.Printf("Поток выведен из очереди %d\n", name)
		return resp, err
	}

	return resp, err
}

func main() {
	t := NewThrottler(5, 2)

	for i := 1; i <= 12; i++ {
		go t.RoundTrip(i * 10)
	}
	time.Sleep(4 * time.Second)
	for i := 1; i <= 12; i++ {
		t.RoundTrip(i)
	}

	time.Sleep(20 * time.Second)
}
