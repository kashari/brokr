package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kashari/brokr/config"
	"github.com/kashari/brokr/persistence"
)

func main() {
	dir := flag.String("dir", "workflows", "directory of workflow definition JSON files")
	flag.Parse()

	config.InitDB()
	if err := config.Db.AutoMigrate(&persistence.WorkflowDefinition{}); err != nil {
		fmt.Fprintf(os.Stderr, "auto-migrate: %v\n", err)
		os.Exit(1)
	}

	n, err := Run(config.Db, *dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed workflows: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("registered %d workflow definition(s) from %s\n", n, *dir)
}
