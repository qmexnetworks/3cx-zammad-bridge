package zammadbridge

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestAuthenticateKeepsClientSecretOutOfErrors(t *testing.T) {
	var gotSecret string
	client := newTestClient3CXPost20(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		gotSecret = r.PostForm.Get("client_secret")
		if r.URL.RawQuery != "" {
			t.Errorf("credentials must not be in the URL, got query %q", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusBadGateway)
	})
	client.Config.Phone3CX.ClientID = "bridge"
	client.Config.Phone3CX.ClientSecret = "s3cr3t"

	err := client.Authenticate(t.Context())
	if err == nil {
		t.Fatal("expected an error")
	}
	if gotSecret != "s3cr3t" {
		t.Errorf("expected the secret in the POST body, got %q", gotSecret)
	}

	// A transport error (3CX unreachable) includes the URL, which must not carry the secret either.
	client.Config.Phone3CX.Host = "http://127.0.0.1:9"
	err = client.Authenticate(t.Context())
	if err == nil || strings.Contains(err.Error(), "s3cr3t") {
		t.Errorf("expected an error without the secret, got %v", err)
	}
}
