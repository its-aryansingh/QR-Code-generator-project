package httpapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
)

// DTOs keep secrets (password hashes, token hashes) out of every response.

type userDTO struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	AvatarURL       *string    `json:"avatar_url"`
	Locale          string     `json:"locale"`
	Timezone        string     `json:"timezone"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	HasPassword     bool       `json:"has_password"`
	IsStaff         bool       `json:"is_staff"`
	CreatedAt       time.Time  `json:"created_at"`
}

func toUserDTO(u dbgen.User) userDTO {
	return userDTO{
		ID: u.ID, Email: u.Email, Name: u.Name, AvatarURL: u.AvatarUrl, Locale: u.Locale,
		Timezone: u.Timezone, EmailVerifiedAt: tsPtr(u.EmailVerifiedAt), HasPassword: u.PasswordHash != nil,
		IsStaff: u.IsStaff, CreatedAt: u.CreatedAt,
	}
}

type workspaceDTO struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	OwnerID   uuid.UUID  `json:"owner_id"`
	PlanID    string     `json:"plan_id"`
	Timezone  string     `json:"timezone"`
	Role      string     `json:"role,omitempty"`
	OrgID     *uuid.UUID `json:"org_id,omitempty"`
	IsSandbox bool       `json:"is_sandbox"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func toWorkspaceDTO(w dbgen.Workspace, role string) workspaceDTO {
	d := workspaceDTO{ID: w.ID, Name: w.Name, Slug: w.Slug, OwnerID: w.OwnerID, PlanID: w.PlanID,
		Timezone: w.Timezone, Role: role, IsSandbox: w.IsSandbox, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt}
	if w.OrgID != uuid.Nil {
		id := w.OrgID
		d.OrgID = &id
	}
	return d
}

type versionDTO struct {
	ID              uuid.UUID       `json:"id"`
	VersionNo       int32           `json:"version_no"`
	DestinationKind string          `json:"destination_kind"`
	DestinationURL  *string         `json:"destination_url"`
	HostedPage      json.RawMessage `json:"hosted_page,omitempty"`
	Rules           json.RawMessage `json:"rules"`
	UTM             json.RawMessage `json:"utm"`
	EffectiveAt     time.Time       `json:"effective_at"`
	Status          string          `json:"status"` // current | scheduled | superseded | pending_approval | rejected | cancelled
	ApprovalStatus  string          `json:"approval_status"`
	ApprovalID      *uuid.UUID      `json:"approval_request_id,omitempty"`
	SafetyStatus    string          `json:"safety_status"`
	RestoredFrom    *uuid.UUID      `json:"restored_from"`
	ChangeNote      *string         `json:"change_note"`
	CreatedBy       *uuid.UUID      `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
}

func toVersionDTO(v dbgen.QrVersion, currentID *uuid.UUID, now time.Time) versionDTO {
	status := "superseded"
	switch {
	case v.ApprovalStatus == "pending":
		status = "pending_approval"
	case v.ApprovalStatus == "rejected" || v.ApprovalStatus == "cancelled":
		status = v.ApprovalStatus
	case v.EffectiveAt.After(now):
		status = "scheduled"
	case currentID != nil && *currentID == v.ID:
		status = "current"
	}
	var hp json.RawMessage
	if len(v.HostedPage) > 0 {
		hp = json.RawMessage(v.HostedPage)
	}
	return versionDTO{
		ID: v.ID, VersionNo: v.VersionNo, DestinationKind: v.DestinationKind, DestinationURL: v.DestinationUrl,
		HostedPage: hp, Rules: nonNullJSON(v.Rules, "[]"), UTM: nonNullJSON(v.Utm, "{}"),
		EffectiveAt: v.EffectiveAt, Status: status, SafetyStatus: v.SafetyStatus,
		RestoredFrom: uuidPtr(v.RestoredFrom), ChangeNote: v.ChangeNote, CreatedBy: uuidPtr(v.CreatedBy),
		CreatedAt: v.CreatedAt, ApprovalStatus: v.ApprovalStatus, ApprovalID: uuidPtr(v.ApprovalRequestID),
	}
}

type qrDTO struct {
	ID               uuid.UUID       `json:"id"`
	WorkspaceID      uuid.UUID       `json:"workspace_id"`
	Mode             string          `json:"mode"`
	ContentType      string          `json:"content_type"`
	Name             string          `json:"name"`
	Status           string          `json:"status"`
	SafetyStatus     string          `json:"safety_status"`
	IsReadOnly       bool            `json:"is_read_only"`
	ShortCode        *string         `json:"short_code"`
	ShortURL         *string         `json:"short_url"`
	EncodedPayload   string          `json:"encoded_payload"`
	StaticPayload    *string         `json:"static_payload"`
	StaticContent    json.RawMessage `json:"static_content,omitempty"`
	Design           json.RawMessage `json:"design"`
	DesignHash       string          `json:"design_hash"`
	FolderID         *uuid.UUID      `json:"folder_id"`
	CampaignID       *uuid.UUID      `json:"campaign_id"`
	TemplateID       *uuid.UUID      `json:"template_id"`
	StartsAt         *time.Time      `json:"starts_at"`
	ExpiresAt        *time.Time      `json:"expires_at"`
	ScanLimit        *int64          `json:"scan_limit"`
	HasPassword      bool            `json:"has_password"`
	FallbackURL      *string         `json:"fallback_url"`
	TotalScans       int64           `json:"total_scans"`
	UniqueScans      int64           `json:"unique_scans"`
	LastScannedAt    *time.Time      `json:"last_scanned_at"`
	CurrentVersion   *versionDTO     `json:"current_version,omitempty"`
	ScheduledVersion *versionDTO     `json:"scheduled_version,omitempty"`
	DestinationURL   *string         `json:"destination_url,omitempty"`
	Tags             []tagRef        `json:"tags,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	ArchivedAt       *time.Time      `json:"archived_at"`
	DeletedAt        *time.Time      `json:"deleted_at,omitempty"`
}

func (s *Server) toQRDTO(ctx context.Context, q dbgen.QrCode) qrDTO {
	d := qrDTO{
		ID: q.ID, WorkspaceID: q.WorkspaceID, Mode: q.Mode, ContentType: q.ContentType, Name: q.Name,
		Status: q.Status, SafetyStatus: q.SafetyStatus, IsReadOnly: q.IsReadOnly, ShortCode: q.ShortCode,
		StaticPayload: q.StaticPayload, Design: nonNullJSON(q.Design, "{}"), DesignHash: hex.EncodeToString(q.DesignHash),
		FolderID: uuidPtr(q.FolderID), CampaignID: uuidPtr(q.CampaignID), TemplateID: uuidPtr(q.TemplateID),
		StartsAt: tsPtr(q.StartsAt), ExpiresAt: tsPtr(q.ExpiresAt), ScanLimit: q.ScanLimit,
		HasPassword: q.PasswordHash != nil && *q.PasswordHash != "", FallbackURL: q.FallbackUrl,
		TotalScans: q.TotalScans, UniqueScans: q.UniqueScans, LastScannedAt: tsPtr(q.LastScannedAt),
		CreatedAt: q.CreatedAt, UpdatedAt: q.UpdatedAt, ArchivedAt: tsPtr(q.ArchivedAt), DeletedAt: tsPtr(q.DeletedAt),
	}
	if len(q.StaticContent) > 0 {
		d.StaticContent = json.RawMessage(q.StaticContent)
	}
	if q.Mode == "dynamic" && q.ShortCode != nil && q.DomainID.Valid {
		host := s.hostForDomain(ctx, uuid.UUID(q.DomainID.Bytes))
		u := s.shortURL(host, *q.ShortCode)
		d.ShortURL = &u
		d.EncodedPayload = s.encodedPayload(host, *q.ShortCode)
		if q.ContentType == "gs1" && q.Gs1Gtin != nil {
			d.EncodedPayload = s.encodedPayload(host, "01/"+*q.Gs1Gtin)
		}
	} else if q.StaticPayload != nil {
		d.EncodedPayload = *q.StaticPayload
	}
	return d
}

func nonNullJSON(b []byte, def string) json.RawMessage {
	if len(b) == 0 || string(b) == "null" {
		return json.RawMessage(def)
	}
	return json.RawMessage(b)
}

type memberDTO struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

type inviteDTO struct {
	ID          uuid.UUID `json:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	InvitedBy   uuid.UUID `json:"invited_by"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func toInviteDTO(i dbgen.Invite) inviteDTO {
	return inviteDTO{ID: i.ID, WorkspaceID: i.WorkspaceID, Email: i.Email, Role: i.Role, InvitedBy: i.InvitedBy,
		ExpiresAt: i.ExpiresAt, CreatedAt: i.CreatedAt}
}

type tagRef struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Color string    `json:"color"`
}
