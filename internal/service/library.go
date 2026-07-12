package service

import (
	"context"
	"errors"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type LocalMetadataRepository interface {
	ListLocalTags() ([]models.LocalTag, error)
	CreateLocalTag(string, string) (models.LocalTag, error)
	UpdateLocalTag(uint, string, string) (models.LocalTag, error)
	DeleteLocalTag(uint) error
	GetPostTags(int32) ([]models.LocalTag, error)
	SetPostTags(int32, []uint) error
	GetNote(string, int64) (*models.Note, error)
	UpsertNote(string, int64, string) (models.Note, error)
}

type LocalLibraryService struct{ repository LocalMetadataRepository }

func NewLocalLibraryService(repository LocalMetadataRepository) *LocalLibraryService {
	return &LocalLibraryService{repository: repository}
}

func (s *LocalLibraryService) ready(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if s == nil || s.repository == nil {
		return errRepositoryUnavailable
	}
	return nil
}

func (s *LocalLibraryService) Tags(ctx context.Context) ([]models.LocalTag, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.repository.ListLocalTags()
}

func (s *LocalLibraryService) CreateTag(ctx context.Context, name, color string) (models.LocalTag, error) {
	if err := s.ready(ctx); err != nil {
		return models.LocalTag{}, err
	}
	return s.repository.CreateLocalTag(name, color)
}

func (s *LocalLibraryService) UpdateTag(ctx context.Context, id uint, name, color string) (models.LocalTag, error) {
	if err := s.ready(ctx); err != nil {
		return models.LocalTag{}, err
	}
	return s.repository.UpdateLocalTag(id, name, color)
}

func (s *LocalLibraryService) DeleteTag(ctx context.Context, id uint) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.repository.DeleteLocalTag(id)
}

func (s *LocalLibraryService) PostTags(ctx context.Context, pid int32) ([]models.LocalTag, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	if pid <= 0 {
		return nil, errors.New("pid must be positive")
	}
	return s.repository.GetPostTags(pid)
}

func (s *LocalLibraryService) SetPostTags(ctx context.Context, pid int32, ids []uint) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	if pid <= 0 {
		return errors.New("pid must be positive")
	}
	return s.repository.SetPostTags(pid, ids)
}

func (s *LocalLibraryService) Note(ctx context.Context, ownerType string, ownerID int64) (*models.Note, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	if (ownerType != "post" && ownerType != "comment") || ownerID <= 0 {
		return nil, errors.New("invalid note owner")
	}
	return s.repository.GetNote(ownerType, ownerID)
}

func (s *LocalLibraryService) SaveNote(ctx context.Context, ownerType string, ownerID int64, content string) (models.Note, error) {
	if err := s.ready(ctx); err != nil {
		return models.Note{}, err
	}
	if (ownerType != "post" && ownerType != "comment") || ownerID <= 0 {
		return models.Note{}, errors.New("invalid note owner")
	}
	if len(content) > 100_000 {
		return models.Note{}, errors.New("note is too long")
	}
	return s.repository.UpsertNote(ownerType, ownerID, strings.TrimSpace(content))
}
