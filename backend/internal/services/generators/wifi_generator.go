package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// WiFiGenerator generates QR content for WiFi network connection
type WiFiGenerator struct{}

type WiFiMetadata struct {
	SSID       string `json:"ssid"`
	Password   string `json:"password"`
	Encryption string `json:"encryption"` // WPA, WEP, nopass
	Hidden     bool   `json:"hidden"`
}

func (g *WiFiGenerator) ValidateMetadata(metadata json.RawMessage) error {
	var data WiFiMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return err
	}

	if data.SSID == "" {
		return errors.New("SSID (network name) is required")
	}

	// Validate encryption type
	validEncryption := map[string]bool{
		"WPA":    true,
		"WEP":    true,
		"nopass": true,
		"":       true, // Default to WPA
	}
	if !validEncryption[data.Encryption] {
		return errors.New("encryption must be WPA, WEP, or nopass")
	}

	// Password required for encrypted networks
	if data.Encryption != "nopass" && data.Encryption != "" && data.Password == "" {
		return errors.New("password is required for encrypted networks")
	}

	return nil
}

func (g *WiFiGenerator) GenerateContent(metadata json.RawMessage) (string, error) {
	var data WiFiMetadata
	if err := json.Unmarshal(metadata, &data); err != nil {
		return "", err
	}

	// Default encryption to WPA
	encryption := data.Encryption
	if encryption == "" {
		encryption = "WPA"
	}

	// WiFi QR format: WIFI:T:WPA;S:mynetwork;P:mypass;H:true;
	// Escape special characters
	ssid := escapeWiFiString(data.SSID)
	password := escapeWiFiString(data.Password)

	hidden := ""
	if data.Hidden {
		hidden = "H:true;"
	}

	if encryption == "nopass" {
		return fmt.Sprintf("WIFI:T:nopass;S:%s;%s;", ssid, hidden), nil
	}

	return fmt.Sprintf("WIFI:T:%s;S:%s;P:%s;%s;", encryption, ssid, password, hidden), nil
}

// escapeWiFiString escapes special characters in WiFi QR strings
func escapeWiFiString(s string) string {
	// Escape special characters: \, ;, ,, :, "
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		":", "\\:",
		"\"", "\\\"",
	)
	return replacer.Replace(s)
}
