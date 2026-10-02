package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type V1User struct {
	ID        int
	Email     string
	Password  string
	Name      string
	CreatedAt time.Time
}

type V1QRCode struct {
	ID             int
	UserID         int
	ShortCode      string
	DestinationURL string
	Title          string
	CreatedAt      time.Time
}

func main() {
	v1DBUrl := flag.String("v1-db", "", "Source PostgreSQL connection string for v1 database")
	v2DBUrl := flag.String("v2-db", "", "Destination PostgreSQL connection string for v2 database")
	dryRun := flag.Bool("dry-run", false, "Simulate migration without modifying v2 database")
	flag.Parse()

	if *v1DBUrl == "" || *v2DBUrl == "" {
		fmt.Println("Usage: migrate-v1 --v1-db=<connection_string> --v2-db=<connection_string> [--dry-run]")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Println("[INFO] Connecting to v1 source database...")
	v1DB, err := sql.Open("pgx", *v1DBUrl)
	if err != nil {
		log.Fatalf("[FATAL] Failed to connect to v1 database: %v", err)
	}
	defer v1DB.Close()

	log.Println("[INFO] Connecting to v2 destination database...")
	v2DB, err := sql.Open("pgx", *v2DBUrl)
	if err != nil {
		log.Fatalf("[FATAL] Failed to connect to v2 database: %v", err)
	}
	defer v2DB.Close()

	if err := v1DB.PingContext(ctx); err != nil {
		log.Fatalf("[FATAL] v1 ping failed: %v", err)
	}
	if err := v2DB.PingContext(ctx); err != nil {
		log.Fatalf("[FATAL] v2 ping failed: %v", err)
	}

	log.Printf("[INFO] Starting migration (dry_run=%v)...", *dryRun)

	// 1. Fetch v1 users
	userRows, err := v1DB.QueryContext(ctx, `
		SELECT id, email, password, COALESCE(name, ''), created_at
		FROM users
		ORDER BY id ASC
	`)
	if err != nil {
		log.Fatalf("[FATAL] Query v1 users failed: %v", err)
	}
	defer userRows.Close()

	var v1Users []V1User
	for userRows.Next() {
		var u V1User
		if err := userRows.Scan(&u.ID, &u.Email, &u.Password, &u.Name, &u.CreatedAt); err != nil {
			log.Fatalf("[FATAL] Scan user failed: %v", err)
		}
		v1Users = append(v1Users, u)
	}

	log.Printf("[INFO] Found %d users in v1 database.", len(v1Users))

	userMap := make(map[int]uuid.UUID)
	wsMap := make(map[int]uuid.UUID)

	migratedUsers := 0
	for _, u := range v1Users {
		newUserID := uuid.New()
		newWsID := uuid.New()
		userMap[u.ID] = newUserID
		wsMap[u.ID] = newWsID

		if !*dryRun {
			// Insert user
			_, err := v2DB.ExecContext(ctx, `
				INSERT INTO users (id, email, password_hash, display_name, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $5)
				ON CONFLICT (email) DO UPDATE SET updated_at = now()
			`, newUserID, strings.ToLower(strings.TrimSpace(u.Email)), u.Password, u.Name, u.CreatedAt)
			if err != nil {
				log.Printf("[WARN] Failed to insert user %s: %v", u.Email, err)
				continue
			}

			// Insert default personal workspace
			slug := fmt.Sprintf("ws-%s", strings.Split(strings.ToLower(u.Email), "@")[0])
			_, err = v2DB.ExecContext(ctx, `
				INSERT INTO workspaces (id, name, slug, plan_id, created_at, updated_at)
				VALUES ($1, $2, $3, 'free', $4, $4)
				ON CONFLICT (slug) DO NOTHING
			`, newWsID, "Personal", slug, u.CreatedAt)
			if err != nil {
				log.Printf("[WARN] Failed to insert workspace for user %s: %v", u.Email, err)
				continue
			}

			// Insert workspace membership (owner)
			_, _ = v2DB.ExecContext(ctx, `
				INSERT INTO workspace_members (workspace_id, user_id, role, created_at)
				VALUES ($1, $2, 'owner', $3)
				ON CONFLICT DO NOTHING
			`, newWsID, newUserID, u.CreatedAt)
		}
		migratedUsers++
	}

	log.Printf("[INFO] Migrated %d users and workspaces to v2.", migratedUsers)

	// 2. Fetch v1 QR records
	qrRows, err := v1DB.QueryContext(ctx, `
		SELECT id, user_id, short_code, destination_url, COALESCE(title, 'Migrated QR'), created_at
		FROM qr_records
		ORDER BY id ASC
	`)
	if err != nil {
		log.Printf("[WARN] Query v1 qr_records failed (table might be named differently): %v", err)
		return
	}
	defer qrRows.Close()

	var v1Codes []V1QRCode
	for qrRows.Next() {
		var q V1QRCode
		if err := qrRows.Scan(&q.ID, &q.UserID, &q.ShortCode, &q.DestinationURL, &q.Title, &q.CreatedAt); err != nil {
			log.Printf("[WARN] Scan QR code failed: %v", err)
			continue
		}
		v1Codes = append(v1Codes, q)
	}

	log.Printf("[INFO] Found %d QR codes in v1 database.", len(v1Codes))

	migratedCodes := 0
	for _, q := range v1Codes {
		wsID, ok := wsMap[q.UserID]
		if !ok {
			continue
		}
		userID := userMap[q.UserID]
		newCodeID := uuid.New()
		newVersionID := uuid.New()

		if !*dryRun {
			// Insert QR code record with legacy_short_code populated
			_, err := v2DB.ExecContext(ctx, `
				INSERT INTO qr_codes (
					id, workspace_id, created_by, mode, content_type, name,
					legacy_short_code, status, safety_status, created_at, updated_at
				)
				VALUES ($1, $2, $3, 'dynamic', 'url', $4, $5, 'active', 'safe', $6, $6)
				ON CONFLICT DO NOTHING
			`, newCodeID, wsID, userID, q.Title, q.ShortCode, q.CreatedAt)
			if err != nil {
				log.Printf("[WARN] Failed to insert QR code %s: %v", q.ShortCode, err)
				continue
			}

			// Insert initial destination version
			_, err = v2DB.ExecContext(ctx, `
				INSERT INTO qr_code_versions (
					id, qr_code_id, version_number, destination_url, routing_rules, created_at
				)
				VALUES ($1, $2, 1, $3, '[]'::jsonb, $4)
				ON CONFLICT DO NOTHING
			`, newVersionID, newCodeID, q.DestinationURL, q.CreatedAt)
			if err != nil {
				log.Printf("[WARN] Failed to insert QR version for %s: %v", q.ShortCode, err)
				continue
			}

			// Update current_version_id
			_, _ = v2DB.ExecContext(ctx, `
				UPDATE qr_codes SET current_version_id = $1 WHERE id = $2
			`, newVersionID, newCodeID)
		}
		migratedCodes++
	}

	log.Printf("[INFO] Migration complete! Summary: %d users, %d dynamic QR codes migrated.", migratedUsers, migratedCodes)
}
