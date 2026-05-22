package migration

import (
	"context"
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"gorm.io/gorm"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

func Run(ctx context.Context, db *gorm.DB) error {
	entries, err := migrationFiles.ReadDir("sql")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		if err := runOne(ctx, db, version, path.Join("sql", name)); err != nil {
			return err
		}
	}

	return nil
}

func runOne(ctx context.Context, db *gorm.DB, version string, filePath string) error {
	content, err := migrationFiles.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", version, err)
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if tx.Migrator().HasTable("schema_migrations") {
			if err := tx.Table("schema_migrations").Where("version = ?", version).Count(&count).Error; err != nil {
				return fmt.Errorf("check migration %s: %w", version, err)
			}
			if count > 0 {
				return nil
			}
		}

		if err := tx.Exec(string(content)).Error; err != nil {
			return fmt.Errorf("execute migration %s: %w", version, err)
		}

		if err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?) ON CONFLICT (version) DO NOTHING", version).Error; err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}

		return nil
	})
}
