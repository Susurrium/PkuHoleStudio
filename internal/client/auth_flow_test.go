package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
)

func TestDetectAuthChallenge(t *testing.T) {
	tests := []struct {
		message string
		want    AuthChallengeKind
	}{
		{"需要短信验证", AuthChallengeSMS},
		{"请完成令牌验证", AuthChallengeOTP},
		{"登录态不可用", AuthChallengeNone},
	}

	for _, tt := range tests {
		if got := DetectAuthChallenge(tt.message); got != tt.want {
			t.Fatalf("DetectAuthChallenge(%q) = %q, want %q", tt.message, got, tt.want)
		}
	}
}

func TestParseAuthAPIResponse(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
	}
	if err := parseAuthAPIResponse(resp, "短信验证"); err != nil {
		t.Fatalf("parseAuthAPIResponse success: %v", err)
	}

	resp = &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"success":false,"message":"验证码错误"}`)),
	}
	if err := parseAuthAPIResponse(resp, "短信验证"); err == nil || !strings.Contains(err.Error(), "验证码错误") {
		t.Fatalf("parseAuthAPIResponse error = %v, want message", err)
	}
}

type authRoundTripper struct {
	sendCalls    int
	tokenCalls   int
	messageCalls int
}

func (rt *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body := `{"success":true}`
	switch {
	case strings.Contains(req.URL.String(), string(SEND_MESSAGE)):
		rt.sendCalls++
	case strings.Contains(req.URL.String(), string(LOGIN_BY_TOKEN)):
		rt.tokenCalls++
	case strings.Contains(req.URL.String(), string(LOGIN_BY_MESSAGE)):
		rt.messageCalls++
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

type bootstrapPasswordRoundTripper struct{}

func (rt *bootstrapPasswordRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case strings.Contains(req.URL.String(), string(UN_READ)):
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"success":false,"message":"登录态不可用"}`)),
			Request:    req,
		}, nil
	case strings.Contains(req.URL.String(), string(OAUTH_LOGIN)):
		payload, err := json.Marshal(map[string]interface{}{"success": true})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(payload))),
			Request:    req,
		}, nil
	default:
		return nil, io.EOF
	}
}

type iaaaSMSRoundTripper struct {
	sendCalls   int
	oauthCalls  int
	lastSMSCode string
}

func (rt *iaaaSMSRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body := `{"success":false,"message":"登录态不可用"}`
	switch {
	case strings.Contains(req.URL.String(), string(UN_READ)):
	case strings.Contains(req.URL.String(), string(IAAA_SEND_SMS)):
		rt.sendCalls++
		body = `{"success":true,"mobileMask":"138****0000"}`
	case strings.Contains(req.URL.String(), string(OAUTH_LOGIN)):
		rt.oauthCalls++
		if err := req.ParseForm(); err != nil {
			return nil, err
		}
		rt.lastSMSCode = req.Form.Get("smsCode")
		if rt.lastSMSCode == "" {
			body = `{"success":false,"errors":{"msg":"请使用短信验证"}}`
		} else {
			body = `{"success":true,"token":"oauth-token"}`
		}
	default:
		return nil, io.EOF
	}
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func newIAAASMSTestClient(t *testing.T, rt http.RoundTripper) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &Client{httpClient: &http.Client{Jar: jar, Transport: rt}, deviceUUID: "test-uuid"}
}

func newBootstrapTestClient(t *testing.T) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return &Client{
		httpClient: &http.Client{Jar: jar, Transport: &bootstrapPasswordRoundTripper{}},
		deviceUUID: "test-uuid",
	}
}

func TestAuthSubmitHelpers(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	rt := &authRoundTripper{}
	c := &Client{
		httpClient: &http.Client{Jar: jar, Transport: rt},
		deviceUUID: "test-uuid",
	}
	jar.SetCookies(&url.URL{Scheme: "https", Host: "treehole.pku.edu.cn"}, []*http.Cookie{{Name: "XSRF-TOKEN", Value: "xsrf-token"}})

	if err := c.SendSMSCode(); err != nil {
		t.Fatalf("SendSMSCode: %v", err)
	}
	if err := c.SubmitOTPCode("123456"); err != nil {
		t.Fatalf("SubmitOTPCode: %v", err)
	}
	if err := c.SubmitSMSCode("654321"); err != nil {
		t.Fatalf("SubmitSMSCode: %v", err)
	}
	if rt.sendCalls != 1 || rt.tokenCalls != 1 || rt.messageCalls != 1 {
		t.Fatalf("unexpected auth helper call counts: %+v", rt)
	}
}

func TestBootstrapSessionWithPasswordRequestsPasswordChallengeWhenOAuthReturnsNoToken(t *testing.T) {
	c := newBootstrapTestClient(t)

	result := c.BootstrapSessionWithPassword(&config.Config{Username: "testuser"}, "secret")

	if result.Challenge != AuthChallengePassword {
		t.Fatalf("challenge = %q, want %q", result.Challenge, AuthChallengePassword)
	}
	if result.Status.FailureKind != SessionFailureLogin {
		t.Fatalf("failure kind = %q, want %q", result.Status.FailureKind, SessionFailureLogin)
	}
	if !strings.Contains(result.Status.Message, "OAuth 登录未返回 token") {
		t.Fatalf("message = %q, want oauth token error", result.Status.Message)
	}
}

func TestBootstrapSessionSendsIAAASMSWhenOAuthRequiresIt(t *testing.T) {
	rt := &iaaaSMSRoundTripper{}
	c := newIAAASMSTestClient(t, rt)
	result := c.BootstrapSessionWithPassword(&config.Config{Username: "1234567890"}, "secret")
	if result.Challenge != AuthChallengeSMS || result.ChallengeStage != AuthChallengeStageIAAA {
		t.Fatalf("challenge/stage = %q/%q", result.Challenge, result.ChallengeStage)
	}
	if rt.sendCalls != 1 || !strings.Contains(result.Status.Message, "138****0000") {
		t.Fatalf("send calls/message = %d/%q", rt.sendCalls, result.Status.Message)
	}
}

func TestOAuthLoginWithVerificationSubmitsIAAASMSCode(t *testing.T) {
	rt := &iaaaSMSRoundTripper{}
	c := newIAAASMSTestClient(t, rt)
	result, err := c.OAuthLoginWithVerification("1234567890", "secret", "654321", "")
	if err != nil || result["token"] != "oauth-token" || rt.lastSMSCode != "654321" {
		t.Fatalf("result/error/code = %+v/%v/%q", result, err, rt.lastSMSCode)
	}
}

func TestOAuthFailureMessageUsesNestedIAAAReason(t *testing.T) {
	message := oauthFailureMessage(map[string]interface{}{
		"success": false,
		"errors":  map[string]interface{}{"msg": "用户名或密码错误"},
	})
	if message != "用户名或密码错误" {
		t.Fatalf("message = %q", message)
	}
}

func TestBootstrapSessionWithPasswordRequestsUsernameChallengeWhenUsernameMissing(t *testing.T) {
	c := newBootstrapTestClient(t)

	result := c.BootstrapSessionWithPassword(&config.Config{}, "secret")

	if result.Challenge != AuthChallengeUsername {
		t.Fatalf("challenge = %q, want %q", result.Challenge, AuthChallengeUsername)
	}
	if !strings.Contains(result.Status.Message, "未配置用户名") {
		t.Fatalf("message = %q, want missing username", result.Status.Message)
	}
}

func TestBootstrapSessionWithPasswordRequestsPasswordChallengeWhenPasswordMissing(t *testing.T) {
	c := newBootstrapTestClient(t)

	result := c.BootstrapSessionWithPassword(&config.Config{Username: "testuser"}, "")

	if result.Challenge != AuthChallengePassword {
		t.Fatalf("challenge = %q, want %q", result.Challenge, AuthChallengePassword)
	}
	if !strings.Contains(result.Status.Message, "未配置密码") {
		t.Fatalf("message = %q, want missing password", result.Status.Message)
	}
}

func TestBootstrapSessionRequestsUsernameChallengeWhenOnlyPasswordConfigured(t *testing.T) {
	c := newBootstrapTestClient(t)

	result := c.BootstrapSession(&config.Config{Password: "secret"})

	if result.Challenge != AuthChallengeUsername {
		t.Fatalf("challenge = %q, want %q", result.Challenge, AuthChallengeUsername)
	}
}

func TestBootstrapSessionRequestsPasswordChallengeWhenOnlyUsernameConfigured(t *testing.T) {
	c := newBootstrapTestClient(t)

	result := c.BootstrapSession(&config.Config{Username: "testuser"})

	if result.Challenge != AuthChallengePassword {
		t.Fatalf("challenge = %q, want %q", result.Challenge, AuthChallengePassword)
	}
}

func TestBootstrapSessionSkipsProbeWhenCredentialsArePartial(t *testing.T) {
	c := newBootstrapTestClient(t)

	result := c.BootstrapSession(&config.Config{Password: "secret"})

	if result.Challenge != AuthChallengeUsername {
		t.Fatalf("challenge = %q, want %q", result.Challenge, AuthChallengeUsername)
	}
	if result.Status.Message != "未配置用户名，请输入账号后重试" {
		t.Fatalf("message = %q", result.Status.Message)
	}
}
