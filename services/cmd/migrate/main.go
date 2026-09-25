package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/its-aryansingh/qrit/services/db/migrations"
)

func main() {
	var dir = flag.String("dir", ".", "migration directory")
	flag.Parse()

	args := flag.Args()
	cmd := "up"
	if len(args) > 0 {
		cmd = args[0]
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set goose dialect: %v", err)
	}

	goose.SetBaseFS(migrations.FS)

	ctx := context.Background()
	log.Printf("running goose %s on migrations...", cmd)
	if err := goose.RunContext(ctx, cmd, db, *dir, args[1:]...); err != nil {
		log.Fatalf("goose run error: %v", err)
	}
	log.Println("migration completed successfully.")
}
