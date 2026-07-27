package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/services"
)

func (a *app) tenantMenu() {
	for {
		fmt.Println()
		fmt.Println("Tenant & API Key Management:")
		fmt.Println(" 1) List tenants")
		fmt.Println(" 2) Create tenant")
		fmt.Println(" 3) Edit tenant")
		fmt.Println(" 4) Issue API key")
		fmt.Println(" 5) Revoke API key")
		fmt.Println(" 6) Back")
		fmt.Print("> ")

		choice, _ := a.reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		var err error
		switch choice {
		case "1":
			err = listTenants(a.ctx, a.admin)
		case "2":
			err = createTenant(a.ctx, a.admin, a.reader)
		case "3":
			err = editTenant(a.ctx, a.admin, a.reader)
		case "4":
			err = issueKey(a.ctx, a.admin, a.reader)
		case "5":
			err = revokeKey(a.ctx, a.admin, a.reader)
		case "6":
			return
		default:
			fmt.Println("Unknown option, please choose 1-6.")
			fmt.Println()
			continue
		}
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
		}
	}
}

func listTenants(ctx context.Context, adminService *services.AdminService) error {
	tenants, err := adminService.ListTenants(ctx)
	if err != nil {
		return err
	}
	if len(tenants) == 0 {
		fmt.Println()
		fmt.Println("No tenants found.")
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Printf("%-4s %-20s %-25s %-20s\n", "#", "Slug", "Name", "Created At")
	fmt.Println(strings.Repeat("-", 70))
	for idx, tenant := range tenants {
		fmt.Printf(
			"%-4d %-20s %-25s %-20s\n",
			idx+1,
			tenant.Slug,
			tenant.Name,
			tenant.CreatedAt.Format(time.RFC3339),
		)
	}
	fmt.Println()
	return nil
}

func createTenant(ctx context.Context, adminService *services.AdminService, reader *bufio.Reader) error {
	fmt.Println()
	fmt.Println("Create Tenant")
	name := prompt(reader, "Friendly name (e.g. Acme Co)")
	defaultSlug := slugify(name)
	slug := promptDefault(reader, "Slug (lowercase, no spaces)", defaultSlug)

	tenant, err := adminService.CreateTenant(ctx, slug, name)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Tenant created: %s (%s)\n", tenant.Name, tenant.Slug)
	fmt.Println()
	return nil
}

func editTenant(ctx context.Context, adminService *services.AdminService, reader *bufio.Reader) error {
	tenant, err := selectTenant(ctx, adminService, reader, "Select tenant to edit:")
	if err != nil {
		return err
	}
	if tenant == nil {
		return nil
	}

	fmt.Printf("Slug: %s (permanent)\n", tenant.Slug)
	name := promptDefault(reader, "Display name", tenant.Name)
	updated, err := adminService.UpdateTenantName(ctx, tenant.Slug, name)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Tenant updated: %s (%s)\n", updated.Name, updated.Slug)
	fmt.Println("Existing API keys remain valid.")
	fmt.Println()
	return nil
}

func issueKey(ctx context.Context, adminService *services.AdminService, reader *bufio.Reader) error {
	tenant, err := selectTenant(ctx, adminService, reader, "Select tenant for API key:")
	if err != nil {
		return err
	}
	if tenant == nil {
		return nil
	}

	label := promptDefault(reader, "Label (optional)", fmt.Sprintf("%s key", tenant.Name))

	issued, err := adminService.IssueAPIKey(ctx, tenant.Slug, label)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("API Key issued!")
	fmt.Printf("Tenant: %s (%s)\n", tenant.Name, issued.Tenant)
	fmt.Printf("Key ID: %s\n", issued.KeyID)
	fmt.Printf("Label: %s\n", issued.Label)
	fmt.Println()
	fmt.Println("Secret (store this securely, it will not be shown again):")
	fmt.Println(issued.Secret)

	fmt.Println()
	fmt.Println(".env snippet:")
	fmt.Printf("KD_TENANT=%s\n", issued.Tenant)
	fmt.Printf("KD_API_KEY=%s\n", issued.Secret)
	fmt.Println()
	return nil
}

func revokeKey(ctx context.Context, adminService *services.AdminService, reader *bufio.Reader) error {
	tenant, err := selectTenant(ctx, adminService, reader, "Select tenant for API key revocation:")
	if err != nil {
		return err
	}
	if tenant == nil {
		return nil
	}

	keyID := prompt(reader, "Key ID (for example key_abc123)")
	if err := adminService.RevokeAPIKey(ctx, tenant.Slug, keyID); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("API key %s revoked.\n\n", keyID)
	return nil
}

func selectTenant(ctx context.Context, adminService *services.AdminService, reader *bufio.Reader, heading string) (*services.TenantSummary, error) {
	tenants, err := adminService.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	if len(tenants) == 0 {
		fmt.Println()
		fmt.Println("No tenants available. Create a tenant first.")
		fmt.Println()
		return nil, nil
	}

	fmt.Println()
	fmt.Println(heading)
	for idx, tenant := range tenants {
		fmt.Printf(" %d) %s (%s)\n", idx+1, tenant.Name, tenant.Slug)
	}
	fmt.Print("> ")

	selectionStr, _ := reader.ReadString('\n')
	selection, err := strconv.Atoi(strings.TrimSpace(selectionStr))
	if err != nil || selection < 1 || selection > len(tenants) {
		return nil, errors.New("invalid selection")
	}

	return &tenants[selection-1], nil
}

func slugify(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	input = strings.ReplaceAll(input, " ", "-")
	filtered := make([]rune, 0, len(input))
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			filtered = append(filtered, r)
		}
	}
	return string(filtered)
}
