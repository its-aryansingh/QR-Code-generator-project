package redirect

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/its-aryansingh/qrit/services/internal/resolve"
)

// resolvedLinkSQL loads everything the redirect needs in one round trip: the code, its
// workspace timezone, the version in effect now (greatest effective_at <= now, then
// greatest version_no) and the next scheduled change (cache expiry).
const resolvedLinkSQL = `
SELECT q.id, q.workspace_id, q.campaign_id, q.status, q.safety_status, q.starts_at, q.expires_at,
       q.scan_limit, q.total_scans, q.password_hash, q.fallback_url, w.timezone,
       v.id, v.version_no, v.destination_kind, v.destination_url, v.rules, v.utm, v.hosted_page,
       (SELECT min(v2.effective_at) FROM qr_versions v2 WHERE v2.qr_code_id = q.id AND v2.effective_at > now()
          AND v2.approval_status IN ('not_required','approved'))
FROM qr_codes q
JOIN workspaces w ON w.id = q.workspace_id AND w.deleted_at IS NULL
LEFT JOIN LATERAL (
    SELECT * FROM qr_versions v1
    WHERE v1.qr_code_id = q.id AND v1.effective_at <= now()
      AND v1.approval_status IN ('not_required','approved') -- versions awaiting approval are never served
    ORDER BY v1.effective_at DESC, v1.version_no DESC
    LIMIT 1
) v ON true
WHERE q.domain_id = $1 AND q.short_code = $2 AND q.mode = 'dynamic' AND q.deleted_at IS NULL`

// FetchLink returns a resolve.FetchFunc backed by Postgres.
func FetchLink(pool *pgxpool.Pool) resolve.FetchFunc {
	return func(ctx context.Context, domainID uuid.UUID, code string) (*resolve.ResolvedLink, error) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		var (
			l                            resolve.ResolvedLink
			campaign, verID              *uuid.UUID
			startsAt, expiresAt, nextAt  *time.Time
			verNo                        *int32
			kind, destURL                *string
			rules, utm, hostedPage       []byte
		)
		err := pool.QueryRow(ctx, resolvedLinkSQL, domainID, code).Scan(
			&l.QRCodeID, &l.WorkspaceID, &campaign, &l.Status, &l.Safety, &startsAt, &expiresAt,
			&l.ScanLimit, &l.TotalScans, &l.PasswordHash, &l.FallbackURL, &l.Timezone,
			&verID, &verNo, &kind, &destURL, &rules, &utm, &hostedPage, &nextAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		l.DomainID = domainID
		l.CampaignID = campaign
		l.StartsAt, l.ExpiresAt, l.NextChangeAt = startsAt, expiresAt, nextAt
		l.HasPassword = l.PasswordHash != nil && *l.PasswordHash != ""
		if verID != nil {
			v := &resolve.ResolvedVersion{ID: *verID, Rules: rules, UTM: utm}
			if verNo != nil {
				v.No = int(*verNo)
			}
			if kind != nil {
				v.Kind = *kind
			}
			if destURL != nil {
				v.URL = *destURL
			}
			if len(hostedPage) > 0 {
				v.HostedPage = json.RawMessage(hostedPage)
			}
			l.Version = v
		}
		return &l, nil
	}
}

// Domain is an active short-link host.
type Domain struct {
	ID              uuid.UUID
	Hostname        string
	WorkspaceID     *uuid.UUID
	RootRedirectURL *string
	NotFoundURL     *string
}

func loadDomains(ctx context.Context, pool *pgxpool.Pool) (map[string]Domain, error) {
	rows, err := pool.Query(ctx, `SELECT id, hostname::text, workspace_id, root_redirect_url, not_found_url
		FROM domains WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Domain{}
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Hostname, &d.WorkspaceID, &d.RootRedirectURL, &d.NotFoundURL); err != nil {
			return nil, err
		}
		out[strings.ToLower(d.Hostname)] = d
	}
	return out, rows.Err()
}
