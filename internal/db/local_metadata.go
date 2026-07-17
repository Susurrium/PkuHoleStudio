package db

import (
	"errors"
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *Database) ListLocalTags() ([]models.LocalTag, error) {
	var tags []models.LocalTag
	err := d.db.Order("name ASC").Find(&tags).Error
	return tags, err
}

func (d *Database) CreateLocalTag(name, color string) (models.LocalTag, error) {
	now := time.Now().UTC()
	tag := models.LocalTag{Name: strings.TrimSpace(name), Color: strings.TrimSpace(color), CreatedAt: now, UpdatedAt: now}
	if tag.Name == "" {
		return tag, errors.New("tag name is required")
	}
	err := d.db.Create(&tag).Error
	return tag, err
}

func (d *Database) UpdateLocalTag(id uint, name, color string) (models.LocalTag, error) {
	if id == 0 || strings.TrimSpace(name) == "" {
		return models.LocalTag{}, errors.New("tag id and name are required")
	}
	result := d.db.Model(&models.LocalTag{}).Where("id = ?", id).Updates(map[string]any{"name": strings.TrimSpace(name), "color": strings.TrimSpace(color), "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return models.LocalTag{}, result.Error
	}
	if result.RowsAffected == 0 {
		return models.LocalTag{}, gorm.ErrRecordNotFound
	}
	var tag models.LocalTag
	err := d.db.First(&tag, id).Error
	return tag, err
}

func (d *Database) DeleteLocalTag(id uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ?", id).Delete(&models.PostTag{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.LocalTag{}, id).Error
	})
}

func (d *Database) GetPostTags(pid int32) ([]models.LocalTag, error) {
	var tags []models.LocalTag
	err := d.db.Table("local_tags").Joins("JOIN post_tags ON post_tags.tag_id = local_tags.id").Where("post_tags.pid = ?", pid).Order("local_tags.name ASC").Scan(&tags).Error
	return tags, err
}

func (d *Database) SetPostTags(pid int32, ids []uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("pid = ?", pid).Delete(&models.PostTag{}).Error; err != nil {
			return err
		}
		rows := make([]models.PostTag, 0, len(ids))
		now := time.Now().UTC()
		for _, id := range ids {
			if id > 0 {
				rows = append(rows, models.PostTag{PID: pid, TagID: id, CreatedAt: now})
			}
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
	})
}

func (d *Database) AddPostTags(pids []int32, ids []uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := validateVisiblePosts(tx, pids); err != nil {
			return err
		}
		var tagCount int64
		if err := tx.Model(&models.LocalTag{}).Where("id IN ?", ids).Count(&tagCount).Error; err != nil {
			return err
		}
		if tagCount != int64(len(ids)) {
			return errors.New("one or more local tags do not exist")
		}
		now := time.Now().UTC()
		rows := make([]models.PostTag, 0, len(pids)*len(ids))
		for _, pid := range pids {
			for _, id := range ids {
				rows = append(rows, models.PostTag{PID: pid, TagID: id, CreatedAt: now})
			}
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
	})
}

func (d *Database) ListResearchProjects() ([]models.ResearchProject, error) {
	var projects []models.ResearchProject
	err := d.db.Model(&models.ResearchProject{}).
		Select("research_projects.*, COUNT(project_posts.pid) AS post_count").
		Joins("LEFT JOIN project_posts ON project_posts.project_id = research_projects.id").
		Group("research_projects.id").
		Order("research_projects.updated_at DESC, research_projects.id DESC").
		Scan(&projects).Error
	return projects, err
}

func (d *Database) CreateResearchProject(name, description, color string) (models.ResearchProject, error) {
	now := time.Now().UTC()
	project := models.ResearchProject{
		Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), Color: strings.TrimSpace(color),
		CreatedAt: now, UpdatedAt: now,
	}
	if project.Name == "" {
		return project, errors.New("project name is required")
	}
	err := d.db.Create(&project).Error
	return project, err
}

func (d *Database) UpdateResearchProject(id uint, name, description, color string) (models.ResearchProject, error) {
	if id == 0 || strings.TrimSpace(name) == "" {
		return models.ResearchProject{}, errors.New("project id and name are required")
	}
	result := d.db.Model(&models.ResearchProject{}).Where("id = ?", id).Updates(map[string]any{
		"name": strings.TrimSpace(name), "description": strings.TrimSpace(description), "color": strings.TrimSpace(color), "updated_at": time.Now().UTC(),
	})
	if result.Error != nil {
		return models.ResearchProject{}, result.Error
	}
	if result.RowsAffected == 0 {
		return models.ResearchProject{}, gorm.ErrRecordNotFound
	}
	var project models.ResearchProject
	err := d.db.First(&project, id).Error
	return project, err
}

func (d *Database) DeleteResearchProject(id uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", id).Delete(&models.ProjectPost{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&models.ResearchProject{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (d *Database) GetPostProjects(pid int32) ([]models.ResearchProject, error) {
	var projects []models.ResearchProject
	err := d.db.Table("research_projects").
		Joins("JOIN project_posts ON project_posts.project_id = research_projects.id").
		Where("project_posts.pid = ?", pid).
		Order("research_projects.updated_at DESC, research_projects.id DESC").
		Scan(&projects).Error
	return projects, err
}

func (d *Database) SetPostProjects(pid int32, ids []uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		var visible int64
		if err := tx.Model(&models.Post{}).Where(visibleLibraryPostSQL).Where("pid = ?", pid).Count(&visible).Error; err != nil {
			return err
		}
		if visible == 0 {
			return errors.New("post is not in the visible local library")
		}
		unique := make([]uint, 0, len(ids))
		seen := make(map[uint]bool, len(ids))
		for _, id := range ids {
			if id > 0 && !seen[id] {
				seen[id] = true
				unique = append(unique, id)
			}
		}
		if len(unique) > 0 {
			var count int64
			if err := tx.Model(&models.ResearchProject{}).Where("id IN ?", unique).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(unique)) {
				return errors.New("one or more research projects do not exist")
			}
		}
		if err := tx.Where("pid = ?", pid).Delete(&models.ProjectPost{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		rows := make([]models.ProjectPost, 0, len(unique))
		for _, id := range unique {
			rows = append(rows, models.ProjectPost{ProjectID: id, PID: pid, CreatedAt: now})
		}
		if len(rows) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.ResearchProject{}).Where("id IN ?", unique).Update("updated_at", now).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *Database) AddPostsToProjects(pids []int32, ids []uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := validateVisiblePosts(tx, pids); err != nil {
			return err
		}
		var projectCount int64
		if err := tx.Model(&models.ResearchProject{}).Where("id IN ?", ids).Count(&projectCount).Error; err != nil {
			return err
		}
		if projectCount != int64(len(ids)) {
			return errors.New("one or more research projects do not exist")
		}
		now := time.Now().UTC()
		rows := make([]models.ProjectPost, 0, len(pids)*len(ids))
		for _, pid := range pids {
			for _, id := range ids {
				rows = append(rows, models.ProjectPost{ProjectID: id, PID: pid, CreatedAt: now})
			}
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
			return err
		}
		return tx.Model(&models.ResearchProject{}).Where("id IN ?", ids).Update("updated_at", now).Error
	})
}

func (d *Database) RemovePostsFromProject(pids []int32, projectID uint) (int64, error) {
	var removed int64
	err := d.db.Transaction(func(tx *gorm.DB) error {
		var projectCount int64
		if err := tx.Model(&models.ResearchProject{}).Where("id = ?", projectID).Count(&projectCount).Error; err != nil {
			return err
		}
		if projectCount != 1 {
			return errors.New("research project does not exist")
		}
		result := tx.Where("project_id = ? AND pid IN ?", projectID, pids).Delete(&models.ProjectPost{})
		if result.Error != nil {
			return result.Error
		}
		removed = result.RowsAffected
		return tx.Model(&models.ResearchProject{}).Where("id = ?", projectID).Update("updated_at", time.Now().UTC()).Error
	})
	return removed, err
}

func validateVisiblePosts(tx *gorm.DB, pids []int32) error {
	var count int64
	if err := tx.Model(&models.Post{}).Where(visibleLibraryPostSQL).Where("pid IN ?", pids).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(pids)) {
		return errors.New("one or more posts are not in the visible local library")
	}
	return nil
}

func (d *Database) GetResearchProjectPosts(id uint) ([]models.Post, error) {
	var posts []models.Post
	err := d.db.Table("posts").
		Joins("JOIN project_posts ON project_posts.pid = posts.pid").
		Where("project_posts.project_id = ?", id).
		Where(visibleLibraryPostSQL).
		Order("project_posts.created_at DESC, posts.pid DESC").
		Find(&posts).Error
	return posts, err
}

func (d *Database) GetNote(ownerType string, ownerID int64) (*models.Note, error) {
	var note models.Note
	err := d.db.Where("owner_type = ? AND owner_id = ?", ownerType, ownerID).First(&note).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &note, err
}

func (d *Database) UpsertNote(ownerType string, ownerID int64, content string) (models.Note, error) {
	now := time.Now().UTC()
	note := models.Note{OwnerType: ownerType, OwnerID: ownerID, Content: content, CreatedAt: now, UpdatedAt: now}
	err := d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_type"}, {Name: "owner_id"}},
		DoUpdates: clause.Assignments(map[string]any{"content": content, "updated_at": now}),
	}).Create(&note).Error
	if err != nil {
		return note, err
	}
	stored, err := d.GetNote(ownerType, ownerID)
	if err != nil {
		return note, err
	}
	if stored == nil {
		return note, errors.New("saved note could not be read")
	}
	return *stored, nil
}
