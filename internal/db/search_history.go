package db

import (
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

func (d *Database) RecordSearch(query, filtersJSON string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	if err := d.db.Create(&models.SearchHistory{Query: query, FiltersJSON: filtersJSON, CreatedAt: time.Now().UTC()}).Error; err != nil {
		return err
	}
	return d.db.Exec(`DELETE FROM search_history WHERE id NOT IN (SELECT id FROM search_history ORDER BY created_at DESC, id DESC LIMIT 100)`).Error
}

func (d *Database) ListSearchHistory(limit int) ([]models.SearchHistory, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []models.SearchHistory
	err := d.db.Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
