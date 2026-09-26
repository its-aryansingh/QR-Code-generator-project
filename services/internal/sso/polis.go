package sso

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Polis is the SAML bridge. Each organisation is a Polis tenant (its slug); the product is
// fixed. SAML connections are created through the admin API; logins use Polis' OAuth 2.0
// endpoints (authorize → token → userinfo).
type Polis struct {
	http        *http.Client
	internalURL string // service-to-service base (token, userinfo, admin API)
	externalURL string // browser-facing base (authorize)
	apiKey      string
	product     string
}

func NewPolis(client *http.Client, internalURL, externalURL, apiKey, product string) *Polis {
	if externalURL == "" {
		externalURL = internalURL
	}
	return &Polis{http: client, internalURL: strings.TrimRight(internalURL, "/"),
		externalURL: strings.TrimRight(externalURL, "/"), apiKey: apiKey, product: product}
}

// Configured reports whether a bridge URL is set.
func (p *Polis) Configured() bool { return p != nil && p.internalURL != "" }

func (p *Polis) clientID(tenant string) string {
	return "tenant=" + url.QueryEscape(tenant) + "&product=" + url.QueryEscape(p.product)
}

// AuthURL starts an SP-initiated SAML login through the bridge.
func (p *Polis) AuthURL(tenant, redirectURI, state, challenge string, forceAuthn bool) string {
	v := url.Values{"response_type": {"code"}, "client_id": {p.clientID(tenant)}, "redirect_uri": {redirectURI},
		"state": {state}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	if forceAuthn {
		v.Set("forceAuthn", "true")
	}
	return p.externalURL + "/api/oauth/authorize?" + v.Encode()
}

// Exchange redeems the code for an access token.
func (p *Polis) Exchange(ctx context.Context, tenant, code, verifier, redirectURI string) (string, error) {
	tr, err := Exchange(ctx, p.http, p.internalURL+"/api/oauth/token", url.Values{"grant_type": {"authorization_code"},
		"code": {code}, "redirect_uri": {redirectURI}, "client_id": {p.clientID(tenant)}, "client_secret": {"dummy"},
		"code_verifier": {verifier}})
	if err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", errors.New("polis: no access token")
	}
	return tr.AccessToken, nil
}

// polisProfile is Polis' userinfo response.
type polisProfile struct {
	ID        string         `json:"id"`
	Email     string         `json:"email"`
	FirstName string         `json:"firstName"`
	LastName  string         `json:"lastName"`
	Groups    []string       `json:"groups"`
	Raw       map[string]any `json:"raw"`
	Requested map[string]any `json:"requested"`
}

// UserInfo returns the SAML assertion's identity. requested.tenant must equal the tenant
// we started with, so a code minted for another tenant can't be redeemed here.
func (p *Polis) UserInfo(ctx context.Context, token, tenant string, mapping map[string]string) (Claims, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.internalURL+"/api/oauth/userinfo", nil)
	if err != nil {
		return Claims{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := p.http.Do(req)
	if err != nil {
		return Claims{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Claims{}, fmt.Errorf("polis userinfo: %s", res.Status)
	}
	var pr polisProfile
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&pr); err != nil {
		return Claims{}, err
	}
	if t, _ := pr.Requested["tenant"].(string); t != "" && t != tenant {
		return Claims{}, errors.New("polis: tenant mismatch")
	}
	c := Claims{Subject: pr.ID, Email: strings.ToLower(strings.TrimSpace(pr.Email)), EmailVerified: true,
		GivenName: pr.FirstName, FamilyName: pr.LastName, Groups: pr.Groups, Raw: pr.Raw}
	// Custom attribute mapping (e.g. groups under another attribute name).
	if g := mapping["groups"]; g != "" && g != "groups" && pr.Raw != nil {
		if v, ok := pr.Raw[g]; ok {
			c.Groups = nil
			switch x := v.(type) {
			case []any:
				for _, s := range x {
					if str, ok := s.(string); ok {
						c.Groups = append(c.Groups, str)
					}
				}
			case string:
				c.Groups = []string{x}
			}
		}
	}
	if c.Subject == "" {
		c.Subject = c.Email
	}
	return c, nil
}

// CreateConnection registers a SAML IdP with the bridge (metadata XML or URL).
func (p *Polis) CreateConnection(ctx context.Context, tenant, name, metadataXML, metadataURL, redirectURL, defaultRedirect string) error {
	body := url.Values{"tenant": {tenant}, "product": {p.product}, "name": {name},
		"redirectUrl": {`["` + redirectURL + `"]`}, "defaultRedirectUrl": {defaultRedirect}}
	if metadataXML != "" {
		body.Set("rawMetadata", metadataXML)
	} else {
		body.Set("metadataUrl", metadataURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.internalURL+"/api/v1/sso", bytes.NewBufferString(body.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Api-Key "+p.apiKey)
	res, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("polis: create connection: %s %s", res.Status, b)
	}
	return nil
}

// DeleteConnections removes the tenant's SAML connections from the bridge.
func (p *Polis) DeleteConnections(ctx context.Context, tenant string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.internalURL+"/api/v1/sso?tenant="+url.QueryEscape(tenant)+
		"&product="+url.QueryEscape(p.product), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Api-Key "+p.apiKey)
	res, err := p.http.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode >= 300 && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("polis: delete connections: %s", res.Status)
	}
	return nil
}
