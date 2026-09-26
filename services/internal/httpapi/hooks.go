package httpapi

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/audit"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/netutil"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
)

// Provisioner runs inside the transaction that creates a workspace, after the workspace row
// exists. The base implementation adds the owner membership; the enterprise layer also
// creates/attaches the organization, role bindings and default policies.
type Provisioner interface {
	ProvisionWorkspace(ctx context.Context, q *dbgen.Queries, tx pgx.Tx, owner dbgen.User, ws dbgen.Workspace, orgID *uuid.UUID) error
}

type baseProvisioner struct{}

func (baseProvisioner) ProvisionWorkspace(ctx context.Context, q *dbgen.Queries, _ pgx.Tx, owner dbgen.User, ws dbgen.Workspace, _ *uuid.UUID) error {
	return q.AddWorkspaceMember(ctx, dbgen.AddWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: owner.ID, Role: "owner"})
}

// SessionPolicy validates a session at refresh time (idle/max lifetime per org policy).
type SessionPolicy interface {
	CheckRefresh(ctx context.Context, sess dbgen.Session) error
}

type noSessionPolicy struct{}

func (noSessionPolicy) CheckRefresh(context.Context, dbgen.Session) error { return nil }

// Auditor records audit entries. The enterprise layer replaces it with an org-aware recorder.
type Auditor interface {
	Record(ctx context.Context, q *dbgen.Queries, e audit.Entry) error
}

type baseAuditor struct{}

func (baseAuditor) Record(ctx context.Context, q *dbgen.Queries, e audit.Entry) error {
	return audit.Record(ctx, q, e)
}

// SetProvisioner, SetSessionPolicy and SetAuditor let enterprise modules replace defaults.
func (s *Server) SetProvisioner(p Provisioner)     { s.provisioner = p }
func (s *Server) SetSessionPolicy(p SessionPolicy) { s.sessionPolicy = p }
func (s *Server) SetAuditor(a Auditor)             { s.auditor = a }

func (s *Server) prov() Provisioner {
	if s.provisioner == nil {
		return baseProvisioner{}
	}
	return s.provisioner
}

func (s *Server) sessPolicy() SessionPolicy {
	if s.sessionPolicy == nil {
		return noSessionPolicy{}
	}
	return s.sessionPolicy
}

func (s *Server) aud() Auditor {
	if s.auditor == nil {
		return baseAuditor{}
	}
	return s.auditor
}

// audit writes an entry using q (pass tx-bound queries to write inside the business transaction).
func (s *Server) audit(r *http.Request, q *dbgen.Queries, wsID *uuid.UUID, action, targetType string, targetID *uuid.UUID, changes map[string]any) error {
	e := audit.Entry{
		WorkspaceID: wsID, Action: action, TargetType: targetType, TargetID: targetID, Changes: changes,
		UserAgent: truncate(r.UserAgent(), 300), RequestID: middleware.GetReqID(r.Context()),
		ActorType: audit.ActorSystem,
	}
	if ip := clientIP(r); ip != nil {
		e.IP = ip.String()
	}
	if p, ok := auth.GetPrincipal(r.Context()); ok {
		switch {
		case p.APIKeyID != uuid.Nil:
			e.ActorType = audit.ActorAPIKey
			id := p.APIKeyID
			e.ActorID = &id
		case p.StaffGrantID != uuid.Nil:
			e.ActorType = audit.ActorStaff
			id := p.UserID
			e.ActorID = &id
		default:
			e.ActorType = audit.ActorUser
			id := p.UserID
			e.ActorID = &id
		}
	}
	return s.aud().Record(r.Context(), q, e)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ipPrefix returns the privacy-safe /24 or /48 of the client.
func ipPrefix(r *http.Request) *string {
	p := netutil.Prefix(clientIP(r))
	if p == "" {
		return nil
	}
	return &p
}

// auditUserEntry is an account-level (no workspace) entry acted by the user themself.
func auditUserEntry(r *http.Request, userID uuid.UUID, action string) audit.Entry {
	id := userID
	e := audit.Entry{
		ActorType: audit.ActorUser, ActorID: &id, Action: action, TargetType: "user", TargetID: &id,
		UserAgent: truncate(r.UserAgent(), 300), RequestID: middleware.GetReqID(r.Context()),
	}
	if ip := clientIP(r); ip != nil {
		e.IP = ip.String()
	}
	return e
}

// MemberHooks let the enterprise layer mirror workspace membership into org membership
// and role bindings. Both run inside the membership transaction.
type MemberHooks interface {
	MemberAdded(ctx context.Context, q *dbgen.Queries, tx pgx.Tx, ws dbgen.Workspace, userID uuid.UUID, role string) error
	MemberRoleChanged(ctx context.Context, q *dbgen.Queries, tx pgx.Tx, ws dbgen.Workspace, userID uuid.UUID, role string) error
	MemberRemoved(ctx context.Context, q *dbgen.Queries, tx pgx.Tx, ws dbgen.Workspace, userID uuid.UUID) error
}

type noMemberHooks struct{}

func (noMemberHooks) MemberAdded(context.Context, *dbgen.Queries, pgx.Tx, dbgen.Workspace, uuid.UUID, string) error {
	return nil
}
func (noMemberHooks) MemberRoleChanged(context.Context, *dbgen.Queries, pgx.Tx, dbgen.Workspace, uuid.UUID, string) error {
	return nil
}
func (noMemberHooks) MemberRemoved(context.Context, *dbgen.Queries, pgx.Tx, dbgen.Workspace, uuid.UUID) error {
	return nil
}

func (s *Server) SetMemberHooks(h MemberHooks) { s.memberHooks = h }

func (s *Server) mem() MemberHooks {
	if s.memberHooks == nil {
		return noMemberHooks{}
	}
	return s.memberHooks
}
