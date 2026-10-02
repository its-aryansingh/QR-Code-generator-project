package audit

import (
	"context"
	"encoding/json"
	"net"
	"strings"

	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type ActorType string

const (
	ActorUser   ActorType = "user"
	ActorAPIKey ActorType = "api_key"
	ActorSystem ActorType = "system"
	ActorStaff  ActorType = "staff"
)

// TruncateIPToPrefix truncates an IP address to /24 (IPv4) or /48 (IPv6) for privacy compliance.
func TruncateIPToPrefix(rawIP string) *string {
	ipStr := strings.TrimSpace(rawIP)
	if ipStr == "" {
		return nil
	}
	parsed := net.ParseIP(ipStr)
	if parsed == nil {
		return nil
	}

	if ipv4 := parsed.To4(); ipv4 != nil {
		mask := net.CIDRMask(24, 32)
		network := ipv4.Mask(mask)
		res := network.String() + "/24"
		return &res
	}

	mask := net.CIDRMask(48, 128)
	network := parsed.Mask(mask)
	res := network.String() + "/48"
	return &res
}

// Entry contains the audit trail record details.
type Entry struct {
	// OrgID defaults to the workspace's organisation when nil.
	OrgID       *uuid.UUID
	WorkspaceID *uuid.UUID
	ActorType   ActorType
	ActorID     *uuid.UUID
	Action      string
	TargetType  string
	TargetID    *uuid.UUID
	Changes     map[string]interface{}
	IP          string
	UserAgent   string
	RequestID   string
}

func toPgUUID(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

// Record persists an immutable audit log entry into the database.
func Record(ctx context.Context, q *dbgen.Queries, entry Entry) error {
	var changesBytes []byte
	if entry.Changes != nil {
		changesBytes, _ = json.Marshal(Redact(entry.Changes))
	} else {
		changesBytes = []byte("{}")
	}

	var ipPrefix *string
	if entry.IP != "" {
		ipPrefix = TruncateIPToPrefix(entry.IP)
	}

	var ua *string
	if entry.UserAgent != "" {
		ua = &entry.UserAgent
	}

	var reqID *string
	if entry.RequestID != "" {
		reqID = &entry.RequestID
	}

	params := dbgen.CreateAuditLogParams{
		OrgID:       toPgUUID(entry.OrgID),
		WorkspaceID: toPgUUID(entry.WorkspaceID),
		ActorType:   string(entry.ActorType),
		ActorID:     toPgUUID(entry.ActorID),
		Action:      entry.Action,
		TargetType:  entry.TargetType,
		TargetID:    toPgUUID(entry.TargetID),
		Changes:     changesBytes,
		IpPrefix:    ipPrefix,
		UserAgent:   ua,
		RequestID:   reqID,
	}

	_, err := q.CreateAuditLog(ctx, params)
	return err
}

// sensitiveKeys are never written to the audit log, whatever the caller passes.
var sensitiveKeys = []string{"password", "secret", "token", "private_key", "api_key", "credential", "totp", "recovery"}

// Redact masks values of sensitive keys at any depth.
func Redact(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		lk := strings.ToLower(k)
		masked := false
		for _, s := range sensitiveKeys {
			if strings.Contains(lk, s) {
				masked = true
				break
			}
		}
		switch {
		case masked:
			if str, ok := v.(string); ok && (str == "set" || str == "cleared" || str == "rotated") {
				out[k] = str
			} else {
				out[k] = "[redacted]"
			}
		default:
			if sub, ok := v.(map[string]interface{}); ok {
				out[k] = Redact(sub)
			} else {
				out[k] = v
			}
		}
	}
	return out
}
