package zammadbridge

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient3CXPost20(t *testing.T, handler http.HandlerFunc) *Client3CXPost20 {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	config := new(Config)
	config.Phone3CX.Host = server.URL

	return &Client3CXPost20{Config: config}
}

func TestAuthenticateRetryStopsWhenContextIsCancelled(t *testing.T) {
	client := newTestClient3CXPost20(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := client.AuthenticateRetry(ctx, time.Minute)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected the context error, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("AuthenticateRetry kept waiting after the context was done")
	}
}

func TestFetchCallsStopsWhenContextIsCancelled(t *testing.T) {
	client := newTestClient3CXPost20(t, func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // 3CX hangs
	})

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.FetchCalls(ctx)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected the context error, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("FetchCalls kept waiting after the context was done")
	}
}
