package common

import (
	"errors"
	"testing"
)

func TestResolveAuthMethod(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		wantMethod AuthMethod
		wantErr    error
	}{
		{
			name:       "only API key configured",
			cfg:        Config{AccessKeyID: "key", SecretKey: "secret"},
			wantMethod: AuthMethodAPIKey,
		},
		{
			name:       "only service account configured",
			cfg:        Config{ServiceAccountClientID: "client-id", ServiceAccountClientSecret: "client-secret"},
			wantMethod: AuthMethodServiceAccount,
		},
		{
			name:    "nothing configured",
			cfg:     Config{},
			wantErr: ErrMissingCredentials,
		},
		{
			name:    "both fully configured is ambiguous",
			cfg:     Config{AccessKeyID: "key", SecretKey: "secret", ServiceAccountClientID: "client-id", ServiceAccountClientSecret: "client-secret"},
			wantErr: ErrConflictingCredentials,
		},
		{
			name:    "only access_key_id set, missing secret_key",
			cfg:     Config{AccessKeyID: "key"},
			wantErr: ErrIncompleteAPIKeyCredentials,
		},
		{
			name:    "only secret_key set, missing access_key_id",
			cfg:     Config{SecretKey: "secret"},
			wantErr: ErrIncompleteAPIKeyCredentials,
		},
		{
			name:    "only service_account_client_id set, missing secret",
			cfg:     Config{ServiceAccountClientID: "client-id"},
			wantErr: ErrIncompleteServiceAccountCredentials,
		},
		{
			name:    "only service_account_client_secret set, missing id",
			cfg:     Config{ServiceAccountClientSecret: "client-secret"},
			wantErr: ErrIncompleteServiceAccountCredentials,
		},
		{
			name: "partial API key plus partial service account is reported as incomplete API key first",
			// This documents current precedence: a partial API key pair is checked before a
			// partial service account pair, so this combination surfaces the API key error.
			cfg:     Config{AccessKeyID: "key", ServiceAccountClientID: "client-id"},
			wantErr: ErrIncompleteAPIKeyCredentials,
		},
		{
			name:       "complete service account plus unrelated empty API key fields",
			cfg:        Config{ServiceAccountClientID: "client-id", ServiceAccountClientSecret: "client-secret", AccessKeyID: "", SecretKey: ""},
			wantMethod: AuthMethodServiceAccount,
		},
		{
			name: "complete API key plus partial service account is reported as incomplete SA",
			// The likely real-world case of a user migrating from API key to service account
			// auth and not fully cleaning up the new pair yet.
			cfg:     Config{AccessKeyID: "key", SecretKey: "secret", ServiceAccountClientID: "client-id"},
			wantErr: ErrIncompleteServiceAccountCredentials,
		},
		{
			name: "complete service account plus partial API key is reported as incomplete API key",
			// The mirror case: migrating from service account to API key auth.
			cfg:     Config{ServiceAccountClientID: "client-id", ServiceAccountClientSecret: "client-secret", AccessKeyID: "key"},
			wantErr: ErrIncompleteAPIKeyCredentials,
		},
		{
			name: "service account with only token_url overridden",
			cfg: Config{ //nolint:gosec // G101: fake credentials in a test
				ServiceAccountClientID: "client-id", ServiceAccountClientSecret: "client-secret",
				ServiceAccountTokenURL: "https://auth.dev.crusoe.ai/oauth2/token",
				ServiceAccountAudience: defaultServiceAccountAudience,
			},
			wantErr: ErrMismatchedServiceAccountEnvironment,
		},
		{
			name: "service account with only audience overridden",
			cfg: Config{
				ServiceAccountClientID: "client-id", ServiceAccountClientSecret: "client-secret",
				ServiceAccountTokenURL: defaultServiceAccountTokenURL,
				ServiceAccountAudience: "https://api.dev.crusoe.ai",
			},
			wantErr: ErrMismatchedServiceAccountEnvironment,
		},
		{
			name:       "service account with token_url and audience both overridden together",
			cfg:        Config{ServiceAccountClientID: "client-id", ServiceAccountClientSecret: "client-secret", ServiceAccountTokenURL: "https://auth.dev.crusoe.ai/oauth2/token", ServiceAccountAudience: "https://api.dev.crusoe.ai"}, //nolint:gosec // G101: fake credentials in a test
			wantMethod: AuthMethodServiceAccount,
		},
		{
			name:       "service account with token_url and audience both left unset",
			cfg:        Config{ServiceAccountClientID: "client-id", ServiceAccountClientSecret: "client-secret"},
			wantMethod: AuthMethodServiceAccount,
		},
		{
			name: "service account with token_url and audience both explicitly set to their defaults",
			cfg: Config{
				ServiceAccountClientID: "client-id", ServiceAccountClientSecret: "client-secret",
				ServiceAccountTokenURL: defaultServiceAccountTokenURL,
				ServiceAccountAudience: defaultServiceAccountAudience,
			},
			wantMethod: AuthMethodServiceAccount,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotMethod, gotErr := ResolveAuthMethod(&tc.cfg)

			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("got err %v, want %v", gotErr, tc.wantErr)
				}
				if gotMethod != AuthMethodUnknown {
					t.Errorf("expected AuthMethodUnknown alongside an error, got %v", gotMethod)
				}

				return
			}

			if gotErr != nil {
				t.Fatalf("unexpected error: %v", gotErr)
			}
			if gotMethod != tc.wantMethod {
				t.Errorf("got method %v, want %v", gotMethod, tc.wantMethod)
			}
		})
	}
}
