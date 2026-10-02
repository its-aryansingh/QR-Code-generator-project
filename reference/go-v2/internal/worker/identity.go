package worker

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/its-aryansingh/qrit/services/internal/audit"
	"github.com/its-aryansingh/qrit/services/internal/jobs"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/sso"
)

// DomainPollWindow is how long after a domain is claimed we keep looking for its TXT record.
const DomainPollWindow = 72 * time.Hour

// RegisterIdentity adds identity housekeeping: background domain verification and cleanup
// of expired MFA/SSO artefacts.
func RegisterIdentity(s *jobs.Scheduler, d *Deps, dns sso.TXTResolver) {
	if dns != nil {
		s.Add(jobs.Task{Name: "domains.verify", Every: 10 * time.Minute, Timeout: 5 * time.Minute,
			Run: func(ctx context.Context) error { _, err := d.VerifyPendingDomains(ctx, dns); return err }})
	}
	s.Add(jobs.Task{Name: "identity.cleanup", DailyAtUTC: jobs.Daily(5, 30), Run: d.IdentityCleanup})
}

// VerifyPendingDomains re-checks the TXT record of every unverified domain claimed within the
// poll window, backing off as attempts accumulate (10 min, then hourly after 12 tries).
func (d *Deps) VerifyPendingDomains(ctx context.Context, dns sso.TXTResolver) (int, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id, org_id, domain::text, verification_token FROM org_domains
		WHERE verified_at IS NULL AND created_at > now() - $1::interval
		  AND NOT EXISTS (SELECT 1 FROM org_domains v WHERE v.domain = org_domains.domain AND v.verified_at IS NOT NULL)
		  AND (last_checked_at IS NULL
		       OR last_checked_at < now() - CASE WHEN check_attempts < 12 THEN interval '9 minutes' ELSE interval '59 minutes' END)
		ORDER BY last_checked_at NULLS FIRST LIMIT 200`, DomainPollWindow.String())
	if err != nil {
		return 0, err
	}
	type pending struct {
		id, org uuid.UUID
		domain  string
		token   string
	}
	var list []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.org, &p.domain, &p.token); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	verified := 0
	for _, p := range list {
		ok, lookupErr := checkTXT(ctx, dns, p.domain, p.token)
		if _, err := d.Pool.Exec(ctx, `UPDATE org_domains SET last_checked_at = now(), check_attempts = check_attempts + 1 WHERE id = $1`, p.id); err != nil {
			return verified, err
		}
		if lookupErr != nil {
			d.log().Warn("domain TXT lookup failed", "domain", p.domain, "error", lookupErr)
			continue
		}
		if !ok {
			continue
		}
		tx, err := d.Pool.Begin(ctx)
		if err != nil {
			return verified, err
		}
		tag, err := tx.Exec(ctx, `UPDATE org_domains SET verified_at = now() WHERE id = $1 AND verified_at IS NULL`, p.id)
		if err == nil && tag.RowsAffected() == 1 {
			id, org := p.id, p.org
			err = audit.Record(ctx, dbgen.New(tx), audit.Entry{OrgID: &org, ActorType: audit.ActorSystem,
				Action: "org.domain.verified", TargetType: "org_domain", TargetID: &id,
				Changes: map[string]any{"domain": p.domain, "via": "background_check"}})
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			var pe *pgconn.PgError
			if errors.As(err, &pe) && pe.Code == "23505" {
				continue // another organisation verified the domain first
			}
			return verified, err
		}
		if err := tx.Commit(ctx); err != nil {
			return verified, err
		}
		verified++
		d.log().Info("domain verified", "domain", p.domain, "org_id", p.org)
	}
	return verified, nil
}

func checkTXT(ctx context.Context, dns sso.TXTResolver, domain, token string) (bool, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	txt, err := dns.LookupTXT(cctx, "_qrit-challenge."+domain)
	if err != nil {
		return false, err
	}
	for _, t := range txt {
		if strings.TrimSpace(t) == "qrit-domain-verification="+token {
			return true, nil
		}
	}
	return false, nil
}

// IdentityCleanup drops recovery codes left behind by removed factors and unverified domain
// claims whose poll window lapsed long ago (so the name can be claimed again).
func (d *Deps) IdentityCleanup(ctx context.Context) error {
	stmts := []string{
		`DELETE FROM user_recovery_codes rc WHERE NOT EXISTS (SELECT 1 FROM user_mfa_factors f WHERE f.user_id = rc.user_id)`,
		`DELETE FROM org_domains WHERE verified_at IS NULL AND created_at < now() - interval '30 days'`,
	}
	for _, q := range stmts {
		if _, err := d.Pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}
