package scan

import (
	"strings"
)

// BotRule represents an individual heuristic check.
type BotRule struct {
	Name    string
	Matches func(method, ua string, datacenter bool) bool
}

// BotClassifier coordinates multiple detection heuristics.
type BotClassifier struct {
	customPatterns []string
}

func NewBotClassifier(extraPatterns ...string) *BotClassifier {
	return &BotClassifier{
		customPatterns: extraPatterns,
	}
}

func (c *BotClassifier) Classify(method, ua string, datacenter bool) BotVerdict {
	// 1. Method check: HEAD is preview/ping
	if strings.ToUpper(method) == "HEAD" {
		return BotVerdict{IsBot: true, Reason: "head_request"}
	}

	cleanUA := strings.TrimSpace(ua)
	if cleanUA == "" {
		return BotVerdict{IsBot: true, Reason: "ua_missing"}
	}

	lowerUA := strings.ToLower(cleanUA)

	// 2. Built-in patterns
	for _, pattern := range botPatterns {
		if strings.Contains(lowerUA, pattern) {
			return BotVerdict{IsBot: true, Reason: "ua_bot"}
		}
	}

	// 3. Custom patterns
	for _, pattern := range c.customPatterns {
		if strings.Contains(lowerUA, strings.ToLower(pattern)) {
			return BotVerdict{IsBot: true, Reason: "custom_bot"}
		}
	}

	// 4. Datacenter IP check
	if datacenter {
		return BotVerdict{IsBot: true, Reason: "datacenter"}
	}

	return BotVerdict{IsBot: false}
}
