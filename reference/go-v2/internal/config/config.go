package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config is shared by every Go service. Each binary only uses the fields it needs;
// Validate() enforces production requirements so misconfiguration fails at boot.
type Config struct {
	AppEnv   string `env:"APP_ENV" envDefault:"local"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
	HTTPAddr string `env:"HTTP_ADDR" envDefault:":8080"`

	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisURL    string `env:"REDIS_URL" envDefault:"redis://localhost:6379/0"`

	AppBaseURL                string `env:"APP_BASE_URL" envDefault:"http://localhost:3000"`
	APIPublicURL              string `env:"API_PUBLIC_URL" envDefault:"http://localhost:8080"`
	PlatformShortDomain       string `env:"PLATFORM_SHORT_DOMAIN" envDefault:"localhost:8090"`
	PlatformShortDomainScheme string `env:"PLATFORM_SHORT_DOMAIN_SCHEME" envDefault:"http"`
	WebInternalURL            string `env:"WEB_INTERNAL_URL" envDefault:"http://localhost:3000"`
	RenderURL                 string `env:"RENDER_URL" envDefault:"http://localhost:8081"`
	RenderSharedSecret        string `env:"RENDER_SHARED_SECRET" envDefault:"dev-render-secret"`

	JWTEd25519PrivateKey string `env:"JWT_ED25519_PRIVATE_KEY"`
	JWTKeyID             string `env:"JWT_KEY_ID" envDefault:"dev-key-1"`
	AppEncryptionKey     string `env:"APP_ENCRYPTION_KEY"`
	CookieSecure         bool   `env:"COOKIE_SECURE" envDefault:"false"`

	// CORS is only needed when the dashboard calls the API cross-origin; the same-origin
	// Next.js rewrite needs none. Never "*" because cookies are credentials.
	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:","`
	// Client IP headers (CF-Connecting-IP / X-Forwarded-For) are trusted only from these peers.
	// "cloudflare" expands to Cloudflare's published ranges.
	TrustedProxyCIDRs []string `env:"TRUSTED_PROXY_CIDRS" envSeparator:","`

	SMTPAddr    string `env:"SMTP_ADDR"`
	EmailFrom   string `env:"EMAIL_FROM" envDefault:"QRit <noreply@example.com>"`
	WebRiskKey  string `env:"WEB_RISK_API_KEY"`
	LRUSize     int    `env:"LRU_SIZE" envDefault:"100000"`
	LRUTTL      string `env:"LRU_TTL" envDefault:"30s"`
	EventBuffer int    `env:"EVENT_BUFFER_SIZE" envDefault:"100000"`

	// Enterprise
	PolisURL          string `env:"POLIS_URL"`
	PolisExternalURL  string `env:"POLIS_EXTERNAL_URL"`
	PolisAdminAPIKey  string `env:"POLIS_ADMIN_API_KEY"`
	PolisProduct      string `env:"POLIS_PRODUCT" envDefault:"qrit"`
	SerialMACKey      string `env:"SERIAL_MAC_KEY"`
	VerifyTokenKey    string `env:"VERIFY_TOKEN_KEY"`
	DoHURL            string `env:"DOH_URL" envDefault:"https://cloudflare-dns.com/dns-query"`
	SandboxDomain     string `env:"SANDBOX_SHORT_DOMAIN"`
	AppsCNAMETarget   string `env:"APPS_CNAME_TARGET" envDefault:"apps.example.com"`
	TurnstileSecret   string `env:"TURNSTILE_SECRET_KEY"`
	SellerLegalName   string `env:"SELLER_LEGAL_NAME" envDefault:"QRit Technologies"`
	SellerGSTIN       string `env:"SELLER_GSTIN"`
	SellerStateCode   string `env:"SELLER_STATE_CODE" envDefault:"09"`
	SellerAddress     string `env:"SELLER_ADDRESS"`
	SellerSAC         string `env:"SELLER_SAC" envDefault:"998315"`
	SellerLUTRef      string `env:"SELLER_LUT_REF"`
	SellerUPIVPA      string `env:"SELLER_UPI_VPA"`
	WorkerConcurrency int    `env:"WORKER_CONCURRENCY" envDefault:"8"`

	// Razorpay payment links for INR invoices (optional; bank transfer/UPI otherwise).
	RazorpayKeyID         string `env:"RAZORPAY_KEY_ID"`
	RazorpayKeySecret     string `env:"RAZORPAY_KEY_SECRET"`
	RazorpayWebhookSecret string `env:"RAZORPAY_WEBHOOK_SECRET"`
	// Staff console access is limited to these networks when set (CIDRs or IPs).
	StaffIPAllowlist []string `env:"STAFF_IP_ALLOWLIST" envSeparator:","`
}

// Seller returns the supplier block printed on invoices.
func (c *Config) SellerInfo() (legalName, gstin, address, stateCode, sac, lutRef, upiVPA string) {
	return c.SellerLegalName, c.SellerGSTIN, c.SellerAddress, c.SellerStateCode, c.SellerSAC, c.SellerLUTRef, c.SellerUPIVPA
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// IsLocal reports whether the service runs in local development.
func (c *Config) IsLocal() bool { return c.AppEnv == "local" || c.AppEnv == "test" }

// Validate enforces secrets that must never fall back to development defaults outside local.
func (c *Config) Validate() error {
	if c.IsLocal() {
		return nil
	}
	var missing []string
	if c.JWTEd25519PrivateKey == "" {
		missing = append(missing, "JWT_ED25519_PRIVATE_KEY")
	}
	if c.AppEncryptionKey == "" {
		missing = append(missing, "APP_ENCRYPTION_KEY")
	}
	if !c.CookieSecure {
		missing = append(missing, "COOKIE_SECURE=true")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required production configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

// LRUTTLDuration parses LRU_TTL with a safe default.
func (c *Config) LRUTTLDuration() time.Duration {
	d, err := time.ParseDuration(c.LRUTTL)
	if err != nil || d <= 0 {
		return 30 * time.Second
	}
	return d
}

// JWTPrivateKey decodes JWT_ED25519_PRIVATE_KEY. Accepted encodings (base64 std or URL):
// PKCS#8 DER, a 32-byte seed, or a 64-byte Ed25519 private key.
// Local environments without a key get a deterministic development key.
func (c *Config) JWTPrivateKey() (ed25519.PrivateKey, error) {
	raw := strings.TrimSpace(c.JWTEd25519PrivateKey)
	if raw == "" {
		if c.IsLocal() {
			// Stable across restarts so local sessions survive a rebuild; never used outside local.
			master, _ := c.EncryptionKey()
			return ed25519.NewKeyFromSeed(deriveKey(master, "jwt-local")), nil
		}
		return nil, errors.New("JWT_ED25519_PRIVATE_KEY is required outside local")
	}
	b, err := decodeB64(raw)
	if err != nil {
		return nil, fmt.Errorf("decode JWT_ED25519_PRIVATE_KEY: %w", err)
	}
	switch len(b) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(b), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(b), nil
	}
	k, err := x509.ParsePKCS8PrivateKey(b)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8 key: %w", err)
	}
	ek, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("JWT key is not Ed25519")
	}
	return ek, nil
}

// EncryptionKey returns the 32-byte master key. Local environments get a fixed development key.
func (c *Config) EncryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(c.AppEncryptionKey)
	if raw == "" {
		if c.IsLocal() {
			return []byte("local-development-key-32-bytes!!"), nil
		}
		return nil, errors.New("APP_ENCRYPTION_KEY is required outside local")
	}
	b, err := decodeB64(raw)
	if err != nil {
		return nil, fmt.Errorf("decode APP_ENCRYPTION_KEY: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("APP_ENCRYPTION_KEY must decode to 32 bytes, got %d", len(b))
	}
	return b, nil
}

// SecretOrDerived returns a base64 secret from env, or a key derived from the master key
// (domain-separated) so optional secrets never silently use a shared constant.
func (c *Config) SecretOrDerived(value, label string) ([]byte, error) {
	if strings.TrimSpace(value) != "" {
		return decodeB64(strings.TrimSpace(value))
	}
	master, err := c.EncryptionKey()
	if err != nil {
		return nil, err
	}
	return deriveKey(master, label), nil
}

// TrustedProxies parses TRUSTED_PROXY_CIDRS.
func (c *Config) TrustedProxies() ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, s := range c.TrustedProxyCIDRs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if s == "cloudflare" {
			for _, cidr := range CloudflareRanges {
				_, n, _ := net.ParseCIDR(cidr)
				out = append(out, n)
			}
			continue
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS entry %q: %w", s, err)
		}
		out = append(out, n)
	}
	return out, nil
}

func decodeB64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("not valid base64")
}

// CloudflareRanges are Cloudflare's published edge ranges (https://www.cloudflare.com/ips/).
var CloudflareRanges = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22", "141.101.64.0/18",
	"108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22", "198.41.128.0/17",
	"162.158.0.0/15", "104.16.0.0/13", "104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32", "2405:8100::/32",
	"2a06:98c0::/29", "2c0f:f248::/32",
}
