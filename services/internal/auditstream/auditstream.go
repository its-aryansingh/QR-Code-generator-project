// Package auditstream delivers sealed audit entries to customer SIEMs: a signed webhook,
// Splunk HEC, Datadog logs or S3-compatible object storage. Delivery is in seq order and
// at-least-once; receivers dedupe on (org_id, seq).
package auditstream

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/stdwebhook"
)

// Event is one sealed audit entry as streamed.
type Event struct {
	OrgID       uuid.UUID       `json:"org_id"`
	Seq         int64           `json:"seq"`
	ID          int64           `json:"id"`
	WorkspaceID *uuid.UUID      `json:"workspace_id"`
	ActorType   string          `json:"actor_type"`
	ActorID     *uuid.UUID      `json:"actor_id"`
	Action      string          `json:"action"`
	TargetType  string          `json:"target_type"`
	TargetID    *uuid.UUID      `json:"target_id"`
	Changes     json.RawMessage `json:"changes"`
	IPPrefix    *string         `json:"ip_prefix"`
	UserAgent   *string         `json:"user_agent"`
	RequestID   *string         `json:"request_id"`
	CreatedAt   time.Time       `json:"created_at"`
	Hash        string          `json:"hash"`
}

// Config is the non-secret part of a stream (audit_streams.config).
type Config struct {
	URL      string `json:"url,omitempty"`      // webhook, splunk_hec (base URL)
	Site     string `json:"site,omitempty"`     // datadog: datadoghq.com, datadoghq.eu, us3.datadoghq.com …
	Endpoint string `json:"endpoint,omitempty"` // datadog/s3 override (S3-compatible stores, tests)
	Bucket   string `json:"bucket,omitempty"`
	Region   string `json:"region,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	Index    string `json:"index,omitempty"` // splunk index (optional)
}

// Kinds supported by audit_streams.kind.
var Kinds = map[string]bool{"webhook": true, "splunk_hec": true, "datadog": true, "s3": true}

// Validate checks a config for a kind; allowInsecure permits http:// (local development).
func Validate(kind string, c Config, allowInsecure bool) error {
	httpsURL := func(field, raw string) error {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || (u.Scheme != "https" && !(allowInsecure && u.Scheme == "http")) {
			return fmt.Errorf("%s must be an https URL", field)
		}
		return nil
	}
	switch kind {
	case "webhook", "splunk_hec":
		return httpsURL("url", c.URL)
	case "datadog":
		if c.Endpoint != "" {
			return httpsURL("endpoint", c.Endpoint)
		}
		if c.Site != "" && !strings.HasSuffix(c.Site, "datadoghq.com") && !strings.HasSuffix(c.Site, "datadoghq.eu") &&
			c.Site != "ddog-gov.com" {
			return errors.New("site must be a Datadog site such as datadoghq.com or datadoghq.eu")
		}
		return nil
	case "s3":
		if c.Bucket == "" || c.Region == "" {
			return errors.New("bucket and region are required")
		}
		if c.Endpoint != "" {
			return httpsURL("endpoint", c.Endpoint)
		}
		return nil
	}
	return errors.New("kind must be webhook, splunk_hec, datadog or s3")
}

// Sender delivers one batch. A nil error means the receiver accepted it (2xx).
type Sender struct {
	Kind   string
	Config Config
	Secret string // webhook: whsec_…; splunk: HEC token; datadog: API key; s3: "ACCESS_KEY_ID:SECRET"
	HTTP   *http.Client
	Now    func() time.Time
}

func (s Sender) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// DeliveryError carries the receiver's status for diagnostics.
type DeliveryError struct {
	Status int
	Body   string
}

func (e *DeliveryError) Error() string {
	return fmt.Sprintf("receiver answered %d: %s", e.Status, e.Body)
}

func (s Sender) do(ctx context.Context, req *http.Request) error {
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", "QRit-AuditStream/1.0")
	res, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return &DeliveryError{Status: res.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	return nil
}

// Send delivers events (all from one organisation, ascending seq).
func (s Sender) Send(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	switch s.Kind {
	case "webhook":
		body, _ := json.Marshal(map[string]any{"type": "audit.batch", "org_id": events[0].OrgID,
			"first_seq": events[0].Seq, "last_seq": events[len(events)-1].Seq, "events": events})
		req, _ := http.NewRequest(http.MethodPost, s.Config.URL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		msgID := fmt.Sprintf("audit_%s_%d", events[0].OrgID, events[0].Seq)
		if err := stdwebhook.SetHeaders(req.Header, s.Secret, msgID, s.now(), body); err != nil {
			return err
		}
		return s.do(ctx, req)
	case "splunk_hec":
		var buf bytes.Buffer
		for _, e := range events {
			m := map[string]any{"time": float64(e.CreatedAt.UnixMilli()) / 1000, "source": "qrit", "sourcetype": "qrit:audit", "event": e}
			if s.Config.Index != "" {
				m["index"] = s.Config.Index
			}
			b, _ := json.Marshal(m)
			buf.Write(b)
			buf.WriteByte('\n')
		}
		req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(s.Config.URL, "/")+"/services/collector/event", &buf)
		req.Header.Set("Authorization", "Splunk "+s.Secret)
		req.Header.Set("Content-Type", "application/json")
		return s.do(ctx, req)
	case "datadog":
		endpoint := s.Config.Endpoint
		if endpoint == "" {
			site := s.Config.Site
			if site == "" {
				site = "datadoghq.com"
			}
			endpoint = "https://http-intake.logs." + site
		}
		logs := make([]map[string]any, 0, len(events))
		for _, e := range events {
			msg, _ := json.Marshal(e)
			logs = append(logs, map[string]any{"ddsource": "qrit", "service": "qrit-audit", "hostname": "qrit",
				"ddtags": "org_id:" + e.OrgID.String() + ",action:" + e.Action, "message": string(msg)})
		}
		body, _ := json.Marshal(logs)
		req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(endpoint, "/")+"/api/v2/logs", bytes.NewReader(body))
		req.Header.Set("DD-API-KEY", s.Secret)
		req.Header.Set("Content-Type", "application/json")
		return s.do(ctx, req)
	case "s3":
		var gz bytes.Buffer
		zw := gzip.NewWriter(&gz)
		enc := json.NewEncoder(zw)
		for _, e := range events {
			_ = enc.Encode(e)
		}
		_ = zw.Close()
		// One object per batch, named by seq range: retries overwrite the same key.
		t := events[0].CreatedAt.UTC()
		key := strings.Trim(s.Config.Prefix, "/")
		if key != "" {
			key += "/"
		}
		key += fmt.Sprintf("%04d/%02d/%02d/%02d/%s-%012d-%012d.jsonl.gz", t.Year(), t.Month(), t.Day(), t.Hour(),
			events[0].OrgID, events[0].Seq, events[len(events)-1].Seq)
		ak, sk, ok := strings.Cut(s.Secret, ":")
		if !ok {
			return errors.New("s3 credentials must be ACCESS_KEY_ID:SECRET_ACCESS_KEY")
		}
		req, err := PutObjectRequest(s.Config, ak, sk, key, gz.Bytes(), "application/gzip", s.now())
		if err != nil {
			return err
		}
		return s.do(ctx, req)
	}
	return errors.New("unknown stream kind " + s.Kind)
}
