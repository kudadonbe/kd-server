package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kudadonbe/kd-server/internal/services"
)

// ingestFileRequest mirrors internal/http/ingest.go's ingestRequest so the
// same JSON file can be posted to the HTTP API or loaded by the TUI.
type ingestFileRequest struct {
	Source  ingestFileSource `json:"source"`
	Records []map[string]any `json:"records"`
}

type ingestFileSource struct {
	Slug string `json:"slug"`
	Name string `json:"name,omitempty"`
}

func (a *app) ingestResolveMenu() {
	tenant := a.requireTenant()
	if tenant == nil {
		return
	}

	for {
		fmt.Println()
		fmt.Printf("Ingest & Resolve (tenant: %s):\n", tenant.Slug)
		fmt.Println(" 1) Ingest records from JSON file")
		fmt.Println(" 2) Run resolve batch")
		fmt.Println(" 3) Back")
		fmt.Print("> ")

		choice, _ := a.reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		var err error
		switch choice {
		case "1":
			err = ingestFromFile(a.ctx, a.ingest, a.reader, tenant.Slug)
		case "2":
			err = runResolve(a.ctx, a.resolve, a.reader, tenant.Slug)
		case "3":
			return
		default:
			fmt.Println("Unknown option, please choose 1-3.")
			fmt.Println()
			continue
		}
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
		}
	}
}

func ingestFromFile(ctx context.Context, ingestService *services.IngestService, reader *bufio.Reader, tenantID string) error {
	path := prompt(reader, "Path to ingest JSON file")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	command, err := parseIngestFile(data)
	if err != nil {
		return err
	}

	result, err := ingestService.Ingest(ctx, tenantID, command)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Created: %d  Linked: %d  Needs review: %d\n\n", result.Created, result.Linked, result.NeedsReview)
	return nil
}

// parseIngestFile decodes an ingest JSON file into an IngestCommand.
func parseIngestFile(data []byte) (services.IngestCommand, error) {
	var req ingestFileRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return services.IngestCommand{}, fmt.Errorf("parse ingest file: %w", err)
	}

	return services.IngestCommand{
		Source: services.IngestSource{
			Slug: req.Source.Slug,
			Name: req.Source.Name,
		},
		Records: req.Records,
	}, nil
}

func runResolve(ctx context.Context, resolveService *services.ResolveService, reader *bufio.Reader, tenantID string) error {
	batchSizeStr := promptDefault(reader, "Batch size", "100")
	batchSize, err := strconv.Atoi(batchSizeStr)
	if err != nil || batchSize <= 0 {
		return fmt.Errorf("invalid batch size: %q", batchSizeStr)
	}

	result, err := resolveService.Resolve(ctx, tenantID, services.ResolveOptions{BatchSize: batchSize})
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Resolved: %d  Needs review: %d  Skipped: %d\n\n", result.Resolved, result.NeedsReview, result.Skipped)
	return nil
}
