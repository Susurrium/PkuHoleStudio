package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSessionPromptDialogPreservesBackgroundAfterInlineResets(t *testing.T) {
	dialog := NewSessionPromptDialog(SessionState{
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	})

	output := dialog.View(60)
	styles := dialog.username.Styles()
	if got := styles.Focused.Placeholder.GetBackground(); got != colorSurface {
		t.Fatalf("session username placeholder background = %v, want %v", got, colorSurface)
	}
	styles = dialog.password.Styles()
	if got := styles.Blurred.Placeholder.GetBackground(); got != colorSurface {
		t.Fatalf("session password placeholder background = %v, want %v", got, colorSurface)
	}
	if strings.Contains(output, "\x1b[m  ") {
		t.Fatalf("session prompt should preserve dialog background after reset:\n%q", output)
	}
	if !strings.Contains(output, paintedDialogSpaces(4)) {
		t.Fatalf("session prompt blanks should carry dialog background:\n%q", output)
	}
	if !containsReverseVideo(output) {
		t.Fatalf("session prompt should render the bubbles virtual reverse cursor:\n%q", output)
	}
	plain := stripANSI(output)
	for _, want := range []string{"▄", "▀", "输入用户名", "输入密码"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("session prompt credential input missing search-style element %q:\n%s", want, plain)
		}
	}
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "用户名") && strings.Contains(line, "▄") {
			t.Fatalf("username label should not share a line with the input border:\n%s", plain)
		}
		if strings.Contains(line, "密码") && strings.Contains(line, "▄") {
			t.Fatalf("password label should not share a line with the input border:\n%s", plain)
		}
	}
	assertInputBlockAligned(t, plain, "输入用户名")
	assertInputBlockAligned(t, plain, "输入密码")
}

func TestAuthChallengeDialogPreservesBackgroundAndInputStyles(t *testing.T) {
	dialog := NewAuthChallengeDialog(SessionState{
		Challenge:        AuthChallengeTypePassword,
		ChallengeMessage: "请输入密码",
	})

	styles := dialog.input.Styles()
	if got := styles.Focused.Text.GetBackground(); got != colorBg {
		t.Fatalf("auth input text background = %v, want %v", got, colorBg)
	}
	if got := styles.Focused.Placeholder.GetBackground(); got != colorBg {
		t.Fatalf("auth input placeholder background = %v, want %v", got, colorBg)
	}

	output := dialog.View(60)
	if strings.Contains(output, "\x1b[m  ") {
		t.Fatalf("auth challenge should preserve dialog background after reset:\n%q", output)
	}
	if !containsReverseVideo(output) {
		t.Fatalf("auth challenge should render the bubbles virtual reverse cursor:\n%q", output)
	}
	if !strings.Contains(output, paintedDialogSpaces(4)) {
		t.Fatalf("auth challenge blanks should carry dialog background:\n%q", output)
	}
}

func TestSessionPromptRendersAsPanelLayerOverDashboard(t *testing.T) {
	m := newTestModel()
	m.Page = PageDashboard
	m.Width = 100
	m.Height = 36
	m.Dialog = DialogSessionPrompt
	m.Session = SessionState{
		Mode:          SessionModeOffline,
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "未检测到可用登录态，也未配置账号密码。请先输入账号密码。",
	}
	m.SessionDialog = NewSessionPromptDialog(m.Session)

	output := stripANSI(viewString(m))
	for _, want := range []string{"登录不可用", "用户名", "输入用户名", "密码", "输入密码", "进入离线模式"} {
		if !strings.Contains(output, want) {
			t.Fatalf("panel-layer session prompt missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "╭") || strings.Contains(output, "╰") {
		t.Fatalf("session prompt should use the tools-style panel layer, not rounded dialog card:\n%s", output)
	}
}

func TestSessionPromptWriteLoginFrame(t *testing.T) {
	m := newTestModel()
	m.Page = PageDashboard
	m.Width = 126
	m.Height = 36
	m.Dialog = DialogSessionPrompt
	m.Session = SessionState{
		Mode:          SessionModeOffline,
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "未检测到可用登录态，也未配置账号密码。请先打开配置填写账号密码。",
	}
	m.SessionDialog = NewSessionPromptDialog(m.Session)

	output := viewString(m)
	stripped := stripANSI(output)

	_, filename, _, _ := runtime.Caller(0)
	outDir := filepath.Join(filepath.Dir(filename), "../..", ".out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", outDir, err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "current-frame-login.ansi"), []byte(output), 0644); err != nil {
		t.Fatalf("write login ansi frame: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "current-frame-login.txt"), []byte(stripped), 0644); err != nil {
		t.Fatalf("write login text frame: %v", err)
	}

	for _, line := range strings.Split(stripped, "\n") {
		if strings.Contains(line, "用户名") && strings.Contains(line, "▄") {
			t.Fatalf("login frame username label collides with border:\n%s", stripped)
		}
		if strings.Contains(line, "密码") && strings.Contains(line, "▄") {
			t.Fatalf("login frame password label collides with border:\n%s", stripped)
		}
	}
	if !containsReverseVideo(output) {
		t.Fatalf("login frame should render the bubbles virtual reverse cursor:\n%q", output)
	}
	assertInputBlockAligned(t, stripped, "输入用户名")
	assertInputBlockAligned(t, stripped, "输入密码")
}

func TestSessionPromptVirtualCursorFollowsInput(t *testing.T) {
	dialog := NewSessionPromptDialog(SessionState{
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	})
	dialog.Update(keyPress('a'))

	output := stripANSI(dialog.View(60))
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "a") {
			if strings.Contains(line, "a输入用户名") {
				t.Fatalf("placeholder should not remain after typing:\n%s", output)
			}
			if strings.Index(line, "a") > strings.Index(line, "输入用户名") && strings.Contains(line, "输入用户名") {
				t.Fatalf("cursor/input ordering is wrong:\n%s", output)
			}
			return
		}
	}
	t.Fatalf("typed input missing from session prompt:\n%s", output)
}

func containsReverseVideo(output string) bool {
	return strings.Contains(output, "\x1b[7;") ||
		strings.Contains(output, ";7m") ||
		strings.Contains(output, ";7;")
}

func assertInputBlockAligned(t *testing.T, frame, placeholder string) {
	t.Helper()
	lines := strings.Split(frame, "\n")
	for i, line := range lines {
		if !strings.Contains(line, placeholder) {
			continue
		}
		if i == 0 || i+1 >= len(lines) {
			t.Fatalf("input placeholder %q has no surrounding border lines:\n%s", placeholder, frame)
		}
		top := strings.Index(lines[i-1], "▄")
		mid := strings.Index(line, placeholder) - 1
		bottom := strings.Index(lines[i+1], "▀")
		if top < 0 || bottom < 0 || mid < 0 {
			t.Fatalf("input block around %q missing border/content:\n%s", placeholder, frame)
		}
		if top != bottom {
			t.Fatalf("input block around %q has misaligned borders: top=%d bottom=%d\n%s", placeholder, top, bottom, frame)
		}
		return
	}
	t.Fatalf("placeholder %q not found:\n%s", placeholder, frame)
}

func TestSessionPromptAppendsSMSChallengeInputAndButton(t *testing.T) {
	dialog := NewSessionPromptDialog(SessionState{
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	})
	dialog.SetChallenge(SessionState{
		Challenge:        AuthChallengeTypeSMS,
		ChallengeMessage: "需要短信验证码",
	})

	output := stripANSI(dialog.View(80))
	for _, want := range []string{"用户名", "密码", "验证码", "输入短信验证码", "发送验证码"} {
		if !strings.Contains(output, want) {
			t.Fatalf("sms challenge form missing %q:\n%s", want, output)
		}
	}
}

func TestSessionPromptAppendsOTPChallengeInputAndButton(t *testing.T) {
	dialog := NewSessionPromptDialog(SessionState{
		FailureReason: SessionFailureReasonLogin,
		NeedsConfig:   true,
		Message:       "请先填写账号密码",
	})
	dialog.SetChallenge(SessionState{
		Challenge:        AuthChallengeTypeOTP,
		ChallengeMessage: "需要手机令牌",
	})

	output := stripANSI(dialog.View(80))
	for _, want := range []string{"用户名", "密码", "手机令牌", "输入手机令牌", "提交令牌"} {
		if !strings.Contains(output, want) {
			t.Fatalf("otp challenge form missing %q:\n%s", want, output)
		}
	}
}
