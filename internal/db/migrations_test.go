package db

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMigrationsCreateNewDatabaseAndRemainIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.db")
	database := openDatabaseAt(t, path)

	version, err := database.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion() error = %v", err)
	}
	if version != 2 {
		t.Fatalf("SchemaVersion() = %d, want 2", version)
	}
	for _, table := range []string{
		"schema_migrations", "posts", "comments", "exclusive_id_infos",
		"sync_runs", "sync_run_items", "import_runs", "post_sources",
		"local_tags", "post_tags", "notes", "references", "media",
		"search_history", "ai_sessions", "ai_messages", "ai_sources",
		"jobs", "job_items", "job_events",
	} {
		if !database.db.Migrator().HasTable(table) {
			t.Errorf("new database is missing table %q", table)
		}
	}
	if err := database.UpsertPosts([]models.Post{{Pid: 12345, Text: "preserved"}}); err != nil {
		t.Fatalf("UpsertPosts() error = %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened := openDatabaseAt(t, path)
	defer reopened.Close()
	migrations, err := reopened.AppliedMigrations()
	if err != nil {
		t.Fatalf("AppliedMigrations() error = %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("applied migrations after reopen = %d, want 2", len(migrations))
	}
	post, err := reopened.GetPostByPid(12345)
	if err != nil || post.Text != "preserved" {
		t.Fatalf("post after reopen = %+v, error = %v", post, err)
	}
}

func TestMigrationsAdoptLegacyDatabaseWithoutLosingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy := openRawSQLite(t, path)
	if err := legacy.AutoMigrate(&models.Post{}, &models.Comment{}, &models.ExclusiveIdInfo{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := legacy.Create(&models.Post{Pid: 54321, Text: "legacy", Timestamp: 10}).Error; err != nil {
		t.Fatalf("seed legacy post: %v", err)
	}
	closeRaw(t, legacy)

	database := openDatabaseAt(t, path)
	defer database.Close()
	migrations, err := database.AppliedMigrations()
	if err != nil {
		t.Fatalf("AppliedMigrations() error = %v", err)
	}
	if len(migrations) != 2 || migrations[0].Version != 1 || migrations[1].Version != 2 {
		t.Fatalf("legacy migrations = %+v", migrations)
	}
	if migrations[0].Name != "pkuholetui baseline (adopted)" {
		t.Fatalf("baseline name = %q", migrations[0].Name)
	}
	post, err := database.GetPostByPid(54321)
	if err != nil || post.Text != "legacy" {
		t.Fatalf("legacy post = %+v, error = %v", post, err)
	}
}

func TestMigrationsRejectIncompleteOrIncompatibleLegacySchema(t *testing.T) {
	for _, test := range []struct {
		name   string
		create []string
	}{
		{
			name: "posts only",
			create: []string{
				"CREATE TABLE posts (pid integer primary key, text text, timestamp integer)",
			},
		},
		{
			name: "missing required post column",
			create: []string{
				"CREATE TABLE posts (pid integer primary key, timestamp integer)",
				"CREATE TABLE comments (cid integer primary key, pid integer, text text, timestamp integer)",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "broken.db")
			raw := openRawSQLite(t, path)
			for _, statement := range test.create {
				if err := raw.Exec(statement).Error; err != nil {
					t.Fatalf("create broken legacy schema: %v", err)
				}
			}
			closeRaw(t, raw)

			cfg := sqliteConfigAt(path)
			database, err := NewDatabase(cfg)
			if err == nil {
				database.Close()
				t.Fatal("NewDatabase() accepted an incompatible legacy schema")
			}
		})
	}
}

func TestFailedMigrationRollsBackDDLAndVersionRecord(t *testing.T) {
	database := openDatabaseAt(t, filepath.Join(t.TempDir(), "rollback.db"))
	defer database.Close()

	err := database.applyMigrations([]schemaMigration{{
		version: 99,
		name:    "deliberate failure",
		apply: func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE TABLE migration_should_rollback (id integer primary key)").Error; err != nil {
				return err
			}
			return errors.New("boom")
		},
	}})
	if err == nil {
		t.Fatal("applyMigrations() succeeded, want failure")
	}
	if database.db.Migrator().HasTable("migration_should_rollback") {
		t.Fatal("failed migration left its DDL behind")
	}
	var count int64
	if err := database.db.Model(&models.SchemaMigration{}).Where("version = 99").Count(&count).Error; err != nil {
		t.Fatalf("count failed migration record: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed migration recorded %d version rows", count)
	}
}

func TestSQLiteForeignKeysEnabled(t *testing.T) {
	database := openDatabaseAt(t, filepath.Join(t.TempDir(), "foreign-keys.db"))
	defer database.Close()
	var enabled int
	if err := database.db.Raw("PRAGMA foreign_keys").Scan(&enabled).Error; err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", enabled)
	}
}

func openDatabaseAt(t *testing.T, path string) *Database {
	t.Helper()
	database, err := NewDatabase(sqliteConfigAt(path))
	if err != nil {
		t.Fatalf("NewDatabase(%q): %v", path, err)
	}
	t.Cleanup(func() {
		_ = os.Remove(path + "-wal")
		_ = os.Remove(path + "-shm")
	})
	return database
}

func sqliteConfigAt(path string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Database.Type = "sqlite3"
	cfg.Database.DBFile = path
	return &cfg
}

func openRawSQLite(t *testing.T, path string) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	return database
}

func closeRaw(t *testing.T, database *gorm.DB) {
	t.Helper()
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("raw DB(): %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close raw sqlite: %v", err)
	}
}
