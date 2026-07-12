package service

import (
	"context"
	"errors"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/config"
)

type AuthStatus struct {
	Checked         bool   `json:"checked"`
	HasSession      bool   `json:"has_session"`
	CanReadOnline   bool   `json:"can_read_online"`
	CanWriteOnline  bool   `json:"can_write_online"`
	FailureKind     string `json:"failure_kind,omitempty"`
	Message         string `json:"message,omitempty"`
	Challenge       string `json:"challenge,omitempty"`
	ChallengeReason string `json:"challenge_reason,omitempty"`
}

type AuthService interface {
	CachedStatus(ctx context.Context) AuthStatus
	Probe(ctx context.Context) AuthStatus
	Login(ctx context.Context, username, password string) AuthStatus
	Continue(ctx context.Context, challenge, code string) AuthStatus
}

type TreeholeAuthService struct {
	client *client.Client
	config *config.Config
}

func NewAuthService(treeholeClient *client.Client, cfg *config.Config) *TreeholeAuthService {
	return &TreeholeAuthService{client: treeholeClient, config: cfg}
}

func (s *TreeholeAuthService) CachedStatus(ctx context.Context) AuthStatus {
	if err := authContextError(ctx); err != nil {
		return AuthStatus{FailureKind: "network", Message: err.Error()}
	}
	if s == nil || s.client == nil {
		return AuthStatus{FailureKind: "login", Message: "树洞客户端未初始化"}
	}
	hasSession := s.client.GetAuthorization() != ""
	message := "尚未检测在线登录状态"
	if !hasSession {
		message = "未检测到本机登录凭据"
	}
	return AuthStatus{
		HasSession: hasSession, CanWriteOnline: hasSession && s.client.GetXSRFToken() != "",
		FailureKind: "none", Message: message,
	}
}

func (s *TreeholeAuthService) Probe(ctx context.Context) AuthStatus {
	if err := authContextError(ctx); err != nil {
		return AuthStatus{Checked: true, FailureKind: "network", Message: err.Error()}
	}
	if s == nil || s.client == nil {
		return AuthStatus{Checked: true, FailureKind: "login", Message: "树洞客户端未初始化"}
	}
	return authStatusFromSession(s.client.ProbeSession())
}

func (s *TreeholeAuthService) Login(ctx context.Context, username, password string) AuthStatus {
	if err := authContextError(ctx); err != nil {
		return AuthStatus{Checked: true, FailureKind: "network", Message: err.Error()}
	}
	if s == nil || s.client == nil {
		return AuthStatus{Checked: true, FailureKind: "login", Message: "树洞客户端未初始化"}
	}
	cfg := config.Config{}
	if s.config != nil {
		cfg = *s.config
	}
	cfg.Username = strings.TrimSpace(username)
	result := s.client.BootstrapSessionWithPassword(&cfg, password)
	return authStatusFromBootstrap(result)
}

func (s *TreeholeAuthService) Continue(ctx context.Context, challenge, code string) AuthStatus {
	if err := authContextError(ctx); err != nil {
		return AuthStatus{Checked: true, FailureKind: "network", Message: err.Error()}
	}
	if s == nil || s.client == nil {
		return AuthStatus{Checked: true, FailureKind: "login", Message: "树洞客户端未初始化"}
	}
	kind := client.AuthChallengeKind(strings.TrimSpace(challenge))
	if kind != client.AuthChallengeSMS && kind != client.AuthChallengeOTP {
		return AuthStatus{Checked: true, FailureKind: "login", Message: "不支持的验证类型"}
	}
	return authStatusFromBootstrap(s.client.ContinueAuthChallenge(kind, strings.TrimSpace(code)))
}

func authStatusFromBootstrap(result client.AuthBootstrapResult) AuthStatus {
	status := authStatusFromSession(result.Status)
	status.Challenge = string(result.Challenge)
	status.ChallengeReason = result.ChallengeReason
	return status
}

func authStatusFromSession(status client.SessionStatus) AuthStatus {
	return AuthStatus{
		Checked: true, HasSession: status.HasSession, CanReadOnline: status.CanReadOnline,
		CanWriteOnline: status.CanWriteOnline, FailureKind: string(status.FailureKind), Message: status.Message,
	}
}

func authContextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return errors.New("请求已取消")
	}
	return nil
}
