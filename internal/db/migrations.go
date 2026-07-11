package db

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/models"

	"gorm.io/gorm"
)

type MigrationInfo struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	AppliedAt time.Time `json:"applied_at"`
}

type schemaMigration struct {
	version int
	name    string
	apply   func(*gorm.DB) error
}

func (d *Database) runMigrations() error {
	if d == nil || d.db == nil {
		return errors.New("database is not initialized")
	}
	if err := d.ensureMigrationTable(); err != nil {
		return err
	}
	if err := d.establishLegacyBaseline(); err != nil {
		return err
	}
	return d.applyMigrations(d.schemaMigrations())
}

func (d *Database) schemaMigrations() []schemaMigration {
	return []schemaMigration{
		{
			version: 1,
			name:    "pkuholetui baseline",
			apply: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.Post{}, &models.Comment{}, &models.ExclusiveIdInfo{})
			},
		},
		{
			version: 2,
			name:    "local library, jobs, imports, and ai metadata",
			apply: func(tx *gorm.DB) error {
				return tx.AutoMigrate(
					&models.ExclusiveIdInfo{},
					&models.SyncRun{},
					&models.SyncRunItem{},
					&models.ImportRun{},
					&models.PostSource{},
					&models.LocalTag{},
					&models.PostTag{},
					&models.Note{},
					&models.Reference{},
					&models.Media{},
					&models.SearchHistory{},
					&models.AISession{},
					&models.AIMessage{},
					&models.AISource{},
					&models.Job{},
					&models.JobItem{},
					&models.JobEvent{},
				)
			},
		},
	}
}

func (d *Database) ensureMigrationTable() error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		return tx.AutoMigrate(&models.SchemaMigration{})
	})
}

// establishLegacyBaseline recognizes the exact pre-migration shape without
// rewriting its posts/comments tables. New metadata is added by later versions.
func (d *Database) establishLegacyBaseline() error {
	var count int64
	if err := d.db.Model(&models.SchemaMigration{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count schema migrations: %w", err)
	}
	if count != 0 {
		return nil
	}

	hasPosts := d.db.Migrator().HasTable(&models.Post{})
	hasComments := d.db.Migrator().HasTable(&models.Comment{})
	if !hasPosts && !hasComments {
		return nil
	}
	if hasPosts != hasComments {
		return errors.New("legacy database is incomplete: posts and comments tables must both exist")
	}

	for _, column := range []string{"pid", "text", "timestamp"} {
		if !d.db.Migrator().HasColumn(&models.Post{}, column) {
			return fmt.Errorf("legacy posts table is incompatible: missing %s column", column)
		}
	}
	for _, column := range []string{"cid", "pid", "text", "timestamp"} {
		if !d.db.Migrator().HasColumn(&models.Comment{}, column) {
			return fmt.Errorf("legacy comments table is incompatible: missing %s column", column)
		}
	}

	return d.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&models.SchemaMigration{
			Version:   1,
			Name:      "pkuholetui baseline (adopted)",
			AppliedAt: time.Now().UTC(),
		}).Error
	})
}

func (d *Database) applyMigrations(migrations []schemaMigration) error {
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	for _, migration := range migrations {
		var count int64
		if err := d.db.Model(&models.SchemaMigration{}).Where("version = ?", migration.version).Count(&count).Error; err != nil {
			return fmt.Errorf("check migration %d: %w", migration.version, err)
		}
		if count > 0 {
			continue
		}
		if migration.apply == nil {
			return fmt.Errorf("migration %d %q has no implementation", migration.version, migration.name)
		}
		if err := d.db.Transaction(func(tx *gorm.DB) error {
			if err := migration.apply(tx); err != nil {
				return err
			}
			return tx.Create(&models.SchemaMigration{
				Version:   migration.version,
				Name:      migration.name,
				AppliedAt: time.Now().UTC(),
			}).Error
		}); err != nil {
			return fmt.Errorf("apply migration %d %q: %w", migration.version, migration.name, err)
		}
	}
	return nil
}

func (d *Database) AppliedMigrations() ([]MigrationInfo, error) {
	if d == nil || d.db == nil {
		return nil, errors.New("database is not initialized")
	}
	var rows []models.SchemaMigration
	if err := d.db.Order("version ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]MigrationInfo, len(rows))
	for i, row := range rows {
		result[i] = MigrationInfo{Version: row.Version, Name: row.Name, AppliedAt: row.AppliedAt}
	}
	return result, nil
}

func (d *Database) SchemaVersion() (int, error) {
	migrations, err := d.AppliedMigrations()
	if err != nil {
		return 0, err
	}
	if len(migrations) == 0 {
		return 0, nil
	}
	return migrations[len(migrations)-1].Version, nil
}
