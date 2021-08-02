package main

import (
	t "github.com/Nicrii/throttle-wrapper/throttler"
	"net/http"
	"time"
)

func main() {
	throttled := t.NewThrottler(
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

	_, _ = client.Get("http://apidomain.com/network/routes") // no throttling

	req, _ := http.NewRequest("PUT", "http://apidomain.com/images/reload", nil)
	_, _ = client.Do(req) // throttling might be used

	_, _ = client.Get("http://apidomain.com/servers/1337/status?simple=true") // no throttling

}
