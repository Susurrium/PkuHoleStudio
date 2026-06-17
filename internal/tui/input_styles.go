package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

func styleTextInput(input *textinput.Model, bg, textColor, placeholderColor lipgloss.TerminalColor) {
	base := lipgloss.NewStyle().Background(bg)
	input.PromptStyle = base
	input.TextStyle = base.Foreground(textColor)
	input.PlaceholderStyle = base.Foreground(placeholderColor)
	input.CompletionStyle = base.Foreground(placeholderColor)
}

func styleTextarea(input *textarea.Model, bg, textColor, placeholderColor lipgloss.TerminalColor) {
	base := lipgloss.NewStyle().Background(bg)
	focused := input.FocusedStyle
	focused.Base = base
	focused.CursorLine = lipgloss.NewStyle().Background(bg)
	focused.EndOfBuffer = base.Foreground(textColor)
	focused.Placeholder = base.Foreground(placeholderColor)
	focused.Prompt = base.Foreground(textColor)
	focused.Text = base.Foreground(textColor)

	blurred := input.BlurredStyle
	blurred.Base = base
	blurred.CursorLine = lipgloss.NewStyle().Background(bg)
	blurred.EndOfBuffer = base.Foreground(textColor)
	blurred.Placeholder = base.Foreground(placeholderColor)
	blurred.Prompt = base.Foreground(textColor)
	blurred.Text = base.Foreground(textColor)

	input.FocusedStyle = focused
	input.BlurredStyle = blurred
	if input.Focused() {
		input.Focus()
	} else {
		input.Blur()
	}
}

func fillRenderedBackground(rendered string, width int, fill lipgloss.Style) string {
	if width < 1 || rendered == "" {
		return rendered
	}

	hasTrailingNewline := strings.HasSuffix(rendered, "\n")
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		missing := width - lipgloss.Width(trimmed)
		if missing < 0 {
			missing = 0
		}
		lines[i] = trimmed + fill.Render(strings.Repeat(" ", missing))
	}

	result := strings.Join(lines, "\n")
	if hasTrailingNewline {
		result += "\n"
	}
	return result
}
