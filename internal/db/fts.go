package db

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrFTS5Unavailable = errors.New("SQLite FTS5 is unavailable; rebuild with -tags sqlite_fts5")

func (d *Database) FTS5Available() (bool, error) {
	if d == nil || d.db == nil {
		return false, errors.New("database is not initialized")
	}
	if d.dbType != "sqlite3" {
		return false, nil
	}
	var enabled int
	if err := d.db.Raw("SELECT sqlite_compileoption_used('ENABLE_FTS5')").Scan(&enabled).Error; err != nil {
		return false, err
	}
	return enabled == 1, nil
}

func createFTSSchema(tx *gorm.DB) error {
	statements := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(pid UNINDEXED, text, tokenize='trigram')`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS comments_fts USING fts5(cid UNINDEXED, pid UNINDEXED, text, name_tag, tokenize='trigram')`,
		`CREATE TRIGGER IF NOT EXISTS posts_fts_insert AFTER INSERT ON posts BEGIN
			INSERT INTO posts_fts(pid, text) VALUES (new.pid, COALESCE(new.text, ''));
		END`,
		`CREATE TRIGGER IF NOT EXISTS posts_fts_update AFTER UPDATE OF pid, text ON posts BEGIN
			DELETE FROM posts_fts WHERE pid = old.pid;
			INSERT INTO posts_fts(pid, text) VALUES (new.pid, COALESCE(new.text, ''));
		END`,
		`CREATE TRIGGER IF NOT EXISTS posts_fts_delete AFTER DELETE ON posts BEGIN
			DELETE FROM posts_fts WHERE pid = old.pid;
		END`,
		`CREATE TRIGGER IF NOT EXISTS comments_fts_insert AFTER INSERT ON comments BEGIN
			INSERT INTO comments_fts(cid, pid, text, name_tag)
			VALUES (new.cid, new.pid, COALESCE(new.text, ''), COALESCE(new.name_tag, ''));
		END`,
		`CREATE TRIGGER IF NOT EXISTS comments_fts_update AFTER UPDATE OF cid, pid, text, name_tag ON comments BEGIN
			DELETE FROM comments_fts WHERE cid = old.cid;
			INSERT INTO comments_fts(cid, pid, text, name_tag)
			VALUES (new.cid, new.pid, COALESCE(new.text, ''), COALESCE(new.name_tag, ''));
		END`,
		`CREATE TRIGGER IF NOT EXISTS comments_fts_delete AFTER DELETE ON comments BEGIN
			DELETE FROM comments_fts WHERE cid = old.cid;
		END`,
		`DELETE FROM posts_fts`,
		`INSERT INTO posts_fts(pid, text) SELECT pid, COALESCE(text, '') FROM posts`,
		`DELETE FROM comments_fts`,
		`INSERT INTO comments_fts(cid, pid, text, name_tag)
		 SELECT cid, pid, COALESCE(text, ''), COALESCE(name_tag, '') FROM comments`,
	}
	for _, statement := range statements {
		if err := tx.Exec(statement).Error; err != nil {
			return fmt.Errorf("create FTS5 schema: %w", err)
		}
	}
	return nil
}

func (d *Database) RebuildSearchIndex() error {
	available, err := d.FTS5Available()
	if err != nil {
		return err
	}
	if !available {
		return ErrFTS5Unavailable
	}
	return d.db.Transaction(func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable("posts_fts") || !tx.Migrator().HasTable("comments_fts") {
			return createFTSSchema(tx)
		}
		for _, statement := range []string{
			"DELETE FROM posts_fts",
			"INSERT INTO posts_fts(pid, text) SELECT pid, COALESCE(text, '') FROM posts",
			"DELETE FROM comments_fts",
			"INSERT INTO comments_fts(cid, pid, text, name_tag) SELECT cid, pid, COALESCE(text, ''), COALESCE(name_tag, '') FROM comments",
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
