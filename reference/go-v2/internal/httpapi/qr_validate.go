package httpapi

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/hosted"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/qr"
	"github.com/its-aryansingh/qrit/services/internal/routing"
	"github.com/its-aryansingh/qrit/services/internal/urlsafety"
	"github.com/its-aryansingh/qrit/services/internal/version"
)

// opt distinguishes an absent JSON field (Set=false) from an explicit null (Set, Null).
type opt[T any] struct {
	Set   bool
	Null  bool
	Value T
}

func (o *opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

func (o opt[T]) ptr() *T {
	if !o.Set || o.Null {
		return nil
	}
	v := o.Value
	return &v
}

const (
	maxRules          = 20
	maxHostedPageSize = 64 << 10
	maxStaticPayload  = 2953 // QR version 40-L binary capacity
	safetyTimeout     = 3 * time.Second
)

// wsSettings are legacy per-workspace settings (JSON); governance lives in workspace_policies.
type wsSettings struct {
	AllowShorteners bool `json:"allow_shorteners"`
}

func settingsOf(ws dbgen.Workspace) wsSettings {
	var st wsSettings
	if len(ws.Settings) > 0 {
		_ = json.Unmarshal(ws.Settings, &st)
	}
	return st
}

// urlPolicy combines platform rules with the workspace policy. With approval mode
// "outside_allowlist" the allowlist routes changes to approval instead of refusing them.
func (s *Server) urlPolicy(ctx context.Context, ws dbgen.Workspace) urlsafety.Policy {
	own := []string{strings.ToLower(s.cfg.PlatformShortDomain)}
	if u, err := url.Parse(s.cfg.AppBaseURL); err == nil && u.Hostname() != "" {
		own = append(own, strings.ToLower(u.Hostname()))
	}
	p := urlsafety.Policy{OwnHosts: own, AllowShorteners: settingsOf(ws).AllowShorteners}
	if wp, err := s.wsPolicy(ctx, ws.ID); err == nil {
		p.RequireHTTPS = wp.RequireHTTPS
		p.BlockedHosts = wp.BlockedDestinationHosts
		if wp.ApprovalMode != "outside_allowlist" {
			p.AllowedHosts = wp.AllowedDestinationHosts
		}
	}
	return p
}

func validationProblem(field string, err error) *apierr.ProblemDetails {
	var ve *urlsafety.ValidationError
	if errors.As(err, &ve) {
		return unprocessable(ve.Code, field+": "+ve.Message, apierr.FieldError{Field: field, Code: ve.Code, Message: ve.Message})
	}
	return unprocessable("invalid_"+strings.ReplaceAll(field, ".", "_"), field+": "+err.Error(),
		apierr.FieldError{Field: field, Code: "invalid", Message: err.Error()})
}

// checkDestination validates syntax and policy, then asks the reputation service.
// Returns the normalised URL and a safety status (safe|pending). Unsafe URLs are rejected.
func (s *Server) checkDestination(ctx context.Context, ws dbgen.Workspace, field, raw string) (string, string, error) {
	norm, err := urlsafety.Validate(raw, s.urlPolicy(ctx, ws))
	if err != nil {
		return "", "", validationProblem(field, err)
	}
	if !strings.HasPrefix(strings.ToLower(norm), "http") {
		return norm, "safe", nil // mailto:, tel:, sms:, geo:, upi: have no reputation lookup
	}
	cctx, cancel := context.WithTimeout(ctx, safetyTimeout)
	defer cancel()
	verdict, err := s.safety.Check(cctx, norm)
	switch {
	case err != nil:
		return norm, "pending", nil // worker rescans pending destinations
	case verdict == urlsafety.VerdictUnsafe:
		return "", "", unprocessable("destination_unsafe", field+": this destination is flagged as malicious by our URL reputation service",
			apierr.FieldError{Field: field, Code: "destination_unsafe", Message: "flagged as unsafe"})
	case verdict == urlsafety.VerdictSafe:
		return norm, "safe", nil
	default:
		return norm, "pending", nil
	}
}

// combineSafety returns the least-safe of statuses (pending beats safe).
func combineSafety(a, b string) string {
	if a == "pending" || b == "pending" {
		return "pending"
	}
	return "safe"
}

var ruleFields = map[string]bool{"country": true, "region": true, "device_type": true, "os": true,
	"language": true, "local_time": true, "weekday": true, "date": true, "scan_count": true}

// normaliseRules validates routing rules and every destination inside them.
func (s *Server) normaliseRules(ctx context.Context, ws dbgen.Workspace, raw json.RawMessage) (json.RawMessage, string, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return json.RawMessage("[]"), "safe", nil
	}
	var rules []routing.Rule
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rules); err != nil {
		return nil, "", unprocessable("invalid_rules", "rules must be an array of rule objects: "+err.Error())
	}
	if len(rules) > maxRules {
		return nil, "", unprocessable("too_many_rules", fmt.Sprintf("at most %d rules are allowed", maxRules))
	}
	safety := "safe"
	seen := map[string]bool{}
	for i := range rules {
		rl := &rules[i]
		field := fmt.Sprintf("rules[%d]", i)
		if rl.ID == "" {
			rl.ID = "r" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
		}
		if seen[rl.ID] {
			return nil, "", unprocessable("duplicate_rule_id", field+": duplicate rule id "+rl.ID)
		}
		seen[rl.ID] = true
		if rl.When != nil {
			if len(rl.When.All)+len(rl.When.Any) > 10 {
				return nil, "", unprocessable("invalid_rules", field+": at most 10 conditions per rule")
			}
			if len(rl.When.All) > 0 && len(rl.When.Any) > 0 {
				return nil, "", unprocessable("invalid_rules", field+".when: use either all or any, not both")
			}
			for _, c := range append(append([]routing.Condition{}, rl.When.All...), rl.When.Any...) {
				if !ruleFields[c.Field] {
					return nil, "", unprocessable("invalid_rules", field+": unsupported condition field "+c.Field)
				}
				switch c.Op {
				case "in", "not_in", "between", "gte", "lte", "gt", "lt", "eq":
				default:
					return nil, "", unprocessable("invalid_rules", field+": unsupported operator "+c.Op)
				}
			}
		}
		switch {
		case rl.Block:
			if rl.DestinationURL != "" || len(rl.Split) > 0 {
				return nil, "", unprocessable("invalid_rules", field+": a block rule cannot have a destination")
			}
			if rl.When == nil || len(rl.When.All)+len(rl.When.Any) == 0 {
				return nil, "", unprocessable("invalid_rules", field+": a block rule needs conditions")
			}
		case len(rl.Split) > 0:
			if len(rl.Split) > 5 {
				return nil, "", unprocessable("invalid_rules", field+": at most 5 split variants")
			}
			if rl.DestinationURL != "" {
				return nil, "", unprocessable("invalid_rules", field+": a split rule cannot also set destination_url")
			}
			total := 0
			for j := range rl.Split {
				v := &rl.Split[j]
				if v.Weight < 0 || v.Weight > 100 {
					return nil, "", unprocessable("invalid_rules", field+": split weights must be 0–100")
				}
				total += v.Weight
				norm, st, err := s.checkDestination(ctx, ws, fmt.Sprintf("%s.split[%d].destination_url", field, j), v.DestinationURL)
				if err != nil {
					return nil, "", err
				}
				v.DestinationURL, safety = norm, combineSafety(safety, st)
				if v.Variant == "" {
					v.Variant = string(rune('A' + j))
				}
			}
			if total != 100 {
				return nil, "", unprocessable("invalid_rules", field+": split weights must add up to 100")
			}
		case rl.DestinationURL != "":
			norm, st, err := s.checkDestination(ctx, ws, field+".destination_url", rl.DestinationURL)
			if err != nil {
				return nil, "", err
			}
			rl.DestinationURL, safety = norm, combineSafety(safety, st)
		default:
			return nil, "", unprocessable("invalid_rules", field+": a rule needs destination_url or split")
		}
	}
	out, _ := json.Marshal(rules)
	return out, safety, nil
}

func normaliseUTM(raw json.RawMessage) (json.RawMessage, bool, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return json.RawMessage("{}"), false, nil
	}
	var u version.UTMConfig
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&u); err != nil {
		return nil, false, unprocessable("invalid_utm", "utm accepts source, medium, campaign, term and content: "+err.Error())
	}
	for _, v := range []string{u.Source, u.Medium, u.Campaign, u.Term, u.Content} {
		if len(v) > 200 {
			return nil, false, unprocessable("invalid_utm", "utm values must be at most 200 characters")
		}
	}
	out, _ := json.Marshal(u)
	used := u.Source != "" || u.Medium != "" || u.Campaign != "" || u.Term != "" || u.Content != ""
	return out, used, nil
}

// normaliseHostedPage validates a hosted page and every URL inside it.
func (s *Server) normaliseHostedPage(ctx context.Context, ws dbgen.Workspace, raw json.RawMessage) ([]byte, string, error) {
	if len(raw) > maxHostedPageSize {
		return nil, "", unprocessable("hosted_page_too_large", "hosted_page must be at most 64 KiB")
	}
	safety := "safe"
	page, err := hosted.Parse(raw, func(field, u string) (string, error) {
		norm, st, err := s.checkDestination(ctx, ws, "hosted_page."+field, u)
		if err != nil {
			return "", err
		}
		safety = combineSafety(safety, st)
		return norm, nil
	})
	if err != nil {
		var pd *apierr.ProblemDetails
		if errors.As(err, &pd) {
			return nil, "", pd
		}
		return nil, "", unprocessable("invalid_hosted_page", err.Error())
	}
	if page.HideBranding {
		if err := s.ent().CheckFeature(ctx, ws, entitlements.FeatureRemoveBranding); err != nil {
			return nil, "", featureErr(err)
		}
	}
	return page.Marshal(), safety, nil
}

// staticContent is the structured form input for a static code.
type staticContent struct {
	URL      string   `json:"url"`
	Text     string   `json:"text"`
	To       string   `json:"to"`
	Subject  string   `json:"subject"`
	Body     string   `json:"body"`
	Phone    string   `json:"phone"`
	Message  string   `json:"message"`
	SSID     string   `json:"ssid"`
	Password string   `json:"password"`
	Auth     string   `json:"auth"`
	Hidden   bool     `json:"hidden"`
	VPA      string   `json:"vpa"`
	Name     string   `json:"name"`
	Amount   *float64 `json:"amount"`
	Note     string   `json:"note"`
	Lat      *float64 `json:"lat"`
	Lng      *float64 `json:"lng"`

	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Company    string `json:"company"`
	Title      string `json:"title"`
	Email      string `json:"email"`
	Website    string `json:"website"`
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`

	Description string     `json:"description"`
	Location    string     `json:"location"`
	Start       *time.Time `json:"start"`
	End         *time.Time `json:"end"`
}

// encodeStatic builds the exact payload of a static code from structured content.
func (s *Server) encodeStatic(ctx context.Context, ws dbgen.Workspace, contentType string, raw json.RawMessage, payload string) (string, []byte, string, error) {
	safety := "safe"
	if len(bytes.TrimSpace(raw)) == 0 {
		if payload == "" {
			return "", nil, "", unprocessable("static_content_required", "static codes need static_content (or static_payload)")
		}
		if len(payload) > maxStaticPayload {
			return "", nil, "", unprocessable("payload_too_large", "static payload exceeds QR capacity (2953 bytes)")
		}
		if contentType == "url" {
			norm, st, err := s.checkDestination(ctx, ws, "static_payload", payload)
			if err != nil {
				return "", nil, "", err
			}
			payload, safety = norm, st
		}
		return payload, nil, safety, nil
	}
	var c staticContent
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", nil, "", unprocessable("invalid_static_content", "static_content must be an object: "+err.Error())
	}
	var out string
	var err error
	switch contentType {
	case "url":
		out, safety, err = s.checkDestination(ctx, ws, "static_content.url", c.URL)
	case "text":
		out, err = qr.EncodeText(c.Text)
		if err == nil && out == "" {
			err = errors.New("text is required")
		}
	case "email":
		if _, ok := validEmail(c.To); !ok {
			err = errors.New("to must be a valid email address")
		} else {
			out = qr.EncodeEmail(c.To, c.Subject, c.Body)
		}
	case "phone":
		if !validPhone(c.Phone) {
			err = errors.New("phone must be in E.164 format, e.g. +919876543210")
		} else {
			out = qr.EncodePhone(c.Phone)
		}
	case "sms":
		if !validPhone(c.Phone) {
			err = errors.New("phone must be in E.164 format")
		} else {
			out = qr.EncodeSMS(c.Phone, c.Message)
		}
	case "whatsapp":
		if !validPhone(c.Phone) {
			err = errors.New("phone must be in E.164 format")
		} else {
			out = qr.EncodeWhatsApp(c.Phone, c.Message)
		}
	case "wifi":
		if c.SSID == "" || len(c.SSID) > 32 {
			err = errors.New("ssid must be 1–32 characters")
		} else {
			out = qr.EncodeWiFi(c.SSID, c.Password, c.Auth, c.Hidden)
		}
	case "vcard":
		if c.FirstName == "" && c.LastName == "" && c.Company == "" {
			err = errors.New("a name or company is required")
		} else {
			out = qr.EncodeVCard(qr.VCardData{FirstName: c.FirstName, LastName: c.LastName, Company: c.Company, Title: c.Title,
				Phone: c.Phone, Email: c.Email, Website: c.Website, Street: c.Street, City: c.City, State: c.State,
				PostalCode: c.PostalCode, Country: c.Country, Note: c.Note})
		}
	case "event":
		if c.Title == "" || c.Start == nil || c.End == nil || !c.End.After(*c.Start) {
			err = errors.New("title, start and end (after start) are required")
		} else {
			out = qr.EncodeEvent(qr.EventData{Title: c.Title, Description: c.Description, Location: c.Location, Start: *c.Start, End: *c.End})
		}
	case "upi":
		out, err = qr.EncodeUPI(c.VPA, c.Name, c.Amount, c.Note)
	case "location":
		if c.Lat == nil || c.Lng == nil {
			err = errors.New("lat and lng are required")
		} else {
			out, err = qr.EncodeLocation(*c.Lat, *c.Lng)
		}
	default:
		err = fmt.Errorf("content type %q cannot be static", contentType)
	}
	if err != nil {
		var pd *apierr.ProblemDetails
		if errors.As(err, &pd) {
			return "", nil, "", pd
		}
		return "", nil, "", unprocessable("invalid_static_content", "static_content: "+err.Error())
	}
	if len(out) > maxStaticPayload {
		return "", nil, "", unprocessable("payload_too_large", "encoded content exceeds QR capacity (2953 bytes)")
	}
	canon, _ := json.Marshal(c)
	return out, canon, safety, nil
}

func validPhone(p string) bool {
	p = strings.TrimSpace(p)
	if len(p) < 8 || len(p) > 16 || p[0] != '+' {
		return false
	}
	for _, r := range p[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// resolveDesign applies template rules: a locked template is used verbatim unless the caller
// holds qr.design.bypass_lock. Returns canonical JSON and the sha256 hash bytes.
func (s *Server) resolveDesign(ctx context.Context, ws dbgen.Workspace, templateID *uuid.UUID, in *qr.DesignV1, bypassLock bool) ([]byte, []byte, *uuid.UUID, error) {
	var tpl *dbgen.Template
	if templateID != nil {
		t, err := s.q.GetTemplate(ctx, dbgen.GetTemplateParams{ID: *templateID, WorkspaceID: ws.ID})
		if err != nil {
			return nil, nil, nil, unprocessable("invalid_template", "template not found in this workspace")
		}
		tpl = &t
	} else if t, err := s.q.GetDefaultTemplate(ctx, ws.ID); err == nil {
		tpl = &t
	}
	design := qr.DefaultDesign()
	var tplID *uuid.UUID
	if tpl != nil {
		var td qr.DesignV1
		if err := json.Unmarshal(tpl.Design, &td); err == nil {
			design = td
		}
		id := tpl.ID
		tplID = &id
	}
	if in != nil {
		if tpl != nil && tpl.IsLocked && !bypassLock {
			return nil, nil, nil, forbidden("template_locked", "this workspace's brand template is locked; the design cannot be changed")
		}
		design = *in
	}
	canon, hashHex, err := qr.CanonicalDesignJSON(&design)
	if err != nil {
		return nil, nil, nil, unprocessable("invalid_design", "design: "+err.Error())
	}
	h, _ := hex.DecodeString(hashHex)
	return canon, h, tplID, nil
}

// problemOr500 passes problems through and hides anything else.
func problemOr500(err error, msg string) error {
	var pd *apierr.ProblemDetails
	if errors.As(err, &pd) {
		return pd
	}
	return apierr.New(http.StatusInternalServerError, "internal_error", "Internal Server Error", msg)
}

// dynamicContactURL turns structured contact content into a redirect destination for
// dynamic email/phone/sms/whatsapp codes.
func dynamicContactURL(contentType string, raw json.RawMessage) (string, error) {
	var c staticContent
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", unprocessable("invalid_static_content", "static_content must be an object")
	}
	switch contentType {
	case "email":
		if _, ok := validEmail(c.To); !ok {
			return "", unprocessable("invalid_static_content", "to must be a valid email address")
		}
		return qr.EncodeEmail(c.To, c.Subject, c.Body), nil
	case "phone", "sms", "whatsapp":
		if !validPhone(c.Phone) {
			return "", unprocessable("invalid_static_content", "phone must be in E.164 format, e.g. +919876543210")
		}
		switch contentType {
		case "phone":
			return qr.EncodePhone(c.Phone), nil
		case "sms":
			u := "sms:" + strings.TrimSpace(c.Phone)
			if c.Message != "" {
				u += "?body=" + url.QueryEscape(c.Message)
			}
			return u, nil
		default:
			return qr.EncodeWhatsApp(c.Phone, c.Message), nil
		}
	}
	return "", nil
}
