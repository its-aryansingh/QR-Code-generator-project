// Package hosted defines, validates and renders QRit hosted pages (vCard, links page,
// file, event, menu) that dynamic codes can point at instead of an external URL.
// Pages are rendered by the redirect service on the short-link host, so they work on
// custom domains without a round trip to the dashboard.
package hosted

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Page is a discriminated union on Kind; exactly one payload matches Kind.
type Page struct {
	Kind      string     `json:"kind"`
	Theme     Theme      `json:"theme"`
	VCard     *VCard     `json:"vcard,omitempty"`
	LinksPage *LinksPage `json:"links_page,omitempty"`
	File      *File      `json:"file,omitempty"`
	Event     *Event     `json:"event,omitempty"`
	Menu      *Menu      `json:"menu,omitempty"`
	// HideBranding removes "Made with QRit" (plan feature remove_branding, checked by the API).
	HideBranding bool `json:"hide_branding,omitempty"`
}

type Theme struct {
	Accent     string `json:"accent,omitempty"`
	Background string `json:"background,omitempty"`
}

type Typed struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value"`
}

type Address struct {
	Street     string `json:"street,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country,omitempty"`
}

type VCard struct {
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name,omitempty"`
	Org       string   `json:"org,omitempty"`
	Title     string   `json:"title,omitempty"`
	Phones    []Typed  `json:"phones,omitempty"`
	Emails    []Typed  `json:"emails,omitempty"`
	Website   string   `json:"website,omitempty"`
	Address   *Address `json:"address,omitempty"`
	Note      string   `json:"note,omitempty"`
	PhotoURL  string   `json:"photo_url,omitempty"`
}

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Icon  string `json:"icon,omitempty"`
}

type LinksPage struct {
	Title     string `json:"title"`
	Bio       string `json:"bio,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Links     []Link `json:"links"`
}

type File struct {
	Title    string `json:"title"`
	FileURL  string `json:"file_url"`
	FileType string `json:"file_type,omitempty"` // pdf | image
	CTA      string `json:"cta,omitempty"`
}

type Event struct {
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Timezone    string    `json:"timezone,omitempty"`
	URL         string    `json:"url,omitempty"`
}

type MenuItem struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Price       string `json:"price,omitempty"`
	Veg         *bool  `json:"veg,omitempty"`
}

type MenuSection struct {
	Name  string     `json:"name"`
	Items []MenuItem `json:"items"`
}

type Menu struct {
	Title    string        `json:"title"`
	Currency string        `json:"currency,omitempty"`
	Sections []MenuSection `json:"sections"`
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// URLCheck validates and normalises a URL found inside a page (links, website, file).
type URLCheck func(field, raw string) (string, error)

// Parse decodes and validates a page. check is applied to every outbound URL.
func Parse(raw []byte, check URLCheck) (*Page, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p Page
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("invalid hosted_page: %w", err)
	}
	if err := p.validate(check); err != nil {
		return nil, err
	}
	return &p, nil
}

func limit(field, v string, n int) error {
	if len(v) > n {
		return fmt.Errorf("%s must be at most %d characters", field, n)
	}
	return nil
}

func (p *Page) validate(check URLCheck) error {
	if p.Theme.Accent != "" && !hexColor.MatchString(p.Theme.Accent) {
		return errors.New("theme.accent must be #RRGGBB")
	}
	if p.Theme.Background != "" && !hexColor.MatchString(p.Theme.Background) {
		return errors.New("theme.background must be #RRGGBB")
	}
	payloads := 0
	for _, set := range []bool{p.VCard != nil, p.LinksPage != nil, p.File != nil, p.Event != nil, p.Menu != nil} {
		if set {
			payloads++
		}
	}
	if payloads != 1 {
		return errors.New("hosted_page needs exactly one payload matching kind")
	}
	var err error
	switch p.Kind {
	case "vcard":
		v := p.VCard
		if v == nil {
			return errors.New("kind vcard needs a vcard payload")
		}
		if strings.TrimSpace(v.FirstName+v.LastName+v.Org) == "" {
			return errors.New("vcard needs a name or organisation")
		}
		for _, f := range []struct{ n, v string }{{"vcard.first_name", v.FirstName}, {"vcard.last_name", v.LastName},
			{"vcard.org", v.Org}, {"vcard.title", v.Title}} {
			if err := limit(f.n, f.v, 120); err != nil {
				return err
			}
		}
		if err := limit("vcard.note", v.Note, 1000); err != nil {
			return err
		}
		if len(v.Phones) > 5 || len(v.Emails) > 5 {
			return errors.New("vcard allows at most 5 phones and 5 emails")
		}
		if v.Website != "" {
			if v.Website, err = check("vcard.website", v.Website); err != nil {
				return err
			}
		}
		if v.PhotoURL != "" && !strings.HasPrefix(v.PhotoURL, "https://") {
			return errors.New("vcard.photo_url must be https")
		}
	case "links_page":
		l := p.LinksPage
		if l == nil {
			return errors.New("kind links_page needs a links_page payload")
		}
		if strings.TrimSpace(l.Title) == "" {
			return errors.New("links_page.title is required")
		}
		if err := limit("links_page.bio", l.Bio, 500); err != nil {
			return err
		}
		if len(l.Links) == 0 || len(l.Links) > 30 {
			return errors.New("links_page needs 1–30 links")
		}
		for i := range l.Links {
			if strings.TrimSpace(l.Links[i].Label) == "" {
				return fmt.Errorf("links_page.links[%d].label is required", i)
			}
			if l.Links[i].URL, err = check(fmt.Sprintf("links_page.links[%d].url", i), l.Links[i].URL); err != nil {
				return err
			}
		}
	case "file":
		f := p.File
		if f == nil {
			return errors.New("kind file needs a file payload")
		}
		if f.FileURL == "" {
			return errors.New("file.file_url is required")
		}
		if f.FileURL, err = check("file.file_url", f.FileURL); err != nil {
			return err
		}
		if f.FileType != "" && f.FileType != "pdf" && f.FileType != "image" {
			return errors.New("file.file_type must be pdf or image")
		}
	case "event":
		e := p.Event
		if e == nil {
			return errors.New("kind event needs an event payload")
		}
		if strings.TrimSpace(e.Title) == "" || e.StartsAt.IsZero() || !e.EndsAt.After(e.StartsAt) {
			return errors.New("event needs title, starts_at and ends_at after starts_at")
		}
		if e.Timezone != "" {
			if _, err := time.LoadLocation(e.Timezone); err != nil {
				return errors.New("event.timezone must be an IANA zone")
			}
		}
		if e.URL != "" {
			if e.URL, err = check("event.url", e.URL); err != nil {
				return err
			}
		}
	case "menu":
		m := p.Menu
		if m == nil {
			return errors.New("kind menu needs a menu payload")
		}
		if strings.TrimSpace(m.Title) == "" || len(m.Sections) == 0 || len(m.Sections) > 30 {
			return errors.New("menu needs a title and 1–30 sections")
		}
		for _, s := range m.Sections {
			if len(s.Items) > 100 {
				return errors.New("menu sections allow at most 100 items")
			}
		}
	default:
		return errors.New("hosted_page.kind must be vcard, links_page, file, event or menu")
	}
	return nil
}

// Marshal returns canonical JSON for storage.
func (p *Page) Marshal() []byte {
	b, _ := json.Marshal(p)
	return b
}

func escVCard(s string) string {
	r := strings.NewReplacer(`\`, `\\`, ",", `\,`, ";", `\;`, "\n", `\n`)
	return r.Replace(s)
}

// VCF renders the vCard 3.0 file offered by "Save contact".
func (v *VCard) VCF() string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\r\n") }
	w("BEGIN:VCARD")
	w("VERSION:3.0")
	w("N:" + escVCard(v.LastName) + ";" + escVCard(v.FirstName) + ";;;")
	fn := strings.TrimSpace(v.FirstName + " " + v.LastName)
	if fn == "" {
		fn = v.Org
	}
	w("FN:" + escVCard(fn))
	if v.Org != "" {
		w("ORG:" + escVCard(v.Org))
	}
	if v.Title != "" {
		w("TITLE:" + escVCard(v.Title))
	}
	for _, p := range v.Phones {
		t := strings.ToUpper(p.Type)
		if t == "" {
			t = "CELL"
		}
		w("TEL;TYPE=" + escVCard(t) + ":" + escVCard(p.Value))
	}
	for _, e := range v.Emails {
		t := strings.ToUpper(e.Type)
		if t == "" {
			t = "INTERNET"
		}
		w("EMAIL;TYPE=" + escVCard(t) + ":" + escVCard(e.Value))
	}
	if v.Website != "" {
		w("URL:" + escVCard(v.Website))
	}
	if a := v.Address; a != nil {
		w("ADR;TYPE=WORK:;;" + escVCard(a.Street) + ";" + escVCard(a.City) + ";" + escVCard(a.State) + ";" +
			escVCard(a.PostalCode) + ";" + escVCard(a.Country))
	}
	if v.Note != "" {
		w("NOTE:" + escVCard(v.Note))
	}
	w("END:VCARD")
	return b.String()
}

// ICS renders the calendar file offered by "Add to calendar".
func (e *Event) ICS(uid string) string {
	const f = "20060102T150405Z"
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\r\n") }
	w("BEGIN:VCALENDAR")
	w("VERSION:2.0")
	w("PRODID:-//QRit//Hosted Event//EN")
	w("BEGIN:VEVENT")
	w("UID:" + uid + "@qrit")
	w("DTSTAMP:" + time.Now().UTC().Format(f))
	w("DTSTART:" + e.StartsAt.UTC().Format(f))
	w("DTEND:" + e.EndsAt.UTC().Format(f))
	w("SUMMARY:" + escVCard(e.Title))
	if e.Description != "" {
		w("DESCRIPTION:" + escVCard(e.Description))
	}
	if e.Location != "" {
		w("LOCATION:" + escVCard(e.Location))
	}
	if e.URL != "" {
		w("URL:" + e.URL)
	}
	w("END:VEVENT")
	w("END:VCALENDAR")
	return b.String()
}

// MapsURL links an address/location to a map search.
func MapsURL(q string) string {
	return "https://maps.google.com/?q=" + url.QueryEscape(q)
}
