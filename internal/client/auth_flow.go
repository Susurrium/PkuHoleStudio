package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/config"

	"github.com/pquerna/otp/totp"
)

type AuthChallengeKind string

const (
	AuthChallengeNone     AuthChallengeKind = ""
	AuthChallengeUsername AuthChallengeKind = "username"
	AuthChallengePassword AuthChallengeKind = "password"
	AuthChallengeSMS      AuthChallengeKind = "sms"
	AuthChallengeOTP      AuthChallengeKind = "otp"
)

type AuthBootstrapResult struct {
	Status          SessionStatus
	Challenge       AuthChallengeKind
	ChallengeReason string
	LoginAttempted  bool
}

func DetectAuthChallenge(message string) AuthChallengeKind {
	switch {
	case strings.Contains(message, "短信验证"):
		return AuthChallengeSMS
	case strings.Contains(message, "令牌验证"):
		return AuthChallengeOTP
	default:
		return AuthChallengeNone
	}
}

func (c *Client) BootstrapSession(cfg *config.Config) AuthBootstrapResult {
	if cfg != nil && cfg.HasAnyPasswordLoginInput() && !cfg.HasPasswordLogin() {
		return c.BootstrapSessionWithPassword(cfg, cfg.Password)
	}
	result := c.finalizeAuthStatus()
	if result.Status.CanReadOnline {
		return result
	}
	if cfg == nil || !cfg.HasAnyPasswordLoginInput() {
		return result
	}
	return c.BootstrapSessionWithPassword(cfg, cfg.Password)
}

func (c *Client) BootstrapSessionWithPassword(cfg *config.Config, password string) AuthBootstrapResult {
	if cfg == nil || strings.TrimSpace(cfg.Username) == "" {
		return AuthBootstrapResult{
			Status: SessionStatus{
				HasSession:  c.GetAuthorization() != "",
				FailureKind: SessionFailureLogin,
				Message:     "未配置用户名，请输入账号后重试",
			},
			Challenge:       AuthChallengeUsername,
			ChallengeReason: "未配置用户名，请输入账号后重试",
		}
	}
	if strings.TrimSpace(password) == "" {
		return AuthBootstrapResult{
			Status: SessionStatus{
				HasSession:  c.GetAuthorization() != "",
				FailureKind: SessionFailureLogin,
				Message:     "未配置密码，请输入密码后重试",
			},
			Challenge:       AuthChallengePassword,
			ChallengeReason: "未配置密码，请输入密码后重试",
		}
	}
	return c.bootstrapSessionWithPassword(cfg, password)
}

func (c *Client) bootstrapSessionWithPassword(cfg *config.Config, password string) AuthBootstrapResult {
	result := c.finalizeAuthStatus()
	result.LoginAttempted = true
	oauthResult, err := c.OAuthLogin(cfg.Username, password)
	if err != nil {
		result.Status.FailureKind = ClassifySessionError(err)
		result.Status.Message = err.Error()
		result.Challenge = AuthChallengeNone
		result.ChallengeReason = ""
		return result
	}

	token, ok := oauthResult["token"].(string)
	if !ok || token == "" {
		result.Status.FailureKind = SessionFailureLogin
		result.Status.Message = oauthFailureMessage(oauthResult)
		result.Challenge = AuthChallengePassword
		result.ChallengeReason = result.Status.Message
		return result
	}

	if err := c.SSOLogin(token); err != nil {
		result.Status.FailureKind = ClassifySessionError(err)
		result.Status.Message = err.Error()
		result.Challenge = AuthChallengeNone
		result.ChallengeReason = ""
		return result
	}

	result = c.finalizeAuthStatus()
	result.LoginAttempted = true
	if result.Status.CanReadOnline || result.Challenge != AuthChallengeOTP || cfg == nil || !cfg.HasTOTPSecret() {
		return result
	}

	code, err := totp.GenerateCode(cfg.SecretKey, time.Now())
	if err != nil {
		result.Status.FailureKind = SessionFailureLogin
		result.Status.Message = err.Error()
		result.Challenge = AuthChallengeOTP
		result.ChallengeReason = result.Status.Message
		return result
	}

	submit := c.ContinueAuthChallenge(AuthChallengeOTP, code)
	submit.LoginAttempted = true
	return submit
}

func oauthFailureMessage(result map[string]interface{}) string {
	for _, key := range []string{"message", "msg", "error_description"} {
		if message := authMessageValue(result[key]); message != "" {
			return message
		}
	}
	for _, key := range []string{"errors", "error"} {
		if message := authMessageValue(result[key]); message != "" {
			return message
		}
	}
	return "OAuth 登录未返回 token，请检查学号格式和密码后重试"
}

func authMessageValue(value interface{}) string {
	switch current := value.(type) {
	case string:
		return strings.TrimSpace(current)
	case map[string]interface{}:
		for _, key := range []string{"message", "msg", "error_description", "error"} {
			if message := authMessageValue(current[key]); message != "" {
				return message
			}
		}
	case []interface{}:
		for _, item := range current {
			if message := authMessageValue(item); message != "" {
				return message
			}
		}
	}
	return ""
}

func (c *Client) ContinueAuthChallenge(kind AuthChallengeKind, code string) AuthBootstrapResult {
	var err error
	switch kind {
	case AuthChallengeSMS:
		err = c.SubmitSMSCode(code)
	case AuthChallengeOTP:
		err = c.SubmitOTPCode(code)
	default:
		err = fmt.Errorf("unsupported auth challenge: %s", kind)
	}
	if err != nil {
		return AuthBootstrapResult{
			Status: SessionStatus{
				HasSession:  c.GetAuthorization() != "",
				FailureKind: ClassifySessionError(err),
				Message:     err.Error(),
			},
			Challenge:       kind,
			ChallengeReason: err.Error(),
		}
	}
	return c.finalizeAuthStatus()
}

func (c *Client) SendSMSCode() error {
	resp, err := c.SendMessage()
	if err != nil {
		return err
	}
	return parseAuthAPIResponse(resp, "发送短信验证码")
}

func (c *Client) SubmitSMSCode(code string) error {
	resp, err := c.LoginByMessage(code)
	if err != nil {
		return err
	}
	return parseAuthAPIResponse(resp, "短信验证")
}

func (c *Client) SubmitOTPCode(code string) error {
	resp, err := c.LoginByToken(code)
	if err != nil {
		return err
	}
	return parseAuthAPIResponse(resp, "令牌验证")
}

func (c *Client) finalizeAuthStatus() AuthBootstrapResult {
	status := c.ProbeSession()
	challenge := DetectAuthChallenge(status.Message)
	if status.CanReadOnline {
		_ = c.SaveCookies()
	}
	return AuthBootstrapResult{
		Status:          status,
		Challenge:       challenge,
		ChallengeReason: status.Message,
	}
}

func parseAuthAPIResponse(resp *http.Response, action string) error {
	if resp == nil {
		return fmt.Errorf("%s失败: 空响应", action)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s失败: HTTP %d", action, resp.StatusCode)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("%s失败: %w", action, err)
	}

	if success, ok := payload["success"].(bool); ok && success {
		return nil
	}
	if code, ok := payload["code"].(float64); ok && int(code) == 20000 {
		return nil
	}
	if message, ok := payload["message"].(string); ok && message != "" {
		return fmt.Errorf("%s失败: %s", action, message)
	}
	return fmt.Errorf("%s失败", action)
}
