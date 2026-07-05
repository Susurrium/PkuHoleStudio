package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type SessionPromptDialogModel struct {
	Title            string
	Message          string
	Options          []string
	Selected         int
	needsCredentials bool
	username         textinput.Model
	password         textinput.Model
	challengeInput   textinput.Model
	challenge        AuthChallengeType
	statusText       string
	focusIndex       int
	errorText        string
	smsSentOnce      bool
}

func NewSessionPromptDialog(state SessionState) SessionPromptDialogModel {
	username := textinput.New()
	username.Prompt = ""
	username.Placeholder = "输入用户名"
	username.SetWidth(28)
	styleTextInput(&username, colorSurface, colorText, colorMuted)

	password := textinput.New()
	password.Prompt = ""
	password.Placeholder = "输入密码"
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '*'
	password.SetWidth(28)
	styleTextInput(&password, colorSurface, colorText, colorMuted)

	challengeInput := textinput.New()
	challengeInput.Prompt = ""
	challengeInput.Placeholder = "输入验证码"
	challengeInput.SetWidth(28)
	styleTextInput(&challengeInput, colorSurface, colorText, colorMuted)

	m := SessionPromptDialogModel{
		username:       username,
		password:       password,
		challengeInput: challengeInput,
	}
	m.ApplyState(state)
	return m
}

func (m SessionPromptDialogModel) initialized() bool {
	return m.Options != nil
}

func (m *SessionPromptDialogModel) ApplyState(state SessionState) {
	m.needsCredentials = false
	m.errorText = ""
	m.statusText = ""
	if state.Challenge != AuthChallengeTypeSMS {
		m.smsSentOnce = false
	}
	switch state.FailureReason {
	case SessionFailureReasonLogin:
		m.Title = "登录不可用"
		m.Message = state.Message
		if m.Message == "" {
			m.Message = "当前登录态不可用。"
		}
		if state.NeedsConfig {
			m.needsCredentials = true
			m.challenge = state.Challenge
			m.configureChallengeInput()
			m.Options = []string{"进入离线模式"}
			m.focusIndex = clampInt(m.focusIndex, 0, m.maxFocusIndex())
			m.updateCredentialFocus()
		} else {
			m.Options = []string{"重新登录", "进入离线模式"}
		}
	case SessionFailureReasonNetwork:
		m.Title = "网络错误"
		m.Message = state.Message
		if m.Message == "" {
			m.Message = "当前无法连接树洞服务。"
		}
		m.Options = []string{"进入离线模式"}
	default:
		m.Title = "在线模式"
		m.Message = "在线能力可用"
		m.Options = []string{"确定"}
	}
	if m.Selected >= len(m.Options) {
		m.Selected = 0
	}
}

func (m *SessionPromptDialogModel) Update(msg tea.KeyPressMsg) tea.Cmd {
	if m.needsCredentials {
		switch {
		case key.Matches(msg, shortcut.Tab, shortcut.Up, shortcut.Down):
			if key.Matches(msg, shortcut.Up) {
				m.focusIndex--
			} else {
				m.focusIndex++
			}
			maxFocus := m.maxFocusIndex() + 1
			m.focusIndex = (m.focusIndex + maxFocus) % maxFocus
			m.updateCredentialFocus()
			return nil
		}
		if key.Matches(msg, shortcut.ShiftTab) {
			maxFocus := m.maxFocusIndex() + 1
			m.focusIndex = (m.focusIndex + maxFocus - 1) % maxFocus
			m.updateCredentialFocus()
			return nil
		}
		if m.focusIndex == 0 {
			var cmd tea.Cmd
			m.username, cmd = m.username.Update(msg)
			return cmd
		}
		if m.focusIndex == 1 {
			var cmd tea.Cmd
			m.password, cmd = m.password.Update(msg)
			return cmd
		}
		if m.focusIndex == 2 && m.challenge != AuthChallengeTypeNone {
			var cmd tea.Cmd
			m.challengeInput, cmd = m.challengeInput.Update(msg)
			return cmd
		}
		return nil
	}
	switch {
	case key.Matches(msg, shortcut.Up, shortcut.VimUp):
		if m.Selected > 0 {
			m.Selected--
		}
	case key.Matches(msg, shortcut.Down, shortcut.VimDown):
		if m.Selected < len(m.Options)-1 {
			m.Selected++
		}
	}
	return nil
}

func (m SessionPromptDialogModel) SelectedOption() string {
	if m.Selected < 0 || m.Selected >= len(m.Options) {
		return ""
	}
	return m.Options[m.Selected]
}

func (m SessionPromptDialogModel) NeedsCredentials() bool {
	return m.needsCredentials
}

func (m SessionPromptDialogModel) Credentials() (string, string) {
	return strings.TrimSpace(m.username.Value()), strings.TrimSpace(m.password.Value())
}

func (m SessionPromptDialogModel) ChallengeValue() string {
	return strings.TrimSpace(m.challengeInput.Value())
}

func (m SessionPromptDialogModel) Challenge() AuthChallengeType {
	return m.challenge
}

func (m SessionPromptDialogModel) CredentialFocusIndex() int {
	return m.focusIndex
}

func (m *SessionPromptDialogModel) SetError(err error) {
	if err == nil {
		m.errorText = ""
		return
	}
	m.errorText = err.Error()
}

func (m *SessionPromptDialogModel) SetStatus(message string) {
	m.statusText = message
}

func (m *SessionPromptDialogModel) SetChallenge(state SessionState) {
	m.needsCredentials = true
	m.challenge = state.Challenge
	if state.ChallengeMessage != "" {
		m.Message = state.ChallengeMessage
	} else if state.Message != "" {
		m.Message = state.Message
	}
	m.errorText = ""
	m.statusText = ""
	m.configureChallengeInput()
	m.focusIndex = 2
	m.updateCredentialFocus()
}

func (m *SessionPromptDialogModel) MarkSMSSent() {
	m.smsSentOnce = true
}

func (m SessionPromptDialogModel) ChallengeButtonLabel() string {
	switch m.challenge {
	case AuthChallengeTypeSMS:
		if m.smsSentOnce {
			return "重发验证码"
		}
		return "发送验证码"
	case AuthChallengeTypeOTP:
		return "提交令牌"
	default:
		return ""
	}
}

func (m SessionPromptDialogModel) maxFocusIndex() int {
	if m.challenge != AuthChallengeTypeNone {
		return 4
	}
	return 2
}

func (m *SessionPromptDialogModel) configureChallengeInput() {
	switch m.challenge {
	case AuthChallengeTypeSMS:
		m.challengeInput.Placeholder = "输入短信验证码"
		m.challengeInput.EchoMode = textinput.EchoNormal
	case AuthChallengeTypeOTP:
		m.challengeInput.Placeholder = "输入手机令牌"
		m.challengeInput.EchoMode = textinput.EchoNormal
	default:
		m.challengeInput.Placeholder = "输入验证码"
		m.challengeInput.EchoMode = textinput.EchoNormal
	}
}

func (m *SessionPromptDialogModel) updateCredentialFocus() {
	m.username.Blur()
	m.password.Blur()
	m.challengeInput.Blur()
	if m.focusIndex == 0 {
		_ = m.username.Focus()
	}
	if m.focusIndex == 1 {
		_ = m.password.Focus()
	}
	if m.focusIndex == 2 && m.challenge != AuthChallengeTypeNone {
		_ = m.challengeInput.Focus()
	}
}

func (m SessionPromptDialogModel) View(width int) string {
	var b strings.Builder
	contentWidth := maxInt(20, width-12)
	fill := dialogBackgroundFillStyle()

	username := m.username
	password := m.password
	challengeInput := m.challengeInput
	inputOuterWidth := minInt(46, maxInt(20, contentWidth-4))
	inputInnerWidth := maxInt(1, inputOuterWidth-vSearchInput.GetHorizontalFrameSize()-1)
	username.SetWidth(inputInnerWidth)
	password.SetWidth(inputInnerWidth)
	challengeInput.SetWidth(inputInnerWidth)
	styleTextInput(&username, colorSurface, colorText, colorMuted)
	styleTextInput(&password, colorSurface, colorText, colorMuted)
	styleTextInput(&challengeInput, colorSurface, colorText, colorMuted)

	b.WriteString(vDialogTitleStyle.Render(m.Title))
	b.WriteString("\n\n")
	b.WriteString(fillRenderedBackground(fill.Render(m.Message), contentWidth, fill))
	b.WriteString("\n\n")
	if m.needsCredentials {
		b.WriteString(m.renderCredentialField("用户名", m.renderFramedInput(username.View(), inputOuterWidth, inputInnerWidth, m.focusIndex == 0), m.focusIndex == 0, contentWidth, fill))
		b.WriteString("\n\n")
		b.WriteString(m.renderCredentialField("密码", m.renderFramedInput(password.View(), inputOuterWidth, inputInnerWidth, m.focusIndex == 1), m.focusIndex == 1, contentWidth, fill))
		b.WriteString("\n\n")
		if m.challenge != AuthChallengeTypeNone {
			label := "验证码"
			if m.challenge == AuthChallengeTypeOTP {
				label = "手机令牌"
			}
			b.WriteString(m.renderCredentialField(label, m.renderFramedInput(challengeInput.View(), inputOuterWidth, inputInnerWidth, m.focusIndex == 2), m.focusIndex == 2, contentWidth, fill))
			b.WriteString("\n")
			button := vButtonDefault.Render(m.ChallengeButtonLabel())
			if m.focusIndex == 3 {
				button = vButtonActive.Render(m.ChallengeButtonLabel())
			}
			buttonLine := fill.Render("  ") + button
			b.WriteString(fillRenderedBackground(buttonLine, contentWidth, fill))
			b.WriteString("\n\n")
		}
		offlinePrefix := "  "
		if m.focusIndex == m.maxFocusIndex() {
			offlinePrefix = "→ "
		}
		offline := fill.Render(offlinePrefix) + fill.Render("进入离线模式")
		b.WriteString(fillRenderedBackground(offline, contentWidth, fill))
		if m.statusText != "" {
			b.WriteString("\n\n")
			b.WriteString(fillRenderedBackground(vHelpStyle.Render(m.statusText), contentWidth, fill))
		}
		if m.errorText != "" {
			b.WriteString("\n\n")
			b.WriteString(fillRenderedBackground(vErrorStyle.Render(m.errorText), contentWidth, fill))
		}
		b.WriteString("\n\n")
		//b.WriteString(vDialogHelpStyle.Render("Tab/↑↓: 切换 | Enter: 登录 | Esc: 关闭"))
		return preserveBackgroundAfterReset(fillRenderedBackground(b.String(), width, fill), colorBg)
	}
	for i, option := range m.Options {
		prefix := "  "
		if i == m.Selected {
			prefix = "→ "
		}
		line := fill.Render(prefix) + fill.Render(option)
		b.WriteString(fillRenderedBackground(line, contentWidth, fill))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(vDialogHelpStyle.Render("Enter: 确认 | Esc: 关闭"))
	return preserveBackgroundAfterReset(fillRenderedBackground(b.String(), width, fill), colorBg)
}

func (m SessionPromptDialogModel) renderFramedInput(input string, outerWidth, innerWidth int, focused bool) string {
	input = cleanTextInputView(input)
	style := vSearchInput.Width(outerWidth)
	if focused {
		style = vSearchInputFocused.Width(outerWidth)
	}
	input = preserveBackgroundAfterReset(
		fillRenderedBackground(input, innerWidth, lipgloss.NewStyle().Background(colorSurface).Foreground(colorText)),
		colorSurface,
	)
	return style.Render(input)
}

func (m SessionPromptDialogModel) renderCredentialField(label, input string, focused bool, width int, fill lipgloss.Style) string {
	prefix := "  "
	if focused {
		prefix = "→ "
	}
	labelStyle := vStatLabelStyle.Background(colorBg).Width(8)
	labelLine := fill.Render(prefix) +
		labelStyle.Render(label) +
		fill.Render(" ")
	inputLines := strings.Split(input, "\n")
	for i, line := range inputLines {
		inputLines[i] = fillRenderedBackground(fill.Render("  ")+line, width, fill)
	}
	return fillRenderedBackground(labelLine, width, fill) + "\n" +
		strings.Join(inputLines, "\n")
}
