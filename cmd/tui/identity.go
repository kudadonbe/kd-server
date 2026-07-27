package main

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/kudadonbe/kd-server/internal/services"
	"github.com/kudadonbe/kd-server/internal/store"
)

func (a *app) identityMenu() {
	tenant := a.requireTenant()
	if tenant == nil {
		return
	}

	for {
		fmt.Println()
		fmt.Printf("Identity Documents (tenant: %s):\n", tenant.Slug)
		fmt.Println(" 1) List documents")
		fmt.Println(" 2) View document")
		fmt.Println(" 3) Verify / reject document")
		fmt.Println(" 4) Back")
		fmt.Print("> ")

		choice, _ := a.reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		var err error
		switch choice {
		case "1":
			err = listIdentityDocuments(a.ctx, a.identity, a.reader, tenant.Slug)
		case "2":
			err = viewIdentityDocument(a.ctx, a.identity, a.reader, tenant.Slug)
		case "3":
			err = verifyIdentityDocument(a.ctx, a.identity, a.reader, tenant.Slug)
		case "4":
			return
		default:
			fmt.Println("Unknown option, please choose 1-4.")
			fmt.Println()
			continue
		}
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
		}
	}
}

func listIdentityDocuments(ctx context.Context, identityService *services.IdentityDocumentService, reader *bufio.Reader, tenantID string) error {
	personID := prompt(reader, "Person ID (blank = all)")
	limitStr := promptDefault(reader, "Limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return fmt.Errorf("invalid limit: %q", limitStr)
	}

	documents, err := identityService.List(ctx, tenantID, personID, limit)
	if err != nil {
		return err
	}
	if len(documents) == 0 {
		fmt.Println()
		fmt.Println("No identity documents found.")
		fmt.Println()
		return nil
	}

	printIdentityDocumentTable(documents)
	return nil
}

func printIdentityDocumentTable(documents []store.IdentityDocument) {
	fmt.Println()
	fmt.Printf("%-26s %-20s %-24s %-12s\n", "Document ID", "Person ID", "Name", "Status")
	fmt.Println(strings.Repeat("-", 90))
	for _, doc := range documents {
		name := doc.Name.English
		if name == "" {
			name = doc.Name.Dhivehi
		}
		fmt.Printf("%-26s %-20s %-24s %-12s\n", doc.DocumentID, doc.PersonID, name, doc.VerificationStatus)
	}
	fmt.Println()
}

// selectIdentityDocument lists a person's documents and lets the operator pick one by index.
func selectIdentityDocument(ctx context.Context, identityService *services.IdentityDocumentService, reader *bufio.Reader, tenantID string) (*store.IdentityDocument, error) {
	personID := prompt(reader, "Person ID (blank = all)")
	documents, err := identityService.List(ctx, tenantID, personID, 50)
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		fmt.Println()
		fmt.Println("No identity documents found.")
		fmt.Println()
		return nil, nil
	}

	fmt.Println()
	for idx, doc := range documents {
		name := doc.Name.English
		if name == "" {
			name = doc.Name.Dhivehi
		}
		fmt.Printf(" %d) %s - %s (%s)\n", idx+1, doc.DocumentID, name, doc.VerificationStatus)
	}
	fmt.Print("> ")

	selectionStr, _ := reader.ReadString('\n')
	selection, err := strconv.Atoi(strings.TrimSpace(selectionStr))
	if err != nil || selection < 1 || selection > len(documents) {
		return nil, fmt.Errorf("invalid selection")
	}
	return &documents[selection-1], nil
}

func viewIdentityDocument(ctx context.Context, identityService *services.IdentityDocumentService, reader *bufio.Reader, tenantID string) error {
	doc, err := selectIdentityDocument(ctx, identityService, reader, tenantID)
	if err != nil {
		return err
	}
	if doc == nil {
		return nil
	}

	fmt.Println()
	fmt.Printf("Document ID:   %s\n", doc.DocumentID)
	fmt.Printf("Person ID:     %s\n", doc.PersonID)
	fmt.Printf("Name (EN):     %s\n", doc.Name.English)
	fmt.Printf("Name (DV):     %s\n", doc.Name.Dhivehi)
	fmt.Printf("National ID:   %s\n", doc.NationalID)
	fmt.Printf("Sex:           %s\n", doc.Sex)
	fmt.Printf("Date of Birth: %s\n", doc.DateOfBirth)
	fmt.Printf("Expiry Date:   %s\n", doc.ExpiryDate)
	fmt.Printf("Status:        %s\n", doc.VerificationStatus)
	fmt.Printf("Verified By:   %s\n", doc.VerifiedBy)
	fmt.Printf("Source:        %s\n", doc.Source)
	fmt.Println()
	return nil
}

func verifyIdentityDocument(ctx context.Context, identityService *services.IdentityDocumentService, reader *bufio.Reader, tenantID string) error {
	doc, err := selectIdentityDocument(ctx, identityService, reader, tenantID)
	if err != nil {
		return err
	}
	if doc == nil {
		return nil
	}

	statusInput := promptDefault(reader, "New status (verified/rejected)", services.VerificationStatusVerified)
	var status string
	switch strings.ToLower(strings.TrimSpace(statusInput)) {
	case "verified", "verify":
		status = services.VerificationStatusVerified
	case "rejected", "reject":
		status = services.VerificationStatusRejected
	default:
		return fmt.Errorf("status must be 'verified' or 'rejected', got %q", statusInput)
	}

	verifiedBy := promptDefault(reader, "Verified by", "tui-admin")

	updated := *doc
	updated.VerificationStatus = status
	updated.VerifiedBy = verifiedBy

	saved, err := identityService.Save(ctx, updated, verifiedBy)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Document %s marked as %s.\n\n", saved.DocumentID, saved.VerificationStatus)
	return nil
}
