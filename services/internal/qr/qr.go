package qr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Mode string

const (
	ModeDynamic Mode = "dynamic"
	ModeStatic  Mode = "static"
)

type ContentType string

const (
	ContentTypeURL       ContentType = "url"
	ContentTypeText      ContentType = "text"
	ContentTypeEmail     ContentType = "email"
	ContentTypePhone     ContentType = "phone"
	ContentTypeSMS       ContentType = "sms"
	ContentTypeWhatsApp  ContentType = "whatsapp"
	ContentTypeWiFi      ContentType = "wifi"
	ContentTypeVCard     ContentType = "vcard"
	ContentTypeEvent     ContentType = "event"
	ContentTypeUPI       ContentType = "upi"
	ContentTypeLocation  ContentType = "location"
	ContentTypeAppStore  ContentType = "app_store"
	ContentTypeGS1       ContentType = "gs1"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusPaused   Status = "paused"
	StatusArchived Status = "archived"
	StatusBlocked  Status = "blocked"
)

type SafetyStatus string

const (
	SafetyPending SafetyStatus = "pending"
	SafetySafe    SafetyStatus = "safe"
	SafetyUnsafe  SafetyStatus = "unsafe"
	SafetyBlocked SafetyStatus = "blocked"
)

var (
	hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	ErrInvalidHex = errors.New("invalid hex color; expected format #RRGGBB")
	ErrInvalidECC = errors.New("invalid ecc level; expected auto, L, M, Q, or H")
)

// DesignV1 matches the canonical v1 design schema.
type DesignV1 struct {
	V          int               `json:"v"`
	ECC        string            `json:"ecc"`
	QuietZone  int               `json:"quiet_zone"`
	Modules    ModulesConfig     `json:"modules"`
	Finder     FinderConfig      `json:"finder"`
	Background BackgroundConfig  `json:"background"`
	Logo       *LogoConfig       `json:"logo,omitempty"`
	Frame      *FrameConfig      `json:"frame,omitempty"`
}

type ModulesConfig struct {
	Shape    string          `json:"shape"`
	Color    string          `json:"color"`
	Gradient *GradientConfig `json:"gradient,omitempty"`
}

type GradientConfig struct {
	Type     string         `json:"type"`
	Rotation int            `json:"rotation"`
	Stops    []GradientStop `json:"stops"`
}

type GradientStop struct {
	Offset float64 `json:"offset"`
	Color  string  `json:"color"`
}

type FinderConfig struct {
	OuterShape string `json:"outer_shape"`
	OuterColor string `json:"outer_color"`
	InnerShape string `json:"inner_shape"`
	InnerColor string `json:"inner_color"`
}

type BackgroundConfig struct {
	Color       string `json:"color"`
	Transparent bool   `json:"transparent"`
}

type LogoConfig struct {
	FileID       uuid.UUID `json:"file_id"`
	SizeRatio    float64   `json:"size_ratio"`
	Padding      int       `json:"padding"`
	ClearModules bool      `json:"clear_modules"`
	Shape        string    `json:"shape"`
}

type FrameConfig struct {
	Style     string `json:"style"`
	Text      string `json:"text"`
	TextColor string `json:"text_color"`
	Color     string `json:"color"`
}

// DefaultDesign returns standard black-and-white QR code styling.
func DefaultDesign() DesignV1 {
	return DesignV1{
		V:         1,
		ECC:       "auto",
		QuietZone: 4,
		Modules: ModulesConfig{
			Shape: "square",
			Color: "#111111",
		},
		Finder: FinderConfig{
			OuterShape: "square",
			OuterColor: "#111111",
			InnerShape: "square",
			InnerColor: "#111111",
		},
		Background: BackgroundConfig{
			Color:       "#ffffff",
			Transparent: false,
		},
	}
}

func validateHex(c string) error {
	if !hexColorRegex.MatchString(c) {
		return ErrInvalidHex
	}
	return nil
}

// ValidateDesign validates DesignV1 constraints.
func ValidateDesign(d *DesignV1) error {
	if d.V != 1 {
		return errors.New("design version must be 1")
	}
	switch strings.ToUpper(d.ECC) {
	case "AUTO", "L", "M", "Q", "H":
	default:
		return ErrInvalidECC
	}
	if d.QuietZone < 0 || d.QuietZone > 10 {
		return errors.New("quiet_zone must be between 0 and 10")
	}
	if err := validateHex(d.Modules.Color); err != nil {
		return fmt.Errorf("modules.color: %w", err)
	}
	if err := validateHex(d.Finder.OuterColor); err != nil {
		return fmt.Errorf("finder.outer_color: %w", err)
	}
	if err := validateHex(d.Finder.InnerColor); err != nil {
		return fmt.Errorf("finder.inner_color: %w", err)
	}
	if err := validateHex(d.Background.Color); err != nil {
		return fmt.Errorf("background.color: %w", err)
	}
	if d.Logo != nil {
		if d.Logo.SizeRatio < 0.10 || d.Logo.SizeRatio > 0.30 {
			return errors.New("logo size_ratio must be between 0.10 and 0.30")
		}
	}
	if d.Frame != nil {
		if len(d.Frame.Text) > 24 {
			return errors.New("frame text must be <= 24 characters")
		}
		if err := validateHex(d.Frame.TextColor); err != nil {
			return fmt.Errorf("frame.text_color: %w", err)
		}
		if err := validateHex(d.Frame.Color); err != nil {
			return fmt.Errorf("frame.color: %w", err)
		}
	}
	return nil
}

// CanonicalDesignJSON normalizes hex colors to lowercase and returns deterministic JSON and its SHA-256 hash.
func CanonicalDesignJSON(d *DesignV1) ([]byte, string, error) {
	if err := ValidateDesign(d); err != nil {
		return nil, "", err
	}

	clone := *d
	clone.Modules.Color = strings.ToLower(clone.Modules.Color)
	clone.Finder.OuterColor = strings.ToLower(clone.Finder.OuterColor)
	clone.Finder.InnerColor = strings.ToLower(clone.Finder.InnerColor)
	clone.Background.Color = strings.ToLower(clone.Background.Color)
	if clone.Frame != nil {
		f := *clone.Frame
		f.TextColor = strings.ToLower(f.TextColor)
		f.Color = strings.ToLower(f.Color)
		clone.Frame = &f
	}

	bytes, err := json.Marshal(clone)
	if err != nil {
		return nil, "", err
	}

	h := sha256.Sum256(bytes)
	hashStr := hex.EncodeToString(h[:])
	return bytes, hashStr, nil
}

// EffectiveECC determines ECC: if "auto", uses "H" when logo present, else "M".
func EffectiveECC(d *DesignV1) string {
	ecc := strings.ToUpper(d.ECC)
	if ecc == "AUTO" || ecc == "" {
		if d.Logo != nil {
			return "H"
		}
		return "M"
	}
	return ecc
}

// QRCode represents a QR code record.
type QRCode struct {
	ID               uuid.UUID        `json:"id"`
	WorkspaceID      uuid.UUID        `json:"workspace_id"`
	DomainID         uuid.UUID        `json:"domain_id"`
	ShortCode        string           `json:"short_code"`
	Mode             Mode             `json:"mode"`
	ContentType      ContentType      `json:"content_type"`
	Name             string           `json:"name"`
	Status           Status           `json:"status"`
	SafetyStatus     SafetyStatus     `json:"safety_status"`
	Design           json.RawMessage  `json:"design"`
	DesignHash       string           `json:"design_hash"`
	CurrentVersionID *uuid.UUID       `json:"current_version_id,omitempty"`
	StaticPayload    *string          `json:"static_payload,omitempty"`
	StaticContent    json.RawMessage  `json:"static_content,omitempty"`
	TotalScans       int64            `json:"total_scans"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}
