package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/workspace"
)

const (
	HeaderIdempotencyKey = "Idempotency-Key"
	HeaderReplayed       = "Idempotent-Replayed"
	KeyRetentionDuration = 24 * time.Hour
)

type Store interface {
	GetIdempotencyKey(ctx context.Context, arg dbgen.GetIdempotencyKeyParams) (dbgen.IdempotencyKey, error)
	CreateIdempotencyKey(ctx context.Context, arg dbgen.CreateIdempotencyKeyParams) (dbgen.IdempotencyKey, error)
	SetIdempotencyResponse(ctx context.Context, arg dbgen.SetIdempotencyResponseParams) error
}

type inMemoryStore struct {
	entries map[string]*cachedEntry
}

type cachedEntry struct {
	workspaceID  uuid.UUID
	key          string
	method       string
	path         string
	requestHash  string
	statusCode   *int
	responseBody []byte
	expiresAt    time.Time
}

// NewInMemoryStore returns an in-memory implementation for testing.
func NewInMemoryStore() *inMemoryStore {
	return &inMemoryStore{entries: make(map[string]*cachedEntry)}
}

// Middleware enforces Idempotency-Key semantics for POST/PUT/PATCH inside a workspace.
//
//   - Keys are 8–128 characters and scoped to (workspace, principal): one caller can never
//     replay another caller's response.
//   - A reused key with a different method, path or body is rejected with 422.
//   - Responses with status >= 500 or 429 are not stored, so the client can retry.
//   - Expired keys are treated as absent.
func Middleware(q *dbgen.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
			default:
				next.ServeHTTP(w, r)
				return
			}
			raw := strings.TrimSpace(r.Header.Get(HeaderIdempotencyKey))
			if raw == "" {
				next.ServeHTTP(w, r)
				return
			}
			if len(raw) < 8 || len(raw) > 128 {
				apierr.Render(w, apierr.BadRequest("invalid_idempotency_key", "Idempotency-Key must be 8–128 characters"))
				return
			}
			ws, ok := workspace.GetWorkspace(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			principal := "anonymous"
			if p, ok := auth.GetPrincipal(r.Context()); ok {
				principal = p.UserID.String()
				if p.APIKeyID != uuid.Nil {
					principal = "key:" + p.APIKeyID.String()
				}
			}
			kh := sha256.Sum256([]byte(principal + "\x00" + raw))
			key := hex.EncodeToString(kh[:])

			var bodyBytes []byte
			if r.Body != nil {
				var err error
				bodyBytes, err = io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
				if err != nil {
					apierr.Render(w, apierr.BadRequest("invalid_body", "failed to read request body"))
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
			h := sha256.Sum256(bodyBytes)
			reqHash := h[:]
			ctx := r.Context()
			keyParams := dbgen.GetIdempotencyKeyParams{WorkspaceID: ws.ID, Key: key}

			existing, err := q.GetIdempotencyKey(ctx, keyParams)
			if err == nil && time.Now().After(existing.ExpiresAt) {
				_ = q.DeleteIdempotencyKey(ctx, dbgen.DeleteIdempotencyKeyParams{WorkspaceID: ws.ID, Key: key})
				err = pgx.ErrNoRows
			}
			if err == nil {
				if !bytes.Equal(existing.RequestHash, reqHash) || existing.Method != r.Method || existing.Path != r.URL.Path {
					apierr.Render(w, apierr.New(http.StatusUnprocessableEntity, "idempotency_key_reused",
						"Unprocessable Entity", "this Idempotency-Key was already used with a different request"))
					return
				}
				if existing.StatusCode == nil {
					apierr.Render(w, apierr.Conflict("request_in_flight", "a request with this Idempotency-Key is still processing"))
					return
				}
				w.Header().Set(HeaderReplayed, "true")
				status := int(*existing.StatusCode)
				// Bodies are stored as a JSON string so jsonb normalisation cannot reorder keys.
				var stored string
				if err := json.Unmarshal(existing.ResponseBody, &stored); err == nil {
					existing.ResponseBody = []byte(stored)
				}
				if status == http.StatusNoContent || len(existing.ResponseBody) == 0 || string(existing.ResponseBody) == "null" {
					w.WriteHeader(status)
					return
				}
				ct := "application/json"
				if status >= 400 {
					ct = "application/problem+json"
				}
				w.Header().Set("Content-Type", ct)
				w.WriteHeader(status)
				_, _ = w.Write(existing.ResponseBody)
				return
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				apierr.Render(w, apierr.Internal("idempotency store unavailable"))
				return
			}
			if _, err := q.CreateIdempotencyKey(ctx, dbgen.CreateIdempotencyKeyParams{
				WorkspaceID: ws.ID, Key: key, Method: r.Method, Path: r.URL.Path, RequestHash: reqHash,
				ExpiresAt: time.Now().UTC().Add(KeyRetentionDuration),
			}); err != nil {
				apierr.Render(w, apierr.Conflict("request_in_flight", "a request with this Idempotency-Key is still processing"))
				return
			}

			rec := httptest.NewRecorder()
			completed := false
			defer func() {
				// A panic or abandoned request must not leave the key stuck "in flight".
				if !completed {
					_ = q.DeleteIdempotencyKey(context.WithoutCancel(ctx), dbgen.DeleteIdempotencyKeyParams{WorkspaceID: ws.ID, Key: key})
				}
			}()
			next.ServeHTTP(rec, r)
			completed = true

			for k, v := range rec.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())

			saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if rec.Code >= 500 || rec.Code == http.StatusTooManyRequests {
				_ = q.DeleteIdempotencyKey(saveCtx, dbgen.DeleteIdempotencyKeyParams{WorkspaceID: ws.ID, Key: key})
				return
			}
			body := []byte("null")
			if b := rec.Body.Bytes(); len(bytes.TrimSpace(b)) > 0 {
				body, _ = json.Marshal(string(b))
			}
			status := int32(rec.Code)
			_ = q.SetIdempotencyResponse(saveCtx, dbgen.SetIdempotencyResponseParams{
				WorkspaceID: ws.ID, Key: key, StatusCode: &status, ResponseBody: body,
			})
		})
	}
}
