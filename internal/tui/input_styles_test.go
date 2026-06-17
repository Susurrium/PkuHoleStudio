package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewSearchInputUsesSurfaceBackgroundForPlaceholderAndText(t *testing.T) {
	input := newSearchInput()

	if got := input.PlaceholderStyle.GetBackground(); got != colorSurface {
		t.Fatalf("search placeholder background = %v, want %v", got, colorSurface)
	}
	if got := input.TextStyle.GetBackground(); got != colorSurface {
		t.Fatalf("search text background = %v, want %v", got, colorSurface)
	}
	if got := input.PromptStyle.GetBackground(); got != colorSurface {
		t.Fatalf("search prompt background = %v, want %v", got, colorSurface)
	}
}

func TestNewComposerDialogUsesPanelBackgroundForPlaceholderAndText(t *testing.T) {
	dialog := NewComposerDialog()

	if got := dialog.input.FocusedStyle.Placeholder.GetBackground(); got != colorBg {
		t.Fatalf("composer placeholder background = %v, want %v", got, colorBg)
	}
	if got := dialog.input.FocusedStyle.Text.GetBackground(); got != colorBg {
		t.Fatalf("composer text background = %v, want %v", got, colorBg)
	}
	if got := dialog.input.FocusedStyle.Base.GetBackground(); got != colorBg {
		t.Fatalf("composer base background = %v, want %v", got, colorBg)
	}
}

func TestFillRenderedBackgroundReplacesPlainTrailingSpaces(t *testing.T) {
	fill := lipgloss.NewStyle().Background(colorSurface)
	rendered := "abc   "

	got := fillRenderedBackground(rendered, 4, fill)

	if lipgloss.Width(got) != 4 {
		t.Fatalf("filled output width = %d, want 4", lipgloss.Width(got))
	}
	if got == rendered {
		t.Fatalf("filled output should be normalized when trailing spaces exceed width: %q", got)
	}
}

func TestFillRenderedBackgroundPreservesLineWidthsAcrossNewlines(t *testing.T) {
	fill := lipgloss.NewStyle().Background(colorBg)
	rendered := "a  \nxy"

	got := fillRenderedBackground(rendered, 4, fill)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2", len(lines))
	}
	for i, line := range lines {
		if lipgloss.Width(line) != 4 {
			t.Fatalf("line %d width = %d, want 4", i, lipgloss.Width(line))
		}
	}
}
