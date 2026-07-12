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
