package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const extractToolName = "record_fields"

// anthropicProvider calls the Anthropic (Claude) Messages API.
type anthropicProvider struct{}

func newAnthropicProvider() *anthropicProvider { return &anthropicProvider{} }

func (p *anthropicProvider) Name() string { return ProviderAnthropic }

// Validate makes a tiny completion to confirm the key is accepted. Output is
// discarded; only success/failure matters.
func (p *anthropicProvider) Validate(ctx context.Context, cred Credential) error {
	model := resolveModel(cred.Model, "")

	client := anthropic.NewClient(option.WithAPIKey(cred.APIKey))
	_, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 16,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("ping")),
		},
	})
	if err != nil {
		return classifyErr(err)
	}
	return nil
}

// Extract reads a document image/PDF and returns structured fields via a forced
// tool call whose input schema matches the requested fields.
func (p *anthropicProvider) Extract(ctx context.Context, cred Credential, req ExtractionRequest) (ExtractionResult, error) {
	model := resolveModel(req.Model, cred.Model)
	b64 := base64.StdEncoding.EncodeToString(req.Data)

	var mediaBlock anthropic.ContentBlockParamUnion
	if req.MediaType == "application/pdf" {
		mediaBlock = anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{Data: b64})
	} else {
		mediaBlock = anthropic.NewImageBlockBase64(req.MediaType, b64)
	}

	tool := anthropic.ToolParam{
		Name:        extractToolName,
		Description: anthropic.String("Record the fields extracted from the document. Leave a field empty if it is not present or not legible."),
		InputSchema: anthropic.ToolInputSchemaParam{Properties: req.Schema},
	}

	client := anthropic.NewClient(option.WithAPIKey(cred.APIKey))
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 2048,
		Tools:     []anthropic.ToolUnionParam{{OfTool: &tool}},
		ToolChoice: anthropic.ToolChoiceUnionParam{
			OfTool: &anthropic.ToolChoiceToolParam{Name: extractToolName},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(mediaBlock, anthropic.NewTextBlock(req.Instruction)),
		},
	})
	if err != nil {
		return ExtractionResult{}, classifyErr(err)
	}

	for _, block := range resp.Content {
		if tu, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
			var fields map[string]any
			if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &fields); err != nil {
				return ExtractionResult{}, fmt.Errorf("ai: parse extraction output: %w", err)
			}
			return ExtractionResult{Model: model, Fields: fields}, nil
		}
	}
	return ExtractionResult{}, errors.New("ai: model returned no structured output")
}

func resolveModel(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	if fallback != "" {
		return fallback
	}
	return DefaultModel
}

// classifyErr maps an auth failure to ErrInvalidKey; everything else is wrapped.
func classifyErr(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
		return ErrInvalidKey
	}
	return fmt.Errorf("ai: anthropic request failed: %w", err)
}
