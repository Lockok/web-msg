package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	var contextID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestID(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	headerID := rec.Header().Get(RequestIDHeader)

	if headerID == "" {
		t.Fatal("expected request ID in response header")
	}

	if contextID == "" {
		t.Fatal("expected request ID in request context")
	}

	if headerID != contextID {
		t.Fatalf("header ID %q does not match context ID %q", headerID, contextID)
	}
}

func TestRequestIDIsUnique(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRecorder()
	second := httptest.NewRecorder()

	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))

	firstID := first.Header().Get(RequestIDHeader)
	secondID := second.Header().Get(RequestIDHeader)

	if firstID == secondID {
		t.Fatalf("expected unique request IDs, got %q twice", firstID)
	}
}
