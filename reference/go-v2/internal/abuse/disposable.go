package abuse

import (
	"strings"
)

// Known disposable / throwaway email provider domains
var disposableDomains = map[string]struct{}{
	"10minutemail.com":       {},
	"10minutemail.net":       {},
	"tempmail.com":           {},
	"temp-mail.org":          {},
	"mailinator.com":         {},
	"guerrillamail.com":      {},
	"guerrillamail.block":    {},
	"guerrillamail.net":      {},
	"guerrillamail.org":      {},
	"sharklasers.com":        {},
	"grr.la":                 {},
	"trashmail.com":          {},
	"trashmail.net":          {},
	"trashmail.org":          {},
	"yopmail.com":            {},
	"yopmail.fr":             {},
	"yopmail.net":            {},
	"getnada.com":            {},
	"nada.ltd":               {},
	"inboxbear.com":          {},
	"dispostable.com":        {},
	"dropmail.me":            {},
	"fakeinbox.com":          {},
	"fakemailgenerator.com":  {},
	"mohmal.com":             {},
	"burnermail.io":          {},
	"crazymailing.com":       {},
	"throwawaymail.com":      {},
	"generator.email":        {},
	"emailondeck.com":        {},
	"maildrop.cc":            {},
	"mytemp.email":           {},
	"mytempmail.com":         {},
	"tempail.com":            {},
	"internxt.com/temp-mail": {},
}

// IsDisposable checks if an email or domain belongs to a known temporary/throwaway email service.
func IsDisposable(emailOrDomain string) bool {
	clean := strings.ToLower(strings.TrimSpace(emailOrDomain))
	if clean == "" {
		return false
	}

	// Extract domain if full email address was supplied
	domain := clean
	if atIdx := strings.LastIndex(clean, "@"); atIdx != -1 {
		domain = clean[atIdx+1:]
	}

	// Direct match
	if _, ok := disposableDomains[domain]; ok {
		return true
	}

	// Check parent domain suffixes (e.g. sub.trashmail.com)
	parts := strings.Split(domain, ".")
	if len(parts) > 2 {
		parent := strings.Join(parts[len(parts)-2:], ".")
		if _, ok := disposableDomains[parent]; ok {
			return true
		}
	}

	return false
}
