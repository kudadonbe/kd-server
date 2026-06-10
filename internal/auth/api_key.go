package auth

import "context"

// APIKeyVerifier validates tenant-scoped API keys.
type APIKeyVerifier interface {
	VerifyAPIKey(ctx context.Context, tenantID, apiKey string) error
}
