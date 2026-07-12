package service

import (
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type notificationRemoteStub struct {
	readID int
	all    models.NotificationType
}

func (s *notificationRemoteStub) ListNotificationsV3(kind models.NotificationType, page, limit int) ([]models.Notification, int, error) {
	return []models.Notification{{ID: page, Type: kind}}, limit, nil
}
func (s *notificationRemoteStub) SetNotificationReadV3(id int) error { s.readID = id; return nil }
func (s *notificationRemoteStub) SetAllNotificationsReadV3(kind models.NotificationType) error {
	s.all = kind
	return nil
}

func TestNotificationServiceListsAndMarksRead(t *testing.T) {
	remote := &notificationRemoteStub{}
	service := newNotificationService(remote)
	items, total, err := service.List(t.Context(), models.NotificationTypeInteractive, 2, 20)
	if err != nil || len(items) != 1 || items[0].ID != 2 || total != 20 {
		t.Fatalf("List() = %+v, %d, %v", items, total, err)
	}
	if err := service.MarkRead(t.Context(), 9); err != nil {
		t.Fatal(err)
	}
	if err := service.MarkAllRead(t.Context(), models.NotificationTypeSystem); err != nil {
		t.Fatal(err)
	}
	if remote.readID != 9 || remote.all != models.NotificationTypeSystem {
		t.Fatalf("remote = %+v", remote)
	}
}
