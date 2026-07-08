package ai

import "context"

// globalScope is the tenant sentinel under which the admin-managed default
// credential is stored. Underscores make it impossible to collide with a real
// tenant slug (slugs are lowercase alphanumeric/hyphen).
const globalScope = "__server_default__"

// DefaultStatus describes the server-wide (admin-managed) AI pairing for the
// admin console. It never contains the API key.
type DefaultStatus struct {
	Configured bool   `json:"configured"`
	Paired     bool   `json:"paired"`
	Provider   string `json:"provider"`
	Model      string `json:"model,omitempty"`
	KeyHint    string `json:"key_hint,omitempty"`
	Source     string `json:"source"` // "stored" | "env" | "none"
}

// DefaultStatus reports whether a server-wide default key is available and where
// it comes from (an admin-paired stored key, the ANTHROPIC_API_KEY env fallback,
// or none).
func (s *Service) DefaultStatus(ctx context.Context) (DefaultStatus, error) {
	provider := s.provider.Name()
	status := DefaultStatus{Configured: true, Provider: provider, Model: s.defaultModel, Source: "none"}

	if s.encryptionEnabled() {
		stored, err := s.store.GetAICredential(ctx, globalScope, provider)
		if err != nil {
			return status, err
		}
		if stored != nil {
			status.Paired = true
			status.Source = "stored"
			status.KeyHint = stored.KeyHint
			if stored.Model != "" {
				status.Model = stored.Model
			}
			return status, nil
		}
	}
	if s.defaultKey != "" {
		status.Paired = true
		status.Source = "env"
		status.KeyHint = maskKey(s.defaultKey)
	}
	return status, nil
}

// SetDefault validates and stores the admin-managed default credential.
func (s *Service) SetDefault(ctx context.Context, apiKey, model string) (CredentialMeta, error) {
	return s.ValidateAndStore(ctx, globalScope, apiKey, model)
}

// DeleteDefault removes the admin-managed default credential (the env fallback,
// if any, still applies).
func (s *Service) DeleteDefault(ctx context.Context) error {
	return s.store.DeleteAICredential(ctx, globalScope, s.provider.Name())
}
