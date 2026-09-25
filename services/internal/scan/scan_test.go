package scan_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/scan"
)

func TestVisitorHashProperties(t *testing.T) {
	secret := []byte("master-secret-salt-key-32-bytes!")
	day1 := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	day1Later := time.Date(2026, 9, 25, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 26, 1, 0, 0, 0, time.UTC)

	salt1 := scan.SaltForDate(secret, day1)
	salt1Later := scan.SaltForDate(secret, day1Later)
	salt2 := scan.SaltForDate(secret, day2)

	// Salts within same UTC day are identical
	if string(salt1) != string(salt1Later) {
		t.Fatalf("salts within same day must be identical")
	}
	// Salts across days are different
	if string(salt1) == string(salt2) {
		t.Fatalf("salts across different days must differ")
	}

	qr1 := uuid.New()
	qr2 := uuid.New()
	ip1 := "203.0.113.195"
	ip2 := "198.51.100.4"
	ua1 := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)"
	ua2 := "Mozilla/5.0 (Linux; Android 14; Pixel 8)"

	vh1 := scan.ComputeVisitorHash(salt1, qr1, ip1, ua1)
	vh1Again := scan.ComputeVisitorHash(salt1Later, qr1, ip1, ua1)

	// Stable within same day for same visitor on same QR
	if vh1 != vh1Again {
		t.Errorf("visitor hash must be stable within same UTC day: %s != %s", vh1, vh1Again)
	}

	// Different across days
	vhDay2 := scan.ComputeVisitorHash(salt2, qr1, ip1, ua1)
	if vh1 == vhDay2 {
		t.Errorf("visitor hash must differ across days")
	}

	// Different across QRs
	vhQR2 := scan.ComputeVisitorHash(salt1, qr2, ip1, ua1)
	if vh1 == vhQR2 {
		t.Errorf("visitor hash must differ across different QRs")
	}

	// Different across IPs
	vhIP2 := scan.ComputeVisitorHash(salt1, qr1, ip2, ua1)
	if vh1 == vhIP2 {
		t.Errorf("visitor hash must differ across different IPs")
	}

	// Different across UAs
	vhUA2 := scan.ComputeVisitorHash(salt1, qr1, ip1, ua2)
	if vh1 == vhUA2 {
		t.Errorf("visitor hash must differ across different UAs")
	}
}

func TestClassifyBotFixtures(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		ua         string
		datacenter bool
		wantBot    bool
		wantReason string
	}{
		// HEAD requests
		{"HEAD request on mobile", "HEAD", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)", false, true, "head_request"},
		{"HEAD request empty UA", "HEAD", "", false, true, "head_request"},

		// Missing / empty UA
		{"empty UA", "GET", "", false, true, "ua_missing"},
		{"whitespace UA", "GET", "   ", false, true, "ua_missing"},

		// Real mobile browsers (NOT bots)
		{"iOS Safari", "GET", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Mobile/15E148 Safari/604.1", false, false, ""},
		{"Android Chrome", "GET", "Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 Mobile Safari/537.36", false, false, ""},
		{"Samsung Internet", "GET", "Mozilla/5.0 (Linux; Android 13; SAMSUNG SM-A536B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/24.0 Chrome/118.0.5993.80 Mobile Safari/537.36", false, false, ""},
		{"Firefox Android", "GET", "Mozilla/5.0 (Android 14; Mobile; rv:125.0) Gecko/125.0 Firefox/125.0", false, false, ""},
		{"iPad Safari", "GET", "Mozilla/5.0 (iPad; CPU OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1", false, false, ""},
		{"Desktop Mac Chrome", "GET", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36", false, false, ""},
		{"Desktop Windows Edge", "GET", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36 Edg/124.0.0.0", false, false, ""},

		// Social / Messenger Preview Bots (>= 10 fixtures)
		{"Facebook preview", "GET", "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)", false, true, "ua_bot"},
		{"WhatsApp preview", "GET", "WhatsApp/2.24.8.85 A", false, true, "ua_bot"},
		{"TelegramBot", "GET", "TelegramBot (like TwitterBot)", false, true, "ua_bot"},
		{"Slackbot", "GET", "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", false, true, "ua_bot"},
		{"Twitterbot", "GET", "Twitterbot/1.0", false, true, "ua_bot"},
		{"LinkedInBot", "GET", "LinkedInBot/1.0 (compatible; Mozilla/5.0; Apache-HttpClient +http://www.linkedin.com)", false, true, "ua_bot"},
		{"Discordbot", "GET", "Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)", false, true, "ua_bot"},
		{"Applebot", "GET", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15 (Applebot/0.1; +http://www.apple.com/go/applebot)", false, true, "ua_bot"},
		{"Skype preview", "GET", "Mozilla/5.0 (Windows NT 6.1; WOW64) SkypeUriPreview Preview/0.5", false, true, "ua_bot"},

		// Search Engine Crawlers (>= 9 fixtures)
		{"Googlebot", "GET", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", false, true, "ua_bot"},
		{"Bingbot", "GET", "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)", false, true, "ua_bot"},
		{"Baiduspider", "GET", "Baiduspider+(+http://www.baidu.com/search/spider.htm)", false, true, "ua_bot"},
		{"YandexBot", "GET", "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)", false, true, "ua_bot"},
		{"DuckDuckBot", "GET", "DuckDuckBot/1.0; (+http://duckduckgo.com/duckduckbot.html)", false, true, "ua_bot"},
		{"Yahoo Slurp", "GET", "Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)", false, true, "ua_bot"},
		{"Sogou spider", "GET", "Sogou web spider/4.0(+http://www.sogou.com/docs/help/webmasters.htm#07)", false, true, "ua_bot"},
		{"Exabot", "GET", "Mozilla/5.0 (compatible; Konqueror/3.5; Linux) KHTML/3.5.5 (like Gecko) (Exabot-Thumbnails)", false, true, "ua_bot"},
		{"Internet Archive", "GET", "ia_archiver (+http://www.alexa.com/site/help/webmasters; crawler@alexa.com)", false, true, "ua_bot"},

		// Security Scanners (>= 8 fixtures)
		{"Microsoft Defender SafeLinks", "GET", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 SafeLinks/1.0", false, true, "ua_bot"},
		{"Proofpoint URL Defense", "GET", "Proofpoint-URL-Defense/1.0", false, true, "ua_bot"},
		{"Mimecast scanner", "GET", "Mozilla/5.0 (compatible; Mimecast-Inbound-Filter)", false, true, "ua_bot"},
		{"Barracuda Networks", "GET", "Mozilla/5.0 (compatible; Barracuda-URL-Check/1.0)", false, true, "ua_bot"},
		{"Trend Micro", "GET", "TrendMicro-Web-Reputation/1.0", false, true, "ua_bot"},
		{"Forcepoint", "GET", "Forcepoint-ThreatSeeker", false, true, "ua_bot"},
		{"Symantec scanner", "GET", "Symantec-Threat-Intelligence", false, true, "ua_bot"},
		{"Kaspersky crawler", "GET", "Kaspersky-Lab-Web-Protection/1.0", false, true, "ua_bot"},

		// HTTP Libraries / Scrapers (>= 9 fixtures)
		{"curl tool", "GET", "curl/8.4.0", false, true, "ua_bot"},
		{"wget tool", "GET", "Wget/1.21.4", false, true, "ua_bot"},
		{"Python requests", "GET", "python-requests/2.31.0", false, true, "ua_bot"},
		{"Go HTTP client", "GET", "Go-http-client/2.0", false, true, "ua_bot"},
		{"OkHttp client", "GET", "okhttp/4.12.0", false, true, "ua_bot"},
		{"Axios client", "GET", "axios/1.6.8", false, true, "ua_bot"},
		{"node-fetch", "GET", "node-fetch/1.0 (+https://github.com/bitinn/node-fetch)", false, true, "ua_bot"},
		{"Apache HttpClient", "GET", "Apache-HttpClient/4.5.14 (Java/17.0.10)", false, true, "ua_bot"},
		{"Postman runtime", "GET", "PostmanRuntime/7.37.3", false, true, "ua_bot"},

		// Automation & Headless (>= 5 fixtures)
		{"Headless Chrome", "GET", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/124.0.6367.60 Safari/537.36", false, true, "ua_bot"},
		{"PhantomJS", "GET", "Mozilla/5.0 (Unknown; Linux x86_64) AppleWebKit/538.1 (KHTML, like Gecko) PhantomJS/2.1.1 Safari/538.1", false, true, "ua_bot"},
		{"Selenium browser", "GET", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0; selenium) Gecko/20100101 Firefox/115.0", false, true, "ua_bot"},
		{"Puppeteer", "GET", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 (Puppeteer)", false, true, "ua_bot"},
		{"Playwright", "GET", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Playwright/1.43.0", false, true, "ua_bot"},

		// Datacenter IP flag (1 fixture)
		{"Datacenter Desktop Browser", "GET", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36", true, true, "datacenter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict := scan.ClassifyBot(tt.method, tt.ua, tt.datacenter)
			if verdict.IsBot != tt.wantBot {
				t.Errorf("ClassifyBot() isBot = %v; want %v (ua: %s)", verdict.IsBot, tt.wantBot, tt.ua)
			}
			if tt.wantReason != "" && verdict.Reason != tt.wantReason {
				t.Errorf("ClassifyBot() reason = %q; want %q", verdict.Reason, tt.wantReason)
			}
		})
	}
}
