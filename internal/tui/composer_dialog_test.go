package tui

import (
	"strings"
	"testing"

	"treehole/internal/models"
)

func TestComposerDialogViewUsesPanelSpaceAndShowsQuotePreview(t *testing.T) {
	dialog := NewComposerDialog()
	dialog.Configure(ComposerModeComment)
	dialog.SetQuoteTarget(&models.Comment{
		Cid:     23,
		NameTag: "tester",
		Text:    "quoted comment",
	})

	small := stripANSI(dialog.View(60, 18))
	large := stripANSI(dialog.View(90, 30))

	if !strings.Contains(large, "发布评论") {
		t.Fatalf("composer title missing from large panel:\n%s", large)
	}
	if !strings.Contains(large, "引用 #23 tester: quoted comment") {
		t.Fatalf("quote preview missing from large panel:\n%s", large)
	}
	if !strings.Contains(large, "Ctrl+S: 提交 | Esc: 取消") {
		t.Fatalf("composer help text missing from large panel:\n%s", large)
	}
	if len(frameLines(large)) <= len(frameLines(small)) {
		t.Fatalf("expected larger panel to render taller composer, small=%d large=%d", len(frameLines(small)), len(frameLines(large)))
	}
}
