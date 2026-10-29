package common

import (
	"errors"
	"os"
)

// AuthMethod identifies which credential pair should be used to authenticate the Crusoe API client.
type AuthMethod int

const (
	// AuthMethodUnknown is the zero value, returned alongside an error when credentials could not
	// be resolved to a single authentication method.
	AuthMethodUnknown AuthMethod = iota
	// AuthMethodAPIKey authenticates via the legacy HMAC access_key_id/secret_key pair.
	AuthMethodAPIKey
	// AuthMethodClientSecret authenticates via OAuth2 client_credentials using
	// client_id/client_secret.
	AuthMethodClientSecret
)

var (
	// ErrMissingCredentials is returned when neither credential pair is configured at all.
	ErrMissingCredentials = errors.New("no Crusoe credentials configured")

	// ErrIncompleteAPIKeyCredentials is returned when exactly one of access_key_id/secret_key is set.
	ErrIncompleteAPIKeyCredentials = errors.New("access_key_id and secret_key must both be set")

	// ErrIncompleteClientSecretCredentials is returned when exactly one of
	// client_id/client_secret is set.
	ErrIncompleteClientSecretCredentials = errors.New(
		"client_id and client_secret must both be set")

	// ErrConflictingCredentials is returned when more than one credential pair is fully configured
	// simultaneously. The provider refuses to silently pick one, since that could authenticate as
	// the wrong identity without the user noticing.
	ErrConflictingCredentials = errors.New(
		"multiple credential pairs are fully configured; configure only one authentication method")

	// ErrMismatchedClientSecretEnvironment is returned when exactly one of
	// token_url/audience is overridden from its default. The two default to prod values
	// independently, so overriding only one silently keeps the other pointed at prod, minting a
	// token with the wrong audience for that token endpoint.
	ErrMismatchedClientSecretEnvironment = errors.New(
		"token_url and audience must both be set together when overriding either from its default")
)

// authCandidate bundles everything credential resolution needs to know about one credential
// type, so adding a new type only means adding a new entry here - every function that loops
// over authCandidates (ResolveAuthMethod, isCompleteCredential, hasAnyCredentialField,
// envConfigFromEnviron, GetConfigWithOptions' profile parsing, applyCredential, mergeAncillary)
// picks it up automatically.
type authCandidate struct {
	method        AuthMethod
	complete      func(cfg *Config) bool
	partial       func(cfg *Config) bool
	incompleteErr error
	validate      func(cfg *Config) error

	// fromEnv populates cfg's fields for this credential type from CRUSOE_* environment variables.
	fromEnv func(cfg *Config)
	// fromProfile populates cfg's fields for this credential type from a decoded TOML profile table.
	fromProfile func(cfg *Config, valMap map[string]interface{})
	// apply copies this credential type's identity fields (not ancillary ones - see mergeAncillary)
	// from source onto cfg.
	apply func(cfg, source *Config)
	// mergeAncillary overwrites cfg's non-identity fields for this type with override's non-empty
	// ones. nil for types with no ancillary fields (e.g. APIKey).
	mergeAncillary func(cfg, override *Config)
}

//nolint:gochecknoglobals // static method registry, never mutated
var authCandidates = []authCandidate{
	{
		method:        AuthMethodAPIKey,
		complete:      isCompleteAPIKey,
		partial:       isPartialAPIKey,
		incompleteErr: ErrIncompleteAPIKeyCredentials,
		fromEnv: func(cfg *Config) {
			cfg.AccessKeyID = os.Getenv("CRUSOE_ACCESS_KEY_ID")
			cfg.SecretKey = os.Getenv("CRUSOE_SECRET_KEY")
		},
		fromProfile: func(cfg *Config, valMap map[string]interface{}) {
			if v, ok := valMap["access_key_id"].(string); ok {
				cfg.AccessKeyID = v
			}
			if v, ok := valMap["secret_key"].(string); ok {
				cfg.SecretKey = v
			}
		},
		apply: func(cfg, source *Config) {
			cfg.AccessKeyID = source.AccessKeyID
			cfg.SecretKey = source.SecretKey
		},
	},
	{
		method:        AuthMethodClientSecret,
		complete:      isCompleteClientSecret,
		partial:       isPartialClientSecret,
		incompleteErr: ErrIncompleteClientSecretCredentials,
		validate:      validateClientSecretEnvironment,
		fromEnv: func(cfg *Config) {
			cfg.ClientID = os.Getenv("CRUSOE_CLIENT_ID")
			cfg.ClientSecret = os.Getenv("CRUSOE_CLIENT_SECRET")
			cfg.TokenURL = os.Getenv("CRUSOE_TOKEN_URL")
			cfg.Audience = os.Getenv("CRUSOE_AUDIENCE")
		},
		fromProfile: func(cfg *Config, valMap map[string]interface{}) {
			if v, ok := valMap["client_id"].(string); ok {
				cfg.ClientID = v
			}
			if v, ok := valMap["client_secret"].(string); ok {
				cfg.ClientSecret = v
			}
			if v, ok := valMap["token_url"].(string); ok {
				cfg.TokenURL = v
			}
			if v, ok := valMap["audience"].(string); ok {
				cfg.Audience = v
			}
		},
		apply: func(cfg, source *Config) {
			cfg.ClientID = source.ClientID
			cfg.ClientSecret = source.ClientSecret
		},
		mergeAncillary: func(cfg, override *Config) {
			if override.TokenURL != "" {
				cfg.TokenURL = override.TokenURL
			}
			if override.Audience != "" {
				cfg.Audience = override.Audience
			}
		},
	},
}

func isCompleteAPIKey(cfg *Config) bool {
	return cfg.AccessKeyID != "" && cfg.SecretKey != ""
}

func isPartialAPIKey(cfg *Config) bool {
	hasAny := cfg.AccessKeyID != "" || cfg.SecretKey != ""

	return hasAny && !isCompleteAPIKey(cfg)
}

func isCompleteClientSecret(cfg *Config) bool {
	return cfg.ClientID != "" && cfg.ClientSecret != ""
}

func isPartialClientSecret(cfg *Config) bool {
	hasAny := cfg.ClientID != "" || cfg.ClientSecret != ""

	return hasAny && !isCompleteClientSecret(cfg)
}

// ResolveAuthMethod inspects a Config and determines which authentication method (if any) should
// be used to construct the Crusoe API client.
//
// Precedence / validation rules:
//   - More than one method fully configured   -> ErrConflictingCredentials (ambiguous; refuse to guess).
//   - Any method partially configured          -> that method's own incomplete-credential error.
//   - Exactly one method fully configured      -> that method, after running its own validate check.
//   - No method configured at all              -> ErrMissingCredentials.
//
// Partial-credential checks intentionally take priority over "nothing configured", so a likely
// typo (e.g. only secret_key set) is reported precisely instead of being reported as missing - and
// take priority even when a different method is already complete (e.g. a complete API key pair
// plus a stray, partial client-secret field is still reported as the incomplete client secret,
// not silently accepted as the API key).
func ResolveAuthMethod(cfg *Config) (AuthMethod, error) {
	complete := make([]authCandidate, 0, len(authCandidates))
	for _, c := range authCandidates {
		if c.complete(cfg) {
			complete = append(complete, c)
		}
	}

	if len(complete) > 1 {
		return AuthMethodUnknown, ErrConflictingCredentials
	}

	for _, c := range authCandidates {
		if c.partial != nil && c.partial(cfg) {
			return AuthMethodUnknown, c.incompleteErr
		}
	}

	if len(complete) == 1 {
		c := complete[0]
		if c.validate != nil {
			if err := c.validate(cfg); err != nil {
				return AuthMethodUnknown, err
			}
		}

		return c.method, nil
	}

	return AuthMethodUnknown, ErrMissingCredentials
}

// validateClientSecretEnvironment ensures token_url and audience are overridden together, not
// independently - see ErrMismatchedClientSecretEnvironment. An empty field means "not
// configured" rather than "overridden to empty", so it's treated as the default, not a mismatch.
func validateClientSecretEnvironment(cfg *Config) error {
	hasCustomTokenURL := cfg.TokenURL != "" && cfg.TokenURL != defaultTokenURL
	hasCustomAudience := cfg.Audience != "" && cfg.Audience != defaultAudience

	if hasCustomTokenURL != hasCustomAudience {
		return ErrMismatchedClientSecretEnvironment
	}

	return nil
}

// isCompleteCredential reports whether cfg has a complete credential pair for any known method,
// via the same authCandidates table ResolveAuthMethod uses - so a new method added there doesn't
// require a matching edit here.
func isCompleteCredential(cfg *Config) bool {
	for _, c := range authCandidates {
		if c.complete(cfg) {
			return true
		}
	}

	return false
}

// hasAnyCredentialField reports whether cfg has any credential field set, complete or not, via
// the same authCandidates table isCompleteCredential uses - so a new method added there doesn't
// require a matching edit here. Used only by pickCredentialTier's no-complete-tier fallback.
func hasAnyCredentialField(cfg *Config) bool {
	for _, c := range authCandidates {
		if c.complete(cfg) || (c.partial != nil && c.partial(cfg)) {
			return true
		}
	}

	return false
}

// pickCredentialTier returns the first tier, in the given highest-to-lowest precedence order,
// that carries a complete credential pair, used as-is.
//
// If no tier is complete, returns the first tier (still highest-to-lowest) that has any
// credential field set at all, again used as-is - never combined with another tier's fields.
// Stitching together e.g. an AccessKeyID from one tier and a SecretKey from a different tier
// could silently assemble a "complete" pair out of two unrelated tiers, so the
// incomplete-credential error this produces must reflect one real tier's actual state, not a
// composite of several.
func pickCredentialTier(tiers ...*Config) *Config {
	for _, tier := range tiers {
		if isCompleteCredential(tier) {
			return tier
		}
	}

	for _, tier := range tiers {
		if hasAnyCredentialField(tier) {
			return tier
		}
	}

	return &Config{}
}

// applyCredential copies source's identity fields onto cfg unconditionally, via the same
// authCandidates table ResolveAuthMethod uses - so a new method added there doesn't require a
// matching edit here. ClientSecret's TokenURL/Audience are deliberately left untouched here even
// though they live on the same struct - callers apply those via mergeAncillary instead, which
// falls back to a default rather than copying blindly.
func applyCredential(cfg, source *Config) {
	for _, c := range authCandidates {
		c.apply(cfg, source)
	}
}

// mergeAncillary overwrites cfg's non-identity fields (e.g. TokenURL/Audience) with override's
// non-empty ones, via the same authCandidates table ResolveAuthMethod uses - so a new method
// added there doesn't require a matching edit here, unless it has no ancillary fields at all
// (e.g. APIKey), in which case it simply has no mergeAncillary to run.
func mergeAncillary(cfg, override *Config) {
	for _, c := range authCandidates {
		if c.mergeAncillary != nil {
			c.mergeAncillary(cfg, override)
		}
	}
}
