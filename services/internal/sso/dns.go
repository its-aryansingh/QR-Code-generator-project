package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// TXTResolver looks up TXT records (DNS-over-HTTPS in production, a fake in tests).
type TXTResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// DoH queries a JSON DNS-over-HTTPS endpoint (Cloudflare 1.1.1.1 by default). We never
// fetch from the customer's own host, so domain verification has no SSRF surface.
type DoH struct {
	Endpoint string
	Client   *http.Client
}

func (d DoH) LookupTXT(ctx context.Context, name string) ([]string, error) {
	u := d.Endpoint + "?name=" + url.QueryEscape(name) + "&type=TXT"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")
	res, err := d.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh: %s", res.Status)
	}
	var out struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&out); err != nil {
		return nil, err
	}
	var txt []string
	for _, a := range out.Answer {
		if a.Type != 16 {
			continue
		}
		// TXT data arrives quoted and may be split into several 255-byte strings.
		s := strings.ReplaceAll(a.Data, `" "`, "")
		txt = append(txt, strings.Trim(s, `"`))
	}
	return txt, nil
}

// PublicMailDomains cannot be claimed by an organisation.
var PublicMailDomains = map[string]bool{}

func init() {
	for _, d := range strings.Fields(`gmail.com googlemail.com outlook.com hotmail.com live.com msn.com yahoo.com yahoo.co.in
	yahoo.co.uk ymail.com rocketmail.com aol.com icloud.com me.com mac.com protonmail.com proton.me pm.me gmx.com gmx.de
	gmx.net web.de mail.com zoho.com zohomail.in yandex.com yandex.ru mail.ru inbox.ru list.ru bk.ru rediffmail.com
	qq.com 163.com 126.com sina.com sohu.com naver.com daum.net hanmail.net fastmail.com fastmail.fm tutanota.com
	tuta.io hey.com hushmail.com lycos.com orange.fr free.fr laposte.net sfr.fr wanadoo.fr libero.it virgilio.it
	t-online.de freenet.de seznam.cz wp.pl onet.pl interia.pl o2.pl btinternet.com sky.com virginmedia.com talktalk.net
	comcast.net verizon.net att.net sbcglobal.net cox.net charter.net bellsouth.net earthlink.net rogers.com shaw.ca
	sympatico.ca bigpond.com optusnet.com.au telstra.com uol.com.br bol.com.br terra.com.br ig.com.br hotmail.co.uk
	hotmail.fr hotmail.it hotmail.de outlook.in live.in yahoo.fr yahoo.de yahoo.it yahoo.es yahoo.com.br
	aim.com mailinator.com guerrillamail.com 10minutemail.com temp-mail.org duck.com passmail.net skiff.com`) {
		PublicMailDomains[d] = true
	}
}
