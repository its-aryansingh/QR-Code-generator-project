package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/apierr"
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

// Middleware creates a Chi HTTP middleware that enforces Idempotency-Key semantics.
func Middleware(q *dbgen.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Idempotency applies to non-safe methods (POST, PUT, PATCH)
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodDelete:
				next.ServeHTTP(w, r)
				return
			}

			key := strings.TrimSpace(r.Header.Get(HeaderIdempotencyKey))
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			if len(key) > 128 {
				apierr.Render(w, apierr.BadRequest("invalid_idempotency_key", "Idempotency-Key header must not exceed 128 characters"))
				return
			}

			ws, ok := workspace.GetWorkspace(r.Context())
			if !ok {
				// No workspace tenant context, proceed without idempotency
				next.ServeHTTP(w, r)
				return
			}

			// Read and hash request body
			var bodyBytes []byte
			if r.Body != nil {
				var err error
				bodyBytes, err = io.ReadAll(r.Body)
				if err != nil {
					apierr.Render(w, apierr.BadRequest("invalid_body", "failed to read request body"))
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			h := sha256.Sum256(bodyBytes)
			reqHash := h[:]

			ctx := r.Context()
			existing, err := q.GetIdempotencyKey(ctx, dbgen.GetIdempotencyKeyParams{
				WorkspaceID: ws.ID,
				Key:         key,
			})

			if err == nil {
				// Key exists
				if existing.StatusCode == nil {
					// In-flight request
					apierr.Render(w, apierr.Conflict("request_in_flight", "a request with this idempotency key is currently processing"))
					return
				}

				// Check request hash
				if !bytes.Equal(existing.RequestHash, reqHash) || existing.Method != r.Method || existing.Path != r.URL.Path {
					apierr.Render(w, apierr.Conflict("idempotency_conflict", "idempotency key reused with different request payload"))
					return
				}

				// Replay cached response
				w.Header().Set(HeaderReplayed, "true")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(int(*existing.StatusCode))
				_, _ = w.Write(existing.ResponseBody)
				return
			}

			// Insert in-flight record
			expiresAt := time.Now().UTC().Add(KeyRetentionDuration)
			_, err = q.CreateIdempotencyKey(ctx, dbgen.CreateIdempotencyKeyParams{
				WorkspaceID: ws.ID,
				Key:         key,
				Method:      r.Method,
				Path:        r.URL.Path,
				RequestHash: reqHash,
				ExpiresAt:   expiresAt,
			})
			if err != nil {
				// Another worker inserted it concurrently
				apierr.Render(w, apierr.Conflict("request_in_flight", "a request with this idempotency key is currently processing"))
				return
			}

			// Intercept and record the response
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)

			// Copy response headers and write to client
			for k, v := range rec.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())

			// Save response in idempotency record
			statusCode := int32(rec.Code)
			_ = q.SetIdempotencyResponse(ctx, dbgen.SetIdempotencyResponseParams{
				WorkspaceID:  ws.ID,
				Key:          key,
				StatusCode:   &statusCode,
				ResponseBody: rec.Body.Bytes(),
			})
		})
	}
}
