package redirect

import (
	"html/template"
	"net/http"
	"net/url"
)

// Self-contained, mobile-first pages (inline CSS, no JS, light/dark).
var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow"><title>{{.Title}}</title>
<style>
:root{--bg:#f8fafc;--card:#fff;--text:#0f172a;--muted:#64748b;--line:#e2e8f0;--accent:#4f46e5}
@media (prefers-color-scheme:dark){:root{--bg:#030712;--card:#111827;--text:#f1f5f9;--muted:#94a3b8;--line:#1f2937;--accent:#818cf8}}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;padding:16px;background:var(--bg);color:var(--text);font:15px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.card{width:100%;max-width:400px;background:var(--card);border:1px solid var(--line);border-radius:16px;padding:28px 24px;text-align:center}
h1{font-size:20px;margin:0 0 8px}p{margin:0 0 16px;color:var(--muted)}
input{width:100%;padding:12px;border:1px solid var(--line);border-radius:10px;background:var(--bg);color:var(--text);font-size:16px;margin-bottom:12px}
button,.btn{display:block;width:100%;padding:12px;border:0;border-radius:10px;background:var(--accent);color:#fff;font-size:15px;font-weight:600;text-decoration:none;cursor:pointer}
.err{color:#dc2626;font-size:14px;margin-bottom:12px}.host{font-weight:600;word-break:break-all;color:var(--text)}
small{display:block;margin-top:18px;color:var(--muted)}small a{color:var(--muted)}
</style></head><body><main class="card">
{{if .Password}}<h1>This QR code is protected</h1><p>Enter the password to continue.</p>
{{if .Error}}<div class="err" role="alert">{{.Error}}</div>{{end}}
<form method="post" autocomplete="off"><input type="password" name="password" required autofocus aria-label="Password"><button type="submit">Continue</button></form>
{{else if .Preview}}<h1>Where does this code go?</h1><p>This QR code currently opens</p><p class="host">{{.Host}}</p>
{{if .Target}}<a class="btn" href="{{.Target}}" rel="nofollow noopener">Continue to {{.Host}}</a>{{end}}
{{else}}<h1>{{.Title}}</h1><p>{{.Message}}</p>{{end}}
<small>Created with QRit{{if .ReportURL}} · <a href="{{.ReportURL}}" rel="nofollow">Report this code</a>{{end}}</small>
</main></body></html>`))

type pageData struct {
	Title, Message    string
	Password, Preview bool
	Error             string
	Host, Target      string
	ReportURL         string
}

func (s *Server) reportURL(r *http.Request) string {
	return s.appBaseURL + "/report?u=" + url.QueryEscape("https://"+r.Host+r.URL.Path)
}

func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, status int, d pageData) {
	if d.ReportURL == "" {
		d.ReportURL = s.reportURL(r)
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "private, no-store, max-age=0")
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src https: data:; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = pageTmpl.Execute(w, d)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request, dom *Domain) {
	if dom != nil && dom.NotFoundURL != nil && *dom.NotFoundURL != "" {
		s.redirectTo(w, r, *dom.NotFoundURL)
		return
	}
	s.renderPage(w, r, http.StatusNotFound, pageData{Title: "QR code not found",
		Message: "This code doesn't exist or was removed by its owner."})
}
