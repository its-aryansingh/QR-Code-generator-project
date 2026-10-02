package idempotency_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/its-aryansingh/qrit/services/internal/idempotency"
)

func TestIdempotencyKeyLength(t *testing.T) {
	// A key > 128 chars should be rejected
	longKey := strings.Repeat("a", 129)
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(`{"data":1}`))
	req.Header.Set(idempotency.HeaderIdempotencyKey, longKey)
	w := httptest.NewRecorder()

	handler := idempotency.Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for key > 128 chars, got %d", w.Code)
	}
}

func TestSafeMethodsBypass(t *testing.T) {
	methods := []string{"GET", "HEAD", "OPTIONS", "DELETE"}
	for _, m := range methods {
		req := httptest.NewRequest(m, "/test", nil)
		req.Header.Set(idempotency.HeaderIdempotencyKey, "test-key-1")
		w := httptest.NewRecorder()

		handler := idempotency.Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("method %s should bypass idempotency, got %d", m, w.Code)
		}
	}
}
