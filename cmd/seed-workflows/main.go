// Command seed-workflows registers workflow definitions from a directory of
// JSON files into a brokr database.
//
//	go run ./cmd/seed-workflows -dsn "$BROKR_DSN" -dir workflows
//
// It is a thin wrapper over brokr.Migrate and brokr.SeedDefinitions; an
// application that already holds a *gorm.DB should call those directly instead
// of shelling out to this.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kashari/brokr"
	"github.com/kashari/golog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dir := flag.String("dir", "workflows", "directory of workflow definition JSON files")
	dsn := flag.String("dsn", os.Getenv("BROKR_DSN"), "Postgres DSN; defaults to $BROKR_DSN")
	flag.Parse()

	if err := run(*dsn, *dir); err != nil {
		fmt.Fprintf(os.Stderr, "seed-workflows: %v\n", err)
		os.Exit(1)
	}
}

func run(dsn, dir string) error {
	if dsn == "" {
		return fmt.Errorf("no DSN: pass -dsn or set BROKR_DSN, e.g. %q",
			"host=localhost port=5432 user=brokr password=... dbname=workflow sslmode=disable")
	}
	if err := golog.Init("seed-workflows.log"); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if err := brokr.Migrate(db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	n, err := brokr.SeedDefinitions(db, dir)
	if err != nil {
		return err
	}
	fmt.Printf("registered %d workflow definition(s) from %s\n", n, dir)
	return nil
}
