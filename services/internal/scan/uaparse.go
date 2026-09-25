package scan

import (
	"regexp"
	"strings"
)

type ParsedUA struct {
	DeviceType     string `json:"device_type"` // desktop, mobile, tablet, bot, other
	OS             string `json:"os"`
	OSVersion      string `json:"os_version,omitempty"`
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version,omitempty"`
}

var (
	// OS regexes
	iosRegex     = regexp.MustCompile(`(?i)cpu (?:iphone )?os ([0-9_]+)`)
	androidRegex = regexp.MustCompile(`(?i)android ([0-9.]+)`)
	macosRegex   = regexp.MustCompile(`(?i)mac os x ([0-9_]+)`)
	winRegex     = regexp.MustCompile(`(?i)windows nt ([0-9.]+)`)
	linuxRegex   = regexp.MustCompile(`(?i)linux`)

	// Browser regexes
	chromeRegex  = regexp.MustCompile(`(?i)(?:chrome|crios)/([0-9.]+)`)
	safariRegex  = regexp.MustCompile(`(?i)version/([0-9.]+).*safari`)
	firefoxRegex = regexp.MustCompile(`(?i)(?:firefox|fxios)/([0-9.]+)`)
	edgeRegex    = regexp.MustCompile(`(?i)edg(?:e|a|ios)?/([0-9.]+)`)
	samsungRegex = regexp.MustCompile(`(?i)samsungbrowser/([0-9.]+)`)
)

func ParseUserAgent(ua string) ParsedUA {
	clean := strings.TrimSpace(ua)
	if clean == "" {
		return ParsedUA{
			DeviceType: "unknown",
			OS:         "unknown",
			Browser:    "unknown",
		}
	}

	lower := strings.ToLower(clean)
	result := ParsedUA{
		DeviceType: "desktop",
		OS:         "other",
		Browser:    "other",
	}

	// 1. Device Type
	isTablet := strings.Contains(lower, "ipad") ||
		strings.Contains(lower, "tablet") ||
		strings.Contains(lower, "kindle") ||
		strings.Contains(lower, "playbook") ||
		(strings.Contains(lower, "android") && !strings.Contains(lower, "mobile"))

	isMobile := strings.Contains(lower, "mobi") ||
		strings.Contains(lower, "iphone") ||
		strings.Contains(lower, "ipod") ||
		strings.Contains(lower, "blackberry") ||
		strings.Contains(lower, "opera mini") ||
		strings.Contains(lower, "windows phone")

	if isTablet {
		result.DeviceType = "tablet"
	} else if isMobile {
		result.DeviceType = "mobile"
	} else {
		result.DeviceType = "desktop"
	}

	// 2. Operating System
	if m := iosRegex.FindStringSubmatch(clean); len(m) > 1 {
		result.OS = "iOS"
		result.OSVersion = strings.ReplaceAll(m[1], "_", ".")
	} else if m := androidRegex.FindStringSubmatch(clean); len(m) > 1 {
		result.OS = "Android"
		result.OSVersion = m[1]
	} else if m := macosRegex.FindStringSubmatch(clean); len(m) > 1 {
		result.OS = "macOS"
		result.OSVersion = strings.ReplaceAll(m[1], "_", ".")
	} else if m := winRegex.FindStringSubmatch(clean); len(m) > 1 {
		result.OS = "Windows"
		result.OSVersion = m[1]
	} else if linuxRegex.MatchString(clean) {
		result.OS = "Linux"
	}

	// 3. Browser
	// Check Edge and Samsung before Chrome/Safari
	if m := edgeRegex.FindStringSubmatch(clean); len(m) > 1 {
		result.Browser = "Edge"
		result.BrowserVersion = m[1]
	} else if m := samsungRegex.FindStringSubmatch(clean); len(m) > 1 {
		result.Browser = "Samsung Internet"
		result.BrowserVersion = m[1]
	} else if m := chromeRegex.FindStringSubmatch(clean); len(m) > 1 {
		result.Browser = "Chrome"
		result.BrowserVersion = m[1]
	} else if m := safariRegex.FindStringSubmatch(clean); len(m) > 1 {
		result.Browser = "Safari"
		result.BrowserVersion = m[1]
	} else if m := firefoxRegex.FindStringSubmatch(clean); len(m) > 1 {
		result.Browser = "Firefox"
		result.BrowserVersion = m[1]
	}

	return result
}
