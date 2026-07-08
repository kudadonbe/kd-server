package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// anthropicProvider calls the Anthropic (Claude) Messages API.
type anthropicProvider struct{}

func newAnthropicProvider() *anthropicProvider { return &anthropicProvider{} }

func (p *anthropicProvider) Name() string { return ProviderAnthropic }

// Validate makes a tiny completion to confirm the key is accepted. Output is
// discarded; only success/failure matters.
func (p *anthropicProvider) Validate(ctx context.Context, cred Credential) error {
	model := cred.Model
	if model == "" {
		model = DefaultModel
	}

	client := anthropic.NewClient(option.WithAPIKey(cred.APIKey))
	_, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 16,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("ping")),
		},
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
			return ErrInvalidKey
		}
		return fmt.Errorf("ai: validate anthropic key: %w", err)
	}
	return nil
}
