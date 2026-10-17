package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrRequestBudget = errors.New("本轮请求预算已用完，将在下一轮继续")
var ErrAccountChanged = errors.New("登录账号已切换或退出，更新已停止")

type managedCookieJar struct {
	mu  sync.RWMutex
	jar http.CookieJar
}

func (j *managedCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	j.jar.SetCookies(u, cookies)
}
func (j *managedCookieJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.jar.Cookies(u)
}
func (j *managedCookieJar) reset(jar http.CookieJar) { j.mu.Lock(); defer j.mu.Unlock(); j.jar = jar }

type RequestBudget struct {
	mu        sync.Mutex
	remaining int
}

func NewRequestBudget(limit int) *RequestBudget { return &RequestBudget{remaining: limit} }
func (b *RequestBudget) Remaining() int         { b.mu.Lock(); defer b.mu.Unlock(); return b.remaining }
func (b *RequestBudget) take() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining <= 0 {
		return ErrRequestBudget
	}
	b.remaining--
	return nil
}

type requestBudgetKey struct{}
type requestAccountKey struct{}

func WithRequestBudget(ctx context.Context, budget *RequestBudget) context.Context {
	return context.WithValue(ctx, requestBudgetKey{}, budget)
}
func WithRequestAccount(ctx context.Context, account string) context.Context {
	return context.WithValue(ctx, requestAccountKey{}, account)
}

func RequestAccount(ctx context.Context) string {
	account, _ := ctx.Value(requestAccountKey{}).(string)
	return account
}

// JWT 仅用于区分当前会话所属账号；接口权限仍由树洞验证。
func (c *Client) AccountID() (string, error) {
	return accountIDFromToken(c.GetAuthorization())
}

func accountIDFromToken(token string) (string, error) {
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser(jwt.WithJSONNumber()).ParseUnverified(token, claims); err != nil {
		return "", errors.New("无法识别当前登录账号，请重新登录")
	}
	for _, key := range []string{"sub", "uid", "user_id", "student_id"} {
		if value, ok := claims[key]; ok && fmt.Sprint(value) != "" && value != nil {
			digest := sha256.Sum256([]byte(fmt.Sprint(claims["iss"]) + ":" + key + ":" + fmt.Sprint(value)))
			return hex.EncodeToString(digest[:]), nil
		}
	}
	return "", errors.New("登录会话缺少账号标识，请重新登录")
}

type controlledTransport struct {
	base     http.RoundTripper
	owner    *Client
	slot     chan struct{}
	mu       sync.Mutex
	next     time.Time
	failures int
	interval time.Duration
}

func newControlledTransport(base http.RoundTripper, owner *Client) *controlledTransport {
	return &controlledTransport{base: base, owner: owner, slot: make(chan struct{}, 1), interval: time.Second}
}

func (t *controlledTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case t.slot <- struct{}{}:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	release := func() { <-t.slot }
	t.mu.Lock()
	delay := time.Until(t.next)
	t.mu.Unlock()
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-req.Context().Done():
			release()
			return nil, req.Context().Err()
		}
	}
	if expected, ok := req.Context().Value(requestAccountKey{}).(string); ok {
		current, err := t.owner.AccountID()
		requestAccount, requestErr := accountIDFromToken(strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer "))
		if err != nil || current != expected || requestErr != nil || requestAccount != expected {
			release()
			return nil, ErrAccountChanged
		}
	}
	if budget, ok := req.Context().Value(requestBudgetKey{}).(*RequestBudget); ok {
		if err := budget.take(); err != nil {
			release()
			return nil, err
		}
	}
	t.mu.Lock()
	t.next = time.Now().Add(t.interval)
	t.mu.Unlock()
	response, err := t.base.RoundTrip(req)
	t.mu.Lock()
	if err != nil || response.StatusCode == 429 || response.StatusCode >= 500 {
		t.failures++
		backoff := min(30*time.Second*time.Duration(1<<min(t.failures-1, 6)), 30*time.Minute)
		if response != nil {
			if seconds, parseErr := strconv.Atoi(response.Header.Get("Retry-After")); parseErr == nil {
				backoff = max(backoff, time.Duration(seconds)*time.Second)
			}
			if until, parseErr := http.ParseTime(response.Header.Get("Retry-After")); parseErr == nil {
				backoff = max(backoff, time.Until(until))
			}
		}
		t.next = time.Now().Add(backoff)
	} else {
		t.failures = 0
	}
	t.mu.Unlock()
	if err != nil {
		release()
		return nil, err
	}
	response.Body = &controlledBody{ReadCloser: response.Body, release: release, onFailure: func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.failures++
		t.next = time.Now().Add(min(30*time.Second*time.Duration(1<<min(t.failures-1, 6)), 30*time.Minute))
	}}
	return response, nil
}

type controlledBody struct {
	io.ReadCloser
	once        sync.Once
	failureOnce sync.Once
	release     func()
	onFailure   func()
}

func (b *controlledBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			b.failureOnce.Do(b.onFailure)
		}
		b.once.Do(b.release)
	}
	return n, err
}
func (b *controlledBody) Close() error { err := b.ReadCloser.Close(); b.once.Do(b.release); return err }
