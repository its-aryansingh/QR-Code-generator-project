package qr_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/qr"
)

func TestDefaultDesign(t *testing.T) {
	d := qr.DefaultDesign()
	if err := qr.ValidateDesign(&d); err != nil {
		t.Fatalf("DefaultDesign failed validation: %v", err)
	}

	bytes, hash, err := qr.CanonicalDesignJSON(&d)
	if err != nil || len(bytes) == 0 || hash == "" {
		t.Fatalf("CanonicalDesignJSON failed: %v", err)
	}
}

func TestCanonicalDesignDeterminism(t *testing.T) {
	d1 := qr.DefaultDesign()
	d1.Modules.Color = "#AA00FF"

	d2 := qr.DefaultDesign()
	d2.Modules.Color = "#aa00ff"

	_, hash1, err1 := qr.CanonicalDesignJSON(&d1)
	_, hash2, err2 := qr.CanonicalDesignJSON(&d2)

	if err1 != nil || err2 != nil {
		t.Fatalf("CanonicalDesignJSON failed: %v, %v", err1, err2)
	}

	if hash1 != hash2 {
		t.Errorf("CanonicalDesignJSON should normalize hex colors: hash1 %q != hash2 %q", hash1, hash2)
	}
}

func TestEffectiveECC(t *testing.T) {
	d := qr.DefaultDesign()
	if qr.EffectiveECC(&d) != "M" {
		t.Errorf("expected M for auto without logo, got %s", qr.EffectiveECC(&d))
	}

	d.Logo = &qr.LogoConfig{
		FileID:    uuid.New(),
		SizeRatio: 0.20,
	}
	if qr.EffectiveECC(&d) != "H" {
		t.Errorf("expected H for auto with logo, got %s", qr.EffectiveECC(&d))
	}

	d.ECC = "Q"
	if qr.EffectiveECC(&d) != "Q" {
		t.Errorf("expected explicit Q, got %s", qr.EffectiveECC(&d))
	}
}

func TestDesignValidationErrors(t *testing.T) {
	d := qr.DefaultDesign()
	d.ECC = "INVALID"
	if err := qr.ValidateDesign(&d); err == nil {
		t.Errorf("expected error for invalid ECC")
	}

	d = qr.DefaultDesign()
	d.QuietZone = 15
	if err := qr.ValidateDesign(&d); err == nil {
		t.Errorf("expected error for quiet_zone > 10")
	}

	d = qr.DefaultDesign()
	d.Modules.Color = "not-a-hex"
	if err := qr.ValidateDesign(&d); err == nil {
		t.Errorf("expected error for invalid hex color")
	}

	d = qr.DefaultDesign()
	d.Logo = &qr.LogoConfig{
		FileID:    uuid.New(),
		SizeRatio: 0.50, // > 0.30
	}
	if err := qr.ValidateDesign(&d); err == nil {
		t.Errorf("expected error for logo size_ratio > 0.30")
	}
}
