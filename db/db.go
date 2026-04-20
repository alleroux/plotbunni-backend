package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrations source: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}

	// Check if there are pending migrations before taking a backup.
	version, dirty, err := m.Version()
	hasPending := err == migrate.ErrNilVersion || (err == nil && !dirty)
	if hasPending && err != migrate.ErrNilVersion {
		// Only back up when there is actually something to migrate.
		if backupErr := backupDatabase(); backupErr != nil {
			log.Printf("WARNING: pre-migration backup failed: %v", backupErr)
		}
	}
	_ = version
	_ = dirty

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// backupDatabase runs pg_dump and writes the output to backups/<timestamp>.sql.
// It requires pg_dump to be on the PATH (available inside the postgres Docker image,
// or installed locally alongside PostgreSQL).
func backupDatabase() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL not set, skipping backup")
	}

	if err := os.MkdirAll("backups", 0755); err != nil {
		return fmt.Errorf("create backups dir: %w", err)
	}

	filename := fmt.Sprintf("backups/%s.sql", time.Now().UTC().Format("20060102T150405Z"))
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	cmd := exec.Command("pg_dump", dsn)
	cmd.Stdout = f
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(filename)
		return fmt.Errorf("pg_dump: %w", err)
	}

	log.Printf("pre-migration backup written to %s", filename)
	return nil
}
