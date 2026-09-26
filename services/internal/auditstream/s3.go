package auditstream

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// PutObjectRequest builds an AWS Signature Version 4 signed PUT for S3 or an S3-compatible
// store. With Config.Endpoint set, path-style addressing is used ({endpoint}/{bucket}/{key});
// otherwise virtual-hosted AWS addressing (https://{bucket}.s3.{region}.amazonaws.com/{key}).
func PutObjectRequest(c Config, accessKey, secretKey, key string, body []byte, contentType string, now time.Time) (*http.Request, error) {
	var u *url.URL
	var err error
	escaped := escapePath(key)
	if c.Endpoint != "" {
		u, err = url.Parse(strings.TrimRight(c.Endpoint, "/") + "/" + c.Bucket + "/" + escaped)
	} else {
		u, err = url.Parse("https://" + c.Bucket + ".s3." + c.Region + ".amazonaws.com/" + escaped)
	}
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-amz-server-side-encryption", "AES256")
	SignV4(req, hex.EncodeToString(sum[:]), accessKey, secretKey, c.Region, "s3", now)
	return req, nil
}

// SignV4 signs req with AWS Signature Version 4, covering host and every header already set
// on the request plus x-amz-date and x-amz-content-sha256.
func SignV4(req *http.Request, payloadHash, accessKey, secretKey, region, service string, now time.Time) {
	amzDate := now.UTC().Format("20060102T150405Z")
	day := amzDate[:8]
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)
	signed := map[string]string{"host": req.URL.Host}
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		if lk == "authorization" || lk == "user-agent" {
			continue
		}
		signed[lk] = strings.Join(v, ",")
	}
	names := make([]string, 0, len(signed))
	for k := range signed {
		names = append(names, k)
	}
	sort.Strings(names)
	var canonHeaders strings.Builder
	for _, k := range names {
		canonHeaders.WriteString(k + ":" + strings.TrimSpace(signed[k]) + "\n")
	}
	signedHeaders := strings.Join(names, ";")
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	canonical := strings.Join([]string{req.Method, path, req.URL.RawQuery, canonHeaders.String(), signedHeaders, payloadHash}, "\n")
	scope := day + "/" + region + "/" + service + "/aws4_request"
	ch := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(ch[:])
	k := hmacSHA256([]byte("AWS4"+secretKey), day)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, service)
	k = hmacSHA256(k, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(k, toSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+sig)
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// escapePath URI-encodes each path segment as SigV4 requires (unreserved characters kept).
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		var b strings.Builder
		for _, c := range []byte(s) {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
				b.WriteByte(c)
			} else {
				b.WriteString("%" + strings.ToUpper(hex.EncodeToString([]byte{c})))
			}
		}
		parts[i] = b.String()
	}
	return strings.Join(parts, "/")
}
