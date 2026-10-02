package auditstream

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/stdwebhook"
)

// AWS documentation example "GET Object" (Signature Version 4, single chunk).
func TestSignV4KnownVector(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	req.Header.Set("Range", "bytes=0-9")
	now, _ := time.Parse("20060102T150405Z", "20130524T000000Z")
	SignV4(req, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "s3", now)
	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, " +
		"SignedHeaders=host;range;x-amz-content-sha256;x-amz-date, Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("signature:\n got %s\nwant %s", got, want)
	}
}

func events(n int) []Event {
	org := uuid.New()
	out := make([]Event, n)
	for i := range out {
		out[i] = Event{OrgID: org, Seq: int64(i + 1), ID: int64(100 + i), ActorType: "user", Action: "qr.created",
			TargetType: "qr_code", Changes: json.RawMessage(`{}`), CreatedAt: time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC), Hash: "ab"}
	}
	return out
}

func TestSenders(t *testing.T) {
	var got struct {
		path, auth string
		header     http.Header
		body       []byte
	}
	status := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.auth, got.header = r.URL.Path, r.Header.Get("Authorization"), r.Header.Clone()
		got.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
	}))
	defer srv.Close()
	ev := events(3)
	ctx := context.Background()

	secret := stdwebhook.NewSecret()
	wh := Sender{Kind: "webhook", Config: Config{URL: srv.URL + "/hook"}, Secret: secret, HTTP: srv.Client()}
	if err := wh.Send(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if err := stdwebhook.Verify(secret, got.header, got.body, 5*time.Minute, time.Now()); err != nil {
		t.Fatalf("webhook signature: %v", err)
	}
	var batch struct {
		FirstSeq int64   `json:"first_seq"`
		Events   []Event `json:"events"`
	}
	_ = json.Unmarshal(got.body, &batch)
	if batch.FirstSeq != 1 || len(batch.Events) != 3 {
		t.Fatalf("webhook body: %s", got.body)
	}

	sp := Sender{Kind: "splunk_hec", Config: Config{URL: srv.URL, Index: "security"}, Secret: "hec-token", HTTP: srv.Client()}
	if err := sp.Send(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if got.path != "/services/collector/event" || got.auth != "Splunk hec-token" || strings.Count(string(got.body), "\n") != 3 ||
		!strings.Contains(string(got.body), `"index":"security"`) {
		t.Fatalf("splunk: %s %s %s", got.path, got.auth, got.body)
	}

	dd := Sender{Kind: "datadog", Config: Config{Endpoint: srv.URL}, Secret: "dd-key", HTTP: srv.Client()}
	if err := dd.Send(ctx, ev); err != nil {
		t.Fatal(err)
	}
	var logs []map[string]any
	if got.path != "/api/v2/logs" || got.header.Get("DD-API-KEY") != "dd-key" || json.Unmarshal(got.body, &logs) != nil || len(logs) != 3 {
		t.Fatalf("datadog: %s %s", got.path, got.body)
	}

	s3 := Sender{Kind: "s3", Config: Config{Endpoint: srv.URL, Bucket: "audit", Region: "ap-south-1", Prefix: "/qrit/"},
		Secret: "AKID:SECRET", HTTP: srv.Client(), Now: func() time.Time { return time.Date(2026, 9, 26, 10, 5, 0, 0, time.UTC) }}
	if err := s3.Send(ctx, ev); err != nil {
		t.Fatal(err)
	}
	wantPath := "/audit/qrit/2026/09/26/10/" + ev[0].OrgID.String() + "-000000000001-000000000003.jsonl.gz"
	if got.path != wantPath || !strings.HasPrefix(got.auth, "AWS4-HMAC-SHA256 Credential=AKID/20260926/ap-south-1/s3/aws4_request") {
		t.Fatalf("s3: %s %s", got.path, got.auth)
	}
	zr, err := gzip.NewReader(strings.NewReader(string(got.body)))
	if err != nil {
		t.Fatal(err)
	}
	lines, _ := io.ReadAll(zr)
	if strings.Count(string(lines), "\n") != 3 {
		t.Fatalf("s3 object: %s", lines)
	}
	if err := (Sender{Kind: "s3", Config: s3.Config, Secret: "nocolon", HTTP: srv.Client()}).Send(ctx, ev); err == nil {
		t.Fatal("bad credentials accepted")
	}

	status = 503
	if err := wh.Send(ctx, ev); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("non-2xx must fail: %v", err)
	}
}

func TestValidate(t *testing.T) {
	ok := []struct {
		kind string
		c    Config
	}{{"webhook", Config{URL: "https://siem.example/hook"}}, {"datadog", Config{Site: "datadoghq.eu"}}, {"datadog", Config{}},
		{"s3", Config{Bucket: "b", Region: "ap-south-1"}}, {"splunk_hec", Config{URL: "https://splunk.example:8088"}}}
	for _, c := range ok {
		if err := Validate(c.kind, c.c, false); err != nil {
			t.Errorf("%s: %v", c.kind, err)
		}
	}
	bad := []struct {
		kind string
		c    Config
	}{{"webhook", Config{URL: "http://siem.example"}}, {"datadog", Config{Site: "evil.example"}}, {"s3", Config{Bucket: "b"}}, {"ftp", Config{}}}
	for _, c := range bad {
		if err := Validate(c.kind, c.c, false); err == nil {
			t.Errorf("%s %+v accepted", c.kind, c.c)
		}
	}
}
