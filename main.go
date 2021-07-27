package main

import (
	"net/http"
	t "throttle-wrapper/throttler"
	"time"
)

func main() {
	throttler := t.NewThrottler(
		http.DefaultTransport,
		60,
		time.Minute,                       // 60 rpm
		[]string{"POST", "PUT", "DELETE"}, // limit only POST, PUT, DELETE requests
		nil,                               // use for all URLs
		[]string{"/servers/*/status", "/network/"}, // except servers status and network operations
		false, // wait on limit
	)

	var roundTrip t.Decorator = t.AppendDecorator(throttler)

	throttled := t.Decorate(throttler, []t.Decorator{roundTrip}...)

	client := http.Client{
		Transport: throttled,
	}

	_, _ = client.Get("http://apidomain.com/network/routes") // no throttling

	req, _ := http.NewRequest("PUT", "http://apidomain.com/images/reload", nil)
	_, _ = client.Do(req) // throttling might be used

	_, _ = client.Get("http://apidomain.com/servers/1337/status?simple=true") // no throttling

}
