package database

import (
	"fmt"
	"log"

	"github.com/qrapp/backend/internal/config"
	"github.com/qrapp/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.DatabaseURL()

	logLevel := logger.Info
	if cfg.Environment == "production" {
		logLevel = logger.Error
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // Disables prepared statements and may help with auth
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate models
	if err := db.AutoMigrate(
		&models.User{},
		&models.QRType{},
		&models.QRRecord{},
		&models.QRScan{},
		&models.QRFile{},
		&models.FreeTierUsage{},
		// Enterprise models
		&models.Workspace{},
		&models.WorkspaceMember{},
		&models.WorkspaceInvite{},
		&models.Folder{},
		&models.Webhook{},
		&models.WebhookLog{},
		&models.AuditLog{},
		// Lead Capture models
		&models.LeadCapturePage{},
		&models.Lead{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database connected and migrated successfully")
	return db, nil
}
