package tui

import (
	"fmt"
	"strings"
	"testing"
	"treehole/internal/models"

	"charm.land/lipgloss/v2"
)

func TestHelpPanelAlignmentAcrossWidths(t *testing.T) {
	widths := []int{80, 100, 120, 138, 140, 160}
	for _, w := range widths {
		t.Run(fmt.Sprintf("w=%d", w), func(t *testing.T) {
			m := newTestModel()
			m.Page = PagePosts
			m.Width = w
			m.Height = 30
			m.Dialog = DialogHelp
			m.Posts.CanWrite = true
			m.Posts.PostList = []models.Post{{Pid: 1, Text: "hello", Timestamp: 1000}}

			items := m.helpItems()
			panelWidth := m.helpPanelWidth(w)
			cardWidth := panelWidth - helpCard.GetHorizontalFrameSize()
			if cardWidth < 18 {
				cardWidth = 18
			}
			innerWidth := cardWidth - helpCard.GetHorizontalPadding()
			keyWidth := maxInt(6, minInt(10, innerWidth/3))
			panel := stripANSI(m.renderHelpPanel(panelWidth))
			lines := strings.Split(strings.TrimSuffix(panel, "\n"), "\n")

			for _, item := range items {
				visibleKey := item.key
				if lipgloss.Width(visibleKey) > keyWidth {
					visibleKey = clipToVisibleWidth(visibleKey, keyWidth)
				}
				found := 0
				for _, line := range lines {
					if strings.Contains(line, visibleKey) && strings.Contains(line, item.desc) {
						found++
					}
				}
				if found != 1 {
					t.Errorf("item key=%q desc=%q (visibleKey=%q): found on %d combined lines, want 1",
						item.key, item.desc, visibleKey, found)
				}
			}

			cardStart := panelWidth - cardWidth
			borderOffset := helpCard.GetHorizontalFrameSize()/2 - helpCard.GetHorizontalPadding()/2
			contentStart := cardStart + borderOffset + helpCard.GetHorizontalPadding()/2
			expectedDescCol := contentStart + keyWidth + 1

			detected := 0
			for _, item := range items {
				visibleKey := item.key
				if lipgloss.Width(visibleKey) > keyWidth {
					visibleKey = clipToVisibleWidth(visibleKey, keyWidth)
				}
				for _, line := range lines {
					if !strings.Contains(line, visibleKey) || !strings.Contains(line, item.desc) {
						continue
					}
					descBytePos := strings.Index(line, item.desc)
					descPos := lipgloss.Width(line[:descBytePos])
					if descPos != expectedDescCol {
						t.Errorf("key=%q desc=%q: desc at col %d, want %d in line %q",
							item.key, item.desc, descPos, expectedDescCol, line)
					}
					detected++
					break
				}
			}
			if detected < 3 {
				t.Fatalf("detected only %d help-item rows in panel", detected)
			}
			t.Logf("panelWidth=%d items=%d keyWidth=%d contentStart=%d descCol=%d",
				panelWidth, len(items), keyWidth, contentStart, expectedDescCol)
		})
	}
}
