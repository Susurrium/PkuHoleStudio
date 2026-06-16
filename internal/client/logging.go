package client

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type loggingRoundTripper struct {
	transport http.RoundTripper
	logger    io.Writer
}

func NewLoggingRoundTripper(transport http.RoundTripper, logger io.Writer) http.RoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &loggingRoundTripper{transport: transport, logger: logger}
}

func (l *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	resp, err := l.transport.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		fmt.Fprintf(l.logger, "%s %s %s ERROR %v\n",
			start.Format("2006/01/02 15:04:05.000000"),
			req.Method, req.URL.String(), err)
		return resp, err
	}

	fmt.Fprintf(l.logger, "%s %s %s %d %s\n",
		start.Format("2006/01/02 15:04:05.000000"),
		req.Method, req.URL.String(),
		resp.StatusCode, duration)

	return resp, nil
}
