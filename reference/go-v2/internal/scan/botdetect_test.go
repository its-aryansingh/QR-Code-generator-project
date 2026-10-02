package scan

import (
	"testing"
)

func TestBotDetectionFixtures(t *testing.T) {
	classifier := NewBotClassifier()

	fixtures := []struct {
		name       string
		method     string
		ua         string
		datacenter bool
		wantBot    bool
		wantReason string
	}{
		// Method checks
		{name: "HEAD request", method: "HEAD", ua: "Mozilla/5.0", wantBot: true, wantReason: "head_request"},
		{name: "Empty UA", method: "GET", ua: "", wantBot: true, wantReason: "ua_missing"},
		{name: "Whitespace UA", method: "GET", ua: "   ", wantBot: true, wantReason: "ua_missing"},

		// Datacenter checks
		{name: "Datacenter browser", method: "GET", ua: "Mozilla/5.0", datacenter: true, wantBot: true, wantReason: "datacenter"},

		// Link preview bots
		{name: "WhatsApp preview", method: "GET", ua: "WhatsApp/2.21.11.17 A", wantBot: true, wantReason: "ua_bot"},
		{name: "Slackbot", method: "GET", ua: "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", wantBot: true, wantReason: "ua_bot"},
		{name: "TelegramBot", method: "GET", ua: "TelegramBot (like TwitterBot)", wantBot: true, wantReason: "ua_bot"},
		{name: "Twitterbot", method: "GET", ua: "Twitterbot/1.0", wantBot: true, wantReason: "ua_bot"},
		{name: "LinkedInBot", method: "GET", ua: "LinkedInBot/1.0 (compatible; Mozilla/5.0; Apache-HttpClient +http://www.linkedin.com)", wantBot: true, wantReason: "ua_bot"},
		{name: "DiscordBot", method: "GET", ua: "Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)", wantBot: true, wantReason: "ua_bot"},
		{name: "Facebook preview", method: "GET", ua: "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)", wantBot: true, wantReason: "ua_bot"},
		{name: "Applebot", method: "GET", ua: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_1) AppleWebKit/600.2.5 (KHTML, like Gecko) Version/8.0.2 Safari/600.2.5 (Applebot/0.1)", wantBot: true, wantReason: "ua_bot"},
		{name: "SkypeUriPreview", method: "GET", ua: "Mozilla/5.0 (Windows NT 6.1; WOW64) SkypeUriPreview Preview/0.5", wantBot: true, wantReason: "ua_bot"},

		// Search engines
		{name: "Googlebot", method: "GET", ua: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", wantBot: true, wantReason: "ua_bot"},
		{name: "Bingbot", method: "GET", ua: "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)", wantBot: true, wantReason: "ua_bot"},
		{name: "Baiduspider", method: "GET", ua: "Baiduspider+(+http://www.baidu.com/search/spider.htm)", wantBot: true, wantReason: "ua_bot"},
		{name: "YandexBot", method: "GET", ua: "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)", wantBot: true, wantReason: "ua_bot"},
		{name: "DuckDuckBot", method: "GET", ua: "DuckDuckBot/1.0; (+http://duckduckgo.com/duckduckbot.html)", wantBot: true, wantReason: "ua_bot"},

		// Security scanners
		{name: "SafeLinks", method: "GET", ua: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/64.0.3282.140 Safari/537.36 Edge/18.17763 Safari/537.36 safelinks", wantBot: true, wantReason: "ua_bot"},
		{name: "Proofpoint", method: "GET", ua: "Proofpoint URL Defense Scanner", wantBot: true, wantReason: "ua_bot"},
		{name: "Mimecast", method: "GET", ua: "Mozilla/5.0 (compatible; Mimecast Web Security)", wantBot: true, wantReason: "ua_bot"},
		{name: "Barracuda", method: "GET", ua: "Barracuda Sentinel Scanner", wantBot: true, wantReason: "ua_bot"},
		{name: "TrendMicro", method: "GET", ua: "TrendMicro Web Reputation", wantBot: true, wantReason: "ua_bot"},

		// CLI & HTTP libraries
		{name: "curl", method: "GET", ua: "curl/7.88.1", wantBot: true, wantReason: "ua_bot"},
		{name: "wget", method: "GET", ua: "Wget/1.21.3", wantBot: true, wantReason: "ua_bot"},
		{name: "python-requests", method: "GET", ua: "python-requests/2.31.0", wantBot: true, wantReason: "ua_bot"},
		{name: "go-http-client", method: "GET", ua: "Go-http-client/1.1", wantBot: true, wantReason: "ua_bot"},
		{name: "okhttp", method: "GET", ua: "okhttp/4.9.2", wantBot: true, wantReason: "ua_bot"},
		{name: "axios", method: "GET", ua: "axios/1.6.2", wantBot: true, wantReason: "ua_bot"},
		{name: "node-fetch", method: "GET", ua: "node-fetch/1.0 (+https://github.com/bitinn/node-fetch)", wantBot: true, wantReason: "ua_bot"},
		{name: "PostmanRuntime", method: "GET", ua: "PostmanRuntime/7.32.3", wantBot: true, wantReason: "ua_bot"},

		// Automation / Headless
		{name: "HeadlessChrome", method: "GET", ua: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/121.0.6167.85 Safari/537.36", wantBot: true, wantReason: "ua_bot"},
		{name: "Puppeteer", method: "GET", ua: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Puppeteer Extra", wantBot: true, wantReason: "ua_bot"},
		{name: "Selenium", method: "GET", ua: "Selenium WebDriver Mozilla/5.0", wantBot: true, wantReason: "ua_bot"},
		{name: "Playwright", method: "GET", ua: "Mozilla/5.0 (X11; Linux x86_64) Playwright Browser", wantBot: true, wantReason: "ua_bot"},

		// Real Human Browsers (NOT BOTS)
		{
			name:    "iPhone Safari",
			method:  "GET",
			ua:      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_3_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3.1 Mobile/15E148 Safari/604.1",
			wantBot: false,
		},
		{
			name:    "Android Chrome Mobile",
			method:  "GET",
			ua:      "Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.6167.178 Mobile Safari/537.36",
			wantBot: false,
		},
		{
			name:    "Samsung Internet Mobile",
			method:  "GET",
			ua:      "Mozilla/5.0 (Linux; Android 13; SAMSUNG SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/23.0 Chrome/115.0.0.0 Mobile Safari/537.36",
			wantBot: false,
		},
		{
			name:    "macOS Safari Desktop",
			method:  "GET",
			ua:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
			wantBot: false,
		},
		{
			name:    "Windows Chrome Desktop",
			method:  "GET",
			ua:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			wantBot: false,
		},
		{
			name:    "Linux Firefox Desktop",
			method:  "GET",
			ua:      "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:122.0) Gecko/20100101 Firefox/122.0",
			wantBot: false,
		},
	}

	for _, tt := range fixtures {
		t.Run(tt.name, func(t *testing.T) {
			verdict := classifier.Classify(tt.method, tt.ua, tt.datacenter)
			if verdict.IsBot != tt.wantBot {
				t.Errorf("got IsBot = %v, want %v (reason: %s)", verdict.IsBot, tt.wantBot, verdict.Reason)
			}
			if tt.wantReason != "" && verdict.Reason != tt.wantReason {
				t.Errorf("got Reason = %q, want %q", verdict.Reason, tt.wantReason)
			}
		})
	}
}
