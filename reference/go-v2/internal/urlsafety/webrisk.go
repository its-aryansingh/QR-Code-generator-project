package urlsafety

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type WebRiskClient struct {
	apiKey     string
	httpClient *http.Client
	fallback   SafetyClient
}

func NewWebRiskClient(apiKey string) *WebRiskClient {
	return &WebRiskClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		fallback: NewFakeSafetyClient(),
	}
}

type webRiskResponse struct {
	Threat struct {
		ThreatTypes []string `json:"threatTypes"`
	} `json:"threat"`
}

func (c *WebRiskClient) Check(ctx context.Context, targetURL string) (Verdict, error) {
	if c.apiKey == "" {
		return c.fallback.Check(ctx, targetURL)
	}

	endpoint := "https://webrisk.googleapis.com/v1/uris:search"
	params := url.Values{}
	params.Set("key", c.apiKey)
	params.Set("uri", targetURL)
	params.Add("threatTypes", "MALWARE")
	params.Add("threatTypes", "SOCIAL_ENGINEERING")
	params.Add("threatTypes", "UNWANTED_SOFTWARE")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?%s", endpoint, params.Encode()), nil)
	if err != nil {
		return VerdictSafe, nil
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return c.fallback.Check(ctx, targetURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var body webRiskResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err == nil && len(body.Threat.ThreatTypes) > 0 {
			return VerdictUnsafe, nil
		}
	}

	return VerdictSafe, nil
}
