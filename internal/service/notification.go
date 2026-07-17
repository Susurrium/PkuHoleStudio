package service

import (
	"context"
	"errors"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type notificationRemote interface {
	ListNotificationsV3Context(context.Context, models.NotificationType, int, int) ([]models.Notification, int, error)
	SetNotificationReadV3Context(context.Context, int) error
	SetAllNotificationsReadV3Context(context.Context, models.NotificationType) error
}

type NotificationService struct{ remote notificationRemote }

func NewNotificationService(client *client.Client) *NotificationService {
	if client == nil {
		return &NotificationService{}
	}
	return &NotificationService{remote: client}
}

func newNotificationService(remote notificationRemote) *NotificationService {
	return &NotificationService{remote: remote}
}

func (s *NotificationService) List(ctx context.Context, messageType models.NotificationType, page, limit int) ([]models.Notification, int, error) {
	if err := contextError(ctx); err != nil {
		return nil, 0, err
	}
	if s == nil || s.remote == nil {
		return nil, 0, errors.New("notification service is not configured")
	}
	if messageType != models.NotificationTypeInteractive && messageType != models.NotificationTypeSystem {
		return nil, 0, errors.New("unsupported notification type")
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	items, total, err := s.remote.ListNotificationsV3Context(ctx, messageType, page, limit)
	return items, total, err
}

func (s *NotificationService) MarkRead(ctx context.Context, id int) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil || s.remote == nil {
		return errors.New("notification service is not configured")
	}
	if id <= 0 {
		return errors.New("notification id must be positive")
	}
	writeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	return s.remote.SetNotificationReadV3Context(writeCtx, id)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, messageType models.NotificationType) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil || s.remote == nil {
		return errors.New("notification service is not configured")
	}
	if messageType != models.NotificationTypeInteractive && messageType != models.NotificationTypeSystem {
		return errors.New("unsupported notification type")
	}
	writeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	return s.remote.SetAllNotificationsReadV3Context(writeCtx, messageType)
}
