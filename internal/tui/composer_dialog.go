package tui

import (
	"fmt"
	"strings"

	"treehole/internal/models"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ComposerMode int

const (
	ComposerModePost ComposerMode = iota
	ComposerModeComment
)

type ComposerDialogModel struct {
	input       textarea.Model
	mode        ComposerMode
	title       string
	description string
	errorText   string
	quoteTarget *models.Comment
}

const composerPlaceholder = "输入内容"

func NewComposerDialog() ComposerDialogModel {
	input := textarea.New()
	input.Placeholder = composerPlaceholder
	input.CharLimit = 2000
	input.SetWidth(50)
	input.SetHeight(6)
	input.ShowLineNumbers = false
	input.Prompt = ""
	styleTextarea(&input, colorBg, colorText, colorMuted)
	_ = input.Focus()
	return ComposerDialogModel{input: input, title: "发布内容"}
}

func (m ComposerDialogModel) initialized() bool {
	return m.input.Width() > 0
}

func (m *ComposerDialogModel) Configure(mode ComposerMode) {
	m.mode = mode
	m.errorText = ""
	m.quoteTarget = nil
	m.input.Reset()
	m.input.Placeholder = composerPlaceholder
	styleTextarea(&m.input, colorBg, colorText, colorMuted)
	_ = m.input.Focus()
	m.description = "支持多行输入；Enter 换行，Ctrl+S 提交"
	if mode == ComposerModeComment {
		m.title = "发布评论"
	} else {
		m.title = "发布帖子"
	}
}

func (m *ComposerDialogModel) SetQuoteTarget(comment *models.Comment) {
	m.quoteTarget = comment
}

func (m *ComposerDialogModel) QuoteTarget() *models.Comment {
	return m.quoteTarget
}

func (m *ComposerDialogModel) SetError(err error) {
	if err == nil {
		m.errorText = ""
		return
	}
	m.errorText = err.Error()
}

func (m *ComposerDialogModel) Update(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m ComposerDialogModel) Value() string {
	return strings.TrimSpace(m.input.Value())
}

func (m ComposerDialogModel) Mode() ComposerMode {
	return m.mode
}

func (m ComposerDialogModel) quotePreview(width int) string {
	if m.quoteTarget == nil {
		return ""
	}
	name := m.quoteTarget.NameTag
	if name == "" {
		name = "匿名"
	}
	preview := fmt.Sprintf("引用 #%d %s: %s", m.quoteTarget.Cid, name, strings.ReplaceAll(m.quoteTarget.Text, "\n", " "))
	return truncateVisibleLine(preview, width, "...")
}

func (m ComposerDialogModel) View(width, height int) string {
	var b strings.Builder

	innerWidth := maxInt(24, width-panelContentStyle.GetHorizontalFrameSize())
	inputWidth := maxInt(20, innerWidth-4)
	preview := m.quotePreview(innerWidth - 2)
	errorHeight := 0
	if m.errorText != "" {
		errorHeight = lipgloss.Height(vErrorStyle.Render(m.errorText)) + 2
	}
	previewHeight := 0
	if preview != "" {
		previewHeight = lipgloss.Height(vCommentQuoteStyle.Width(innerWidth).Render(preview)) + 1
	}
	inputHeight := height - panelContentStyle.GetVerticalFrameSize() - 9 - previewHeight - errorHeight
	inputHeight = clampInt(inputHeight, 6, 16)

	input := m.input
	input.SetWidth(inputWidth)
	input.SetHeight(inputHeight)

	b.WriteString(vDialogTitleStyle.Render(m.title))
	b.WriteString("\n\n")
	b.WriteString(m.description)
	if preview != "" {
		b.WriteString("\n")
		b.WriteString(vCommentQuoteStyle.Width(innerWidth).Render(preview))
	}
	b.WriteString("\n\n")
	inputView := m.renderInputBlock(input, inputWidth, inputHeight)
	b.WriteString(inputView)
	if m.errorText != "" {
		b.WriteString("\n\n")
		b.WriteString(vErrorStyle.Render(m.errorText))
	}
	b.WriteString("\n\n")
	b.WriteString(vDialogHelpStyle.Render("Enter: 换行 | Ctrl+S: 提交 | Esc: 取消"))
	return b.String()
}

func (m ComposerDialogModel) renderInputBlock(input textarea.Model, width, height int) string {
	fill := lipgloss.NewStyle().Background(colorBg).Foreground(colorText)
	if input.Value() == "" {
		lines := make([]string, 0, height)
		lines = append(lines, lipgloss.NewStyle().Background(colorBg).Foreground(colorMuted).Width(width).Render(composerPlaceholder))
		for len(lines) < height {
			lines = append(lines, fill.Width(width).Render(""))
		}
		return strings.Join(lines, "\n")
	}
	return fill.Width(width).Height(height).Render(input.View())
}
