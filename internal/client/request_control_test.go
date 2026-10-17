package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestControlledTransportCountsActualRequestsIncludingFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/failure" {
			w.WriteHeader(404)
		}
		_, _ = fmt.Fprint(w, "body")
	}))
	defer server.Close()
	gate := newControlledTransport(http.DefaultTransport, nil)
	if gate.interval != time.Second {
		t.Fatalf("production interval = %v", gate.interval)
	}
	gate.interval = 0
	web := &http.Client{Transport: gate}
	budget := NewRequestBudget(100)
	for index := 0; index < 101; index++ {
		req, err := http.NewRequestWithContext(WithRequestBudget(context.Background(), budget), "GET", server.URL+"/failure", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := web.Do(req)
		if index == 100 {
			if !errors.Is(err, ErrRequestBudget) {
				t.Fatalf("request 101 = %v", err)
			}
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	if calls.Load() != 100 || budget.Remaining() != 0 {
		t.Fatalf("calls=%d budget=%d", calls.Load(), budget.Remaining())
	}
}

func TestControlledTransportSerializesBodiesAndSpacesStarts(t *testing.T) {
	var mu sync.Mutex
	starts := []time.Time{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		_, _ = fmt.Fprint(w, "body")
	}))
	defer server.Close()
	gate := newControlledTransport(http.DefaultTransport, nil)
	gate.interval = 70 * time.Millisecond
	web := &http.Client{Transport: gate}
	first, err := web.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		response, err := web.Get(server.URL)
		if err == nil {
			_, err = io.Copy(io.Discard, response.Body)
			response.Body.Close()
		}
		finished <- err
	}()
	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	count := len(starts)
	mu.Unlock()
	if count != 1 {
		t.Fatalf("second request started before body closed: %d", count)
	}
	first.Body.Close()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(starts) != 2 || starts[1].Sub(starts[0]) < 60*time.Millisecond {
		t.Fatalf("request starts = %v", starts)
	}
}

func TestControlledTransportBackoffAndCancelledBudget(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(429)
	}))
	defer server.Close()
	gate := newControlledTransport(http.DefaultTransport, nil)
	web := &http.Client{Transport: gate}
	response, err := web.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	gate.mu.Lock()
	delay := time.Until(gate.next)
	gate.mu.Unlock()
	if delay < 119*time.Second {
		t.Fatalf("retry after = %v", delay)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	budget := NewRequestBudget(1)
	req, _ := http.NewRequestWithContext(WithRequestBudget(ctx, budget), "GET", server.URL, nil)
	if _, err := web.Do(req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting request = %v", err)
	}
	if calls.Load() != 1 || budget.Remaining() != 1 {
		t.Fatalf("cancelled request spent budget: calls=%d budget=%d", calls.Load(), budget.Remaining())
	}
}

func TestAccountIDSurvivesTokenRotationAndRejectsQueuedOldAccount(t *testing.T) {
	token := func(subject string, issued int64) string {
		text, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": subject, "iat": issued}).SignedString([]byte("test-key"))
		if err != nil {
			t.Fatal(err)
		}
		return text
	}
	c := &Client{}
	c.setAuthorization(token("alice", 1))
	first, err := c.AccountID()
	if err != nil {
		t.Fatal(err)
	}
	c.setAuthorization(token("alice", 2))
	second, err := c.AccountID()
	if err != nil || first != second {
		t.Fatalf("rotated identity: %v %v", first == second, err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	gate := newControlledTransport(http.DefaultTransport, c)
	c.setAuthorization(token("bob", 3))
	req, _ := http.NewRequestWithContext(WithRequestAccount(context.Background(), first), "GET", server.URL, nil)
	req.Header.Set("Authorization", "Bearer "+token("alice", 1))
	if _, err := (&http.Client{Transport: gate}).Do(req); err == nil {
		t.Fatal("old account request succeeded")
	}
	if calls.Load() != 0 {
		t.Fatal("old account reached server")
	}
}

func TestTruncatedResponseBodyTriggersSharedBackoff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = fmt.Fprint(w, "short")
	}))
	defer server.Close()
	gate := newControlledTransport(http.DefaultTransport, nil)
	response, err := (&http.Client{Transport: gate}).Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(response.Body)
	response.Body.Close()
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("body error=%v", err)
	}
	gate.mu.Lock()
	delay := time.Until(gate.next)
	gate.mu.Unlock()
	if delay < 29*time.Second {
		t.Fatalf("truncated body did not back off: %v", delay)
	}
}
