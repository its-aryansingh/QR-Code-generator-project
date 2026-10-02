package hosted

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func okURL(_, raw string) (string, error) {
	if !strings.HasPrefix(raw, "https://") {
		return "", errors.New("https only")
	}
	return raw, nil
}

func TestParseAndRender(t *testing.T) {
	cases := map[string]string{
		"vcard": `{"kind":"vcard","theme":{"accent":"#112233"},"vcard":{"first_name":"Asha","last_name":"Rao","org":"Acme; Ltd","phones":[{"type":"cell","value":"+911234567890"}],"website":"https://acme.example","address":{"city":"Pune","country":"IN"}}}`,
		"links": `{"kind":"links_page","links_page":{"title":"Asha","links":[{"label":"Blog","url":"https://blog.example"}]}}`,
		"file":  `{"kind":"file","file":{"title":"Menu","file_url":"https://cdn.example/menu.pdf","file_type":"pdf"}}`,
		"event": `{"kind":"event","event":{"title":"Launch","starts_at":"2026-10-01T10:00:00Z","ends_at":"2026-10-01T12:00:00Z","timezone":"Asia/Kolkata","location":"Noida"}}`,
		"menu":  `{"kind":"menu","menu":{"title":"Cafe","currency":"₹","sections":[{"name":"Coffee","items":[{"name":"Latte","price":"180","veg":true}]}]}}`,
	}
	for name, raw := range cases {
		p, err := Parse([]byte(raw), okURL)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var buf bytes.Buffer
		if err := Render(&buf, p, RenderOptions{Code: "ABCDEFG", ReportURL: "https://app.example/report"}); err != nil {
			t.Fatalf("%s render: %v", name, err)
		}
		if !strings.Contains(buf.String(), "Report") {
			t.Fatalf("%s: missing footer", name)
		}
	}
	p, _ := Parse([]byte(cases["vcard"]), okURL)
	vcf := p.VCard.VCF()
	if !strings.Contains(vcf, `ORG:Acme\; Ltd`) || !strings.Contains(vcf, "TEL;TYPE=CELL:+911234567890") {
		t.Fatalf("vcf: %s", vcf)
	}
	e, _ := Parse([]byte(cases["event"]), okURL)
	if ics := e.Event.ICS("x"); !strings.Contains(ics, "DTSTART:20261001T100000Z") {
		t.Fatalf("ics: %s", ics)
	}
	bad := []string{
		`{"kind":"vcard","vcard":{"first_name":""}}`,
		`{"kind":"links_page","links_page":{"title":"x","links":[{"label":"a","url":"http://insecure"}]}}`,
		`{"kind":"vcard","vcard":{"first_name":"a"},"file":{"title":"x","file_url":"https://x"}}`,
		`{"kind":"event","event":{"title":"x","starts_at":"2026-10-01T10:00:00Z","ends_at":"2026-10-01T09:00:00Z"}}`,
		`{"kind":"nope"}`,
		`{"kind":"vcard","vcard":{"first_name":"a"},"extra":1}`,
		`{"kind":"vcard","theme":{"accent":"red"},"vcard":{"first_name":"a"}}`,
	}
	for _, b := range bad {
		if _, err := Parse([]byte(b), okURL); err == nil {
			t.Fatalf("accepted %s", b)
		}
	}
}

func TestRenderEscapes(t *testing.T) {
	p, err := Parse([]byte(`{"kind":"links_page","links_page":{"title":"<script>alert(1)</script>","links":[{"label":"x","url":"https://a.example"}]}}`), okURL)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_ = Render(&buf, p, RenderOptions{})
	if strings.Contains(buf.String(), "<script>alert") {
		t.Fatal("not escaped")
	}
}
