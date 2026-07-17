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
	AddPostTags([]int32, []uint) error
	ListResearchProjects() ([]models.ResearchProject, error)
	CreateResearchProject(string, string, string) (models.ResearchProject, error)
	UpdateResearchProject(uint, string, string, string) (models.ResearchProject, error)
	DeleteResearchProject(uint) error
	GetPostProjects(int32) ([]models.ResearchProject, error)
	SetPostProjects(int32, []uint) error
	AddPostsToProjects([]int32, []uint) error
	RemovePostsFromProject([]int32, uint) (int64, error)
	GetResearchProjectPosts(uint) ([]models.Post, error)
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

func (s *LocalLibraryService) AddPostTags(ctx context.Context, pids []int32, ids []uint) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	if containsNonPositiveInt32(pids) || containsZeroUint(ids) {
		return errors.New("PIDs and tag ids must be positive")
	}
	pids = uniquePositiveInt32s(pids)
	ids = uniquePositiveUints(ids)
	if len(pids) == 0 || len(pids) > 200 || len(ids) == 0 || len(ids) > 20 {
		return errors.New("batch tag assignment requires 1 to 200 PIDs and 1 to 20 tags")
	}
	return s.repository.AddPostTags(pids, ids)
}

func (s *LocalLibraryService) Projects(ctx context.Context) ([]models.ResearchProject, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.repository.ListResearchProjects()
}

func (s *LocalLibraryService) CreateProject(ctx context.Context, name, description, color string) (models.ResearchProject, error) {
	if err := s.ready(ctx); err != nil {
		return models.ResearchProject{}, err
	}
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if name == "" || len(name) > 160 {
		return models.ResearchProject{}, errors.New("project name must contain 1 to 160 characters")
	}
	if len(description) > 10_000 {
		return models.ResearchProject{}, errors.New("project description is too long")
	}
	return s.repository.CreateResearchProject(name, description, color)
}

func (s *LocalLibraryService) UpdateProject(ctx context.Context, id uint, name, description, color string) (models.ResearchProject, error) {
	if err := s.ready(ctx); err != nil {
		return models.ResearchProject{}, err
	}
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if id == 0 || name == "" || len(name) > 160 || len(description) > 10_000 {
		return models.ResearchProject{}, errors.New("invalid research project")
	}
	return s.repository.UpdateResearchProject(id, name, description, color)
}

func (s *LocalLibraryService) DeleteProject(ctx context.Context, id uint) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	if id == 0 {
		return errors.New("project id must be positive")
	}
	return s.repository.DeleteResearchProject(id)
}

func (s *LocalLibraryService) PostProjects(ctx context.Context, pid int32) ([]models.ResearchProject, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	if pid <= 0 {
		return nil, errors.New("pid must be positive")
	}
	return s.repository.GetPostProjects(pid)
}

func (s *LocalLibraryService) SetPostProjects(ctx context.Context, pid int32, ids []uint) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	if pid <= 0 || len(ids) > 100 {
		return errors.New("invalid project assignment")
	}
	return s.repository.SetPostProjects(pid, ids)
}

func (s *LocalLibraryService) AddPostsToProjects(ctx context.Context, pids []int32, ids []uint) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	if containsNonPositiveInt32(pids) || containsZeroUint(ids) {
		return errors.New("PIDs and project ids must be positive")
	}
	pids = uniquePositiveInt32s(pids)
	ids = uniquePositiveUints(ids)
	if len(pids) == 0 || len(pids) > 200 || len(ids) == 0 || len(ids) > 20 {
		return errors.New("batch project assignment requires 1 to 200 PIDs and 1 to 20 projects")
	}
	return s.repository.AddPostsToProjects(pids, ids)
}

func (s *LocalLibraryService) RemovePostsFromProject(ctx context.Context, pids []int32, projectID uint) (int64, error) {
	if err := s.ready(ctx); err != nil {
		return 0, err
	}
	if containsNonPositiveInt32(pids) {
		return 0, errors.New("PIDs must be positive")
	}
	pids = uniquePositiveInt32s(pids)
	if len(pids) == 0 || len(pids) > 200 || projectID == 0 {
		return 0, errors.New("batch project removal requires 1 to 200 PIDs and one project")
	}
	return s.repository.RemovePostsFromProject(pids, projectID)
}

func uniquePositiveInt32s(values []int32) []int32 {
	result := make([]int32, 0, len(values))
	seen := make(map[int32]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniquePositiveUints(values []uint) []uint {
	result := make([]uint, 0, len(values))
	seen := make(map[uint]struct{}, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func containsNonPositiveInt32(values []int32) bool {
	for _, value := range values {
		if value <= 0 {
			return true
		}
	}
	return false
}

func containsZeroUint(values []uint) bool {
	for _, value := range values {
		if value == 0 {
			return true
		}
	}
	return false
}

func (s *LocalLibraryService) ProjectPosts(ctx context.Context, id uint) ([]models.Post, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, errors.New("project id must be positive")
	}
	return s.repository.GetResearchProjectPosts(id)
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
