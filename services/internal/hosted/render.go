package hosted

import (
	"html/template"
	"io"
	"strings"
	"time"
)

var loadLocation = time.LoadLocation

// RenderOptions carries per-request context for the page.
type RenderOptions struct {
	Code      string // short code, used for .vcf/.ics download links
	ReportURL string
}

var pageTmpl = template.Must(template.New("page").Funcs(template.FuncMap{
	"maps": MapsURL,
	"join": func(parts ...string) string {
		var out []string
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				out = append(out, p)
			}
		}
		return strings.Join(out, ", ")
	},
	"deref": func(b *bool) bool { return b != nil && *b },
	"initial": func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" {
			return "•"
		}
		return strings.ToUpper(string([]rune(s)[0]))
	},
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow"><title>{{.Title}}</title>
<style>
:root{--accent:{{.Accent}};--bg:{{.Background}};--text:#0f172a;--muted:#64748b;--card:#fff;--line:#e2e8f0}
@media (prefers-color-scheme:dark){:root{--text:#f1f5f9;--muted:#94a3b8;--card:#111827;--line:#1f2937}{{if not .CustomBG}}body{background:#030712}{{end}}}
*{box-sizing:border-box}body{margin:0;font:15px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;background:var(--bg);color:var(--text)}
main{max-width:480px;margin:0 auto;padding:24px 16px 48px}
.hero{background:var(--accent);border-radius:16px;padding:28px 20px;color:#fff;text-align:center}
.avatar{width:72px;height:72px;border-radius:50%;background:rgba(255,255,255,.2);display:grid;place-items:center;font-size:30px;font-weight:700;margin:0 auto 12px;overflow:hidden}
.avatar img{width:100%;height:100%;object-fit:cover}
h1{font-size:22px;margin:0}.sub{opacity:.9;margin:4px 0 0}
.list{margin-top:16px;display:grid;gap:10px}
a.row,.row{display:flex;gap:12px;align-items:center;padding:14px 16px;background:var(--card);border:1px solid var(--line);border-radius:12px;color:var(--text);text-decoration:none;word-break:break-word}
a.row:hover{border-color:var(--accent)}.row small{display:block;color:var(--muted);font-size:12px}
.btn{display:block;text-align:center;background:var(--accent);color:#fff;padding:14px;border-radius:12px;text-decoration:none;font-weight:600;margin-top:16px}
.section{margin-top:20px}.section h2{font-size:16px;margin:0 0 8px}
.item{display:flex;justify-content:space-between;gap:12px;padding:10px 0;border-bottom:1px solid var(--line)}
.item p{margin:2px 0 0;color:var(--muted);font-size:13px}.price{font-weight:600;white-space:nowrap}
.veg{display:inline-block;width:10px;height:10px;border-radius:2px;margin-right:6px}
footer{text-align:center;color:var(--muted);font-size:12px;margin-top:32px}footer a{color:var(--muted)}
</style></head><body><main>
{{with .Page.VCard}}
<div class="hero"><div class="avatar">{{if .PhotoURL}}<img src="{{.PhotoURL}}" alt="">{{else}}{{initial .FirstName}}{{end}}</div>
<h1>{{.FirstName}} {{.LastName}}</h1>{{if or .Title .Org}}<p class="sub">{{.Title}}{{if and .Title .Org}} · {{end}}{{.Org}}</p>{{end}}</div>
<a class="btn" href="{{$.Code}}.vcf" download>Save contact</a>
<div class="list">
{{range .Phones}}<a class="row" href="tel:{{.Value}}"><span>📞</span><span>{{.Value}}<small>{{.Type}}</small></span></a>{{end}}
{{range .Emails}}<a class="row" href="mailto:{{.Value}}"><span>✉️</span><span>{{.Value}}<small>{{.Type}}</small></span></a>{{end}}
{{if .Website}}<a class="row" href="{{.Website}}" rel="noopener"><span>🌐</span><span>{{.Website}}</span></a>{{end}}
{{with .Address}}<a class="row" href="{{maps (join .Street .City .State .PostalCode .Country)}}" rel="noopener"><span>📍</span><span>{{join .Street .City .State .PostalCode .Country}}</span></a>{{end}}
{{if .Note}}<div class="row"><span>📝</span><span>{{.Note}}</span></div>{{end}}
</div>{{end}}
{{with .Page.LinksPage}}
<div class="hero"><div class="avatar">{{if .AvatarURL}}<img src="{{.AvatarURL}}" alt="">{{else}}{{initial .Title}}{{end}}</div><h1>{{.Title}}</h1>{{if .Bio}}<p class="sub">{{.Bio}}</p>{{end}}</div>
<div class="list">{{range .Links}}<a class="row" href="{{.URL}}" rel="noopener"><span>{{.Label}}</span></a>{{end}}</div>{{end}}
{{with .Page.File}}
<div class="hero"><div class="avatar">📄</div><h1>{{.Title}}</h1></div>
<a class="btn" href="{{.FileURL}}" rel="noopener">{{if .CTA}}{{.CTA}}{{else}}Open file{{end}}</a>{{end}}
{{with .Page.Event}}
<div class="hero"><div class="avatar">📅</div><h1>{{.Title}}</h1><p class="sub">{{$.EventWhen}}</p></div>
<a class="btn" href="{{$.Code}}.ics" download>Add to calendar</a>
<div class="list">{{if .Location}}<a class="row" href="{{maps .Location}}" rel="noopener"><span>📍</span><span>{{.Location}}</span></a>{{end}}
{{if .Description}}<div class="row"><span>{{.Description}}</span></div>{{end}}
{{if .URL}}<a class="row" href="{{.URL}}" rel="noopener"><span>🔗</span><span>Event website</span></a>{{end}}</div>{{end}}
{{with .Page.Menu}}
<div class="hero"><h1>{{.Title}}</h1></div>
{{range .Sections}}<div class="section"><h2>{{.Name}}</h2>{{range .Items}}<div class="item"><div>{{if .Veg}}<span class="veg" style="background:{{if deref .Veg}}#16a34a{{else}}#dc2626{{end}}"></span>{{end}}<strong>{{.Name}}</strong>{{if .Description}}<p>{{.Description}}</p>{{end}}</div>{{if .Price}}<span class="price">{{$.Currency}}{{.Price}}</span>{{end}}</div>{{end}}</div>{{end}}{{end}}
<footer>{{if not .Page.HideBranding}}Made with <a href="https://qrit.io" rel="noopener">QRit</a> · {{end}}<a href="{{.ReportURL}}" rel="nofollow">Report</a></footer>
</main></body></html>`))

// Render writes the HTML page.
func Render(w io.Writer, p *Page, o RenderOptions) error {
	accent, bg := p.Theme.Accent, p.Theme.Background
	if accent == "" {
		accent = "#5B5BF7"
	}
	custom := bg != ""
	if bg == "" {
		bg = "#F8FAFC"
	}
	title := "QRit"
	when := ""
	currency := ""
	switch {
	case p.VCard != nil:
		title = strings.TrimSpace(p.VCard.FirstName + " " + p.VCard.LastName)
	case p.LinksPage != nil:
		title = p.LinksPage.Title
	case p.File != nil:
		title = p.File.Title
	case p.Event != nil:
		title = p.Event.Title
		loc := p.Event.StartsAt.Location()
		if p.Event.Timezone != "" {
			if l, err := loadLocation(p.Event.Timezone); err == nil {
				loc = l
			}
		}
		s, e := p.Event.StartsAt.In(loc), p.Event.EndsAt.In(loc)
		when = s.Format("Mon 2 Jan 2006, 15:04") + " – " + e.Format("15:04 MST")
		if s.YearDay() != e.YearDay() || s.Year() != e.Year() {
			when = s.Format("Mon 2 Jan 2006, 15:04") + " – " + e.Format("Mon 2 Jan 2006, 15:04 MST")
		}
	case p.Menu != nil:
		title = p.Menu.Title
		currency = p.Menu.Currency
	}
	return pageTmpl.Execute(w, map[string]any{
		"Page": p, "Title": title, "Accent": template.CSS(accent), "Background": template.CSS(bg), "CustomBG": custom,
		"Code": o.Code, "ReportURL": o.ReportURL, "EventWhen": when, "Currency": currency,
	})
}
