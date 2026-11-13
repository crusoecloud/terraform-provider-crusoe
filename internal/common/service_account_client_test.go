package common

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	authv1 "github.com/crusoecloud/client-go/auth/v1"
)

// TestCtxWithRetryTransport_InstallsRetryTransportAsOAuthBase confirms client-go's
// NewClientSecretConfig picks up the client ctxWithRetryTransport installs as the Base
// transport beneath its OAuth2 transport.
func TestCtxWithRetryTransport_InstallsRetryTransportAsOAuthBase(t *testing.T) {
	ctx := ctxWithRetryTransport(context.Background())

	injected, ok := ctx.Value(oauth2.HTTPClient).(*http.Client)
	if !ok {
		t.Fatal("ctxWithRetryTransport() did not install an *http.Client under oauth2.HTTPClient")
	}

	cfg, err := authv1.NewClientSecretConfig(ctx, "client-id", "client-secret", "https://token.invalid/oauth2/token", "audience")
	if err != nil {
		t.Fatalf("NewClientSecretConfig() error = %v, want nil", err)
	}

	transport, ok := cfg.HTTPClient.Transport.(*oauth2.Transport)
	if !ok {
		t.Fatalf("cfg.HTTPClient.Transport is %T, want *oauth2.Transport", cfg.HTTPClient.Transport)
	}

	if transport.Base != injected.Transport {
		t.Error("NewClientSecretConfig's OAuth2 transport Base is not the retry transport ctxWithRetryTransport installed")
	}
}

// TestCtxWithRetryTransport_RetriesTransientFailures confirms the client ctxWithRetryTransport
// installs retries 500s before succeeding.
func TestCtxWithRetryTransport_RetriesTransientFailures(t *testing.T) {
	hits := 0
	const failuresBeforeSuccess = 2
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if hits <= failuresBeforeSuccess {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := ctxWithRetryTransport(context.Background())

	client, ok := ctx.Value(oauth2.HTTPClient).(*http.Client)
	if !ok {
		t.Fatal("ctxWithRetryTransport() did not install an *http.Client under oauth2.HTTPClient")
	}

	//nolint:noctx // exercising the installed *http.Client directly, not building a new request
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("client.Get() error = %v, want nil", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("final response status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if hits != failuresBeforeSuccess+1 {
		t.Errorf("server hits = %d, want %d (the %d failures plus the succeeding retry)",
			hits, failuresBeforeSuccess+1, failuresBeforeSuccess)
	}
}

// TestServiceAccountConfig_TokenRefreshFailsIfBuiltWithACancelledCtx confirms that building the
// service-account client with a ctx that gets cancelled (instead of context.Background()) fails
// the next token refresh, even though the resource call making it carries its own live ctx.
//
// expires_in is set below oauth2's 10s expiry delta so every Token() call refetches, reproducing
// the failure without waiting out a real token's lifetime.
func TestServiceAccountConfig_TokenRefreshFailsIfBuiltWithACancelledCtx(t *testing.T) {
	fetches := 0
	// A real TLS server, since NewClientSecretConfig rejects a non-https tokenURL.
	tokenServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test-token","token_type":"bearer","expires_in":1}`)
	}))
	defer tokenServer.Close()

	// Distinct from tokenServer, so fetches only counts token fetches, not resource calls.
	resourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer resourceServer.Close()

	configurePhaseCtx, cancelConfigurePhaseCtx := context.WithCancel(context.Background())
	ctx := context.WithValue(configurePhaseCtx, oauth2.HTTPClient, &http.Client{Transport: tokenServer.Client().Transport})
	cfg, err := authv1.NewClientSecretConfig(
		ctxWithRetryTransport(ctx), "client-id", "client-secret", tokenServer.URL, "audience")
	if err != nil {
		t.Fatalf("NewClientSecretConfig() error = %v, want nil", err)
	}

	doGet := func() error {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, resourceServer.URL, http.NoBody)
		if reqErr != nil {
			return reqErr
		}

		resp, doErr := cfg.HTTPClient.Do(req)
		if doErr != nil {
			return doErr
		}
		defer resp.Body.Close()

		return nil
	}

	if getErr := doGet(); getErr != nil {
		t.Fatalf("request while ctx is still live: error = %v, want nil", getErr)
	}
	if fetches != 1 {
		t.Fatalf("token fetches after the first request = %d, want 1", fetches)
	}

	cancelConfigurePhaseCtx() // simulates the Configure RPC returning

	// The cached token now counts as expired, so Token() must refetch using the captured,
	// now-cancelled configurePhaseCtx.
	err = doGet()
	if err == nil {
		t.Fatal("request after configurePhaseCtx was cancelled: error = nil, want a context-canceled error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("request after configurePhaseCtx was cancelled: error = %v, want it to wrap context.Canceled", err)
	}
	t.Logf("reproduced: request after ctx cancellation failed with: %v", err)
}

// TestServiceAccountConfig_TokenRefreshSucceedsWithBackgroundCtx is the same forced-refetch
// scenario as TestServiceAccountConfig_TokenRefreshFailsIfBuiltWithACancelledCtx, but built with
// context.Background() instead of a ctx that gets cancelled. Both requests must succeed.
func TestServiceAccountConfig_TokenRefreshSucceedsWithBackgroundCtx(t *testing.T) {
	fetches := 0
	tokenServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test-token","token_type":"bearer","expires_in":1}`)
	}))
	defer tokenServer.Close()

	resourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer resourceServer.Close()

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: tokenServer.Client().Transport})
	cfg, err := authv1.NewClientSecretConfig(
		ctxWithRetryTransport(ctx), "client-id", "client-secret", tokenServer.URL, "audience")
	if err != nil {
		t.Fatalf("NewClientSecretConfig() error = %v, want nil", err)
	}

	doGet := func() error {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, resourceServer.URL, http.NoBody)
		if reqErr != nil {
			return reqErr
		}

		resp, doErr := cfg.HTTPClient.Do(req)
		if doErr != nil {
			return doErr
		}
		defer resp.Body.Close()

		return nil
	}

	if getErr := doGet(); getErr != nil {
		t.Fatalf("first request: error = %v, want nil", getErr)
	}
	if getErr := doGet(); getErr != nil {
		t.Fatalf("second request (forces a token refetch): error = %v, want nil", getErr)
	}
	if fetches != 2 {
		t.Errorf("token fetches = %d, want 2", fetches)
	}
}

// TestCtxWithRetryTransport_PreservesCallerSuppliedClient confirms a ctx that already carries an
// oauth2.HTTPClient value is left untouched.
func TestCtxWithRetryTransport_PreservesCallerSuppliedClient(t *testing.T) {
	callerClient := &http.Client{}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, callerClient)

	got := ctxWithRetryTransport(ctx)

	gotClient, ok := got.Value(oauth2.HTTPClient).(*http.Client)
	if !ok || gotClient != callerClient {
		t.Error("ctxWithRetryTransport() replaced a caller-supplied oauth2.HTTPClient value, want it preserved unchanged")
	}
}
