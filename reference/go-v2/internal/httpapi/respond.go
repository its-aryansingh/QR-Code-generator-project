package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/platform/httpx"
)

const maxBodyBytes = 1 << 20 // 1 MiB for JSON bodies

func writeJSON(w http.ResponseWriter, status int, v any) { httpx.JSON(w, status, v) }

func fail(w http.ResponseWriter, err error) {
	var pd *apierr.ProblemDetails
	if !errors.As(err, &pd) && err != nil {
		slog.Error("unhandled error", "error", err)
	}
	apierr.Render(w, err)
}

// decode reads a JSON body into dst with a size limit.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			fail(w, apierr.BadRequest("empty_body", "request body is required"))
			return false
		}
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			fail(w, apierr.New(http.StatusRequestEntityTooLarge, "body_too_large", "Payload Too Large", "request body exceeds 1 MiB"))
			return false
		}
		fail(w, apierr.BadRequest("invalid_json", "malformed JSON body: "+err.Error()))
		return false
	}
	return true
}

// decodeOptional is decode but treats an empty body as {}.
func decodeOptional(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.ContentLength == 0 {
		return true
	}
	return decode(w, r, dst)
}

func uuidParam(r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	return id, err == nil
}

func unprocessable(code, detail string, fields ...apierr.FieldError) *apierr.ProblemDetails {
	p := apierr.New(http.StatusUnprocessableEntity, code, "Unprocessable Entity", detail)
	p.Errors = fields
	return p
}

func forbidden(code, detail string) *apierr.ProblemDetails {
	return apierr.New(http.StatusForbidden, code, "Forbidden", detail)
}

func paymentRequired(code, detail string) *apierr.ProblemDetails {
	return apierr.New(http.StatusPaymentRequired, code, "Payment Required", detail)
}

func tooMany(detail string, retry time.Duration) *apierr.ProblemDetails {
	p := apierr.New(http.StatusTooManyRequests, "rate_limited", "Too Many Requests", detail)
	if retry > 0 {
		p.RetryAfterSeconds = int(retry.Seconds()) + 1
	}
	return p
}

func isNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// pgCode returns the Postgres SQLSTATE of err, or "".
func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

func pgConstraint(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.ConstraintName
	}
	return ""
}

const (
	sqlUniqueViolation = "23505"
	sqlCheckViolation  = "23514"
	sqlFKViolation     = "23503"
)

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func pgUUIDPtr(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func uuidPtr(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	return &id
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

func pgTS(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Cursor encodes (created_at, id) for keyset pagination.
type cursor struct {
	T  time.Time `json:"t"`
	ID uuid.UUID `json:"i"`
}

func encodeCursor(t time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(cursor{T: t, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (*cursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var c cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func limitParam(r *http.Request, def, max int) int {
	n, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

type page[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

func trimLower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func errf(format string, a ...any) error { return fmt.Errorf(format, a...) }
