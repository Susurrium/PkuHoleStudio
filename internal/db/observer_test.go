package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func TestObserverAvailabilityTransitionsAndInstanceScopedReceipts(t *testing.T) {
	database := openDatabaseAt(t, filepath.Join(t.TempDir(), "observer.db"))
	defer database.Close()
	if err := database.UpsertPosts([]models.Post{{Pid: 123456, Text: "captured"}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	t1 := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	apply := func(instance string, eventID int64, kind, state, hash, snapshot string, observed time.Time) bool {
		now := observed
		availability := models.PostAvailability{PID: 123456, State: state, ObservedAt: observed, ObserverID: "default", RemoteInstanceID: instance, SnapshotID: snapshot, UpdatedAt: now}
		if state == "confirmed_unavailable" {
			availability.FirstUnavailableAt, availability.LastUnavailableAt = &observed, &observed
		} else {
			availability.RestoredAt = &observed
		}
		applied, err := database.ApplyObserverEvent(ctx,
			models.ObserverSyncState{ObserverID: "default", RemoteInstanceID: instance, LastEventID: eventID, LastSuccessAt: &now, UpdatedAt: now},
			models.ObserverEventReceipt{ObserverID: "default", RemoteInstanceID: instance, EventID: eventID, PID: 123456, EventType: kind, PayloadHash: hash, AppliedAt: now}, availability)
		if err != nil {
			t.Fatal(err)
		}
		return applied
	}
	if !apply("instance-a", 1, "availability.confirmed_unavailable", "confirmed_unavailable", "hash-1", "snapshot-1", t1) {
		t.Fatal("first event was not applied")
	}
	t2 := t1.Add(time.Hour)
	if !apply("instance-a", 2, "availability.restored", "restored", "hash-2", "", t2) {
		t.Fatal("restore event was not applied")
	}
	restored, found, err := database.GetPostAvailability(ctx, 123456)
	if err != nil || !found || restored.RestoredAt == nil || restored.SnapshotID != "snapshot-1" || restored.FirstUnavailableAt == nil || restored.LastUnavailableAt == nil {
		t.Fatalf("restored=%+v found=%v err=%v", restored, found, err)
	}
	t3 := t2.Add(time.Hour)
	if !apply("instance-a", 3, "availability.confirmed_unavailable", "confirmed_unavailable", "hash-3", "snapshot-2", t3) {
		t.Fatal("second removal was not applied")
	}
	removed, _, err := database.GetPostAvailability(ctx, 123456)
	if err != nil || removed.RestoredAt != nil || removed.FirstUnavailableAt == nil || !removed.FirstUnavailableAt.Equal(t1) || removed.LastUnavailableAt == nil || !removed.LastUnavailableAt.Equal(t3) || removed.SnapshotID != "snapshot-2" {
		t.Fatalf("removed=%+v err=%v", removed, err)
	}
	if !apply("instance-b", 1, "availability.restored", "restored", "new-instance-hash", "", t3.Add(time.Hour)) {
		t.Fatal("same event ID from a new instance collided with old receipt")
	}
}

func TestSaveObserverBrowseRecordPersistsSelectedPostWithoutSyncReceipt(t *testing.T) {
	database := openDatabaseAt(t, filepath.Join(t.TempDir(), "observer-browse.db"))
	defer database.Close()
	ctx := context.Background()
	observedAt := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	post := models.Post{Pid: 8_449_490, Text: "selected remote deletion", Timestamp: 1_784_100_000}
	comments := []models.Comment{{Cid: 1, Pid: post.Pid, Text: "captured comment", Timestamp: 1_784_100_100}}
	availability := models.PostAvailability{PID: post.Pid, State: "confirmed_unavailable", ObservedAt: observedAt, FirstUnavailableAt: &observedAt, LastUnavailableAt: &observedAt, ObserverID: "default", RemoteInstanceID: "observer-a", Completeness: "complete"}
	if err := database.SaveObserverBrowseRecord(ctx, post, comments, availability, "observer-a"); err != nil {
		t.Fatal(err)
	}
	savedPost, savedComments, _, err := database.ObserverPostContents(ctx, post.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if savedPost == nil || savedPost.Text != post.Text || len(savedComments) != 1 || savedComments[0].Text != comments[0].Text {
		t.Fatalf("saved post=%+v comments=%+v", savedPost, savedComments)
	}
	savedAvailability, found, err := database.GetPostAvailability(ctx, post.Pid)
	if err != nil || !found || savedAvailability.State != "confirmed_unavailable" || savedAvailability.Completeness != "complete" {
		t.Fatalf("availability=%+v found=%v err=%v", savedAvailability, found, err)
	}
	if _, found, err := database.GetObserverSyncState(ctx, "default"); err != nil || found {
		t.Fatalf("browse save unexpectedly advanced sync state: found=%v err=%v", found, err)
	}
}
