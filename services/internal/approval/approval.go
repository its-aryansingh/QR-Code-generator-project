// Package approval decides when a change needs a second person's sign-off (maker–checker)
// and describes the reasons in plain language. It is pure: storage and notification live in
// the API layer.
package approval

import (
	"net/url"
	"sort"
	"strings"

	"github.com/its-aryansingh/qrit/services/internal/urlsafety"
)

// Modes of workspace_policies.approval_mode.
const (
	ModeOff                   = "off"
	ModeOutsideAllowlist      = "outside_allowlist"
	ModeAllDestinationChanges = "all_destination_changes"
	ModeAllChanges            = "all_changes"
)

// Reason codes (stable strings stored on approval_requests.reasons).
const (
	ReasonHostNotAllowlisted = "host_not_allowlisted"
	ReasonAllDestination     = "policy_all_destination_changes"
	ReasonNewCode            = "policy_new_code"
)

// Policy is the part of a workspace policy that governs approvals.
type Policy struct {
	Mode         string
	AllowedHosts []string
}

// Change describes a proposed version: every URL it can send scanners to.
type Change struct {
	NewCode bool
	URLs    []string
}

// Evaluate returns why the change needs approval (empty: publish immediately).
func Evaluate(p Policy, c Change) []string {
	set := map[string]bool{}
	switch p.Mode {
	case ModeOutsideAllowlist:
		if len(p.AllowedHosts) > 0 {
			for _, h := range Hosts(c.URLs) {
				if !urlsafety.HostMatches(h, p.AllowedHosts) {
					set[ReasonHostNotAllowlisted] = true
				}
			}
		}
	case ModeAllDestinationChanges:
		if !c.NewCode {
			set[ReasonAllDestination] = true
		}
	case ModeAllChanges:
		if c.NewCode {
			set[ReasonNewCode] = true
		} else {
			set[ReasonAllDestination] = true
		}
	}
	out := make([]string, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// Hosts extracts the distinct hosts of http(s) URLs (other schemes have no host to police).
func Hosts(urls []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range urls {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
			continue
		}
		h := strings.ToLower(u.Hostname())
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	sort.Strings(out)
	return out
}

// Describe renders reason codes for people ("Sent for approval because …").
func Describe(reasons []string, allowed []string) []string {
	out := make([]string, 0, len(reasons))
	for _, r := range reasons {
		switch r {
		case ReasonHostNotAllowlisted:
			msg := "the destination is outside the workspace's approved domains"
			if len(allowed) > 0 {
				msg += " (" + strings.Join(allowed, ", ") + ")"
			}
			out = append(out, msg)
		case ReasonAllDestination:
			out = append(out, "this workspace requires approval for every destination change")
		case ReasonNewCode:
			out = append(out, "this workspace requires approval before new codes go live")
		default:
			out = append(out, r)
		}
	}
	return out
}
