package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kudadonbe/kd-server/internal/services"
	"go.mongodb.org/mongo-driver/mongo"
)

func (a *app) lookupMenu() {
	tenant := a.requireTenant()
	if tenant == nil {
		return
	}

	for {
		fmt.Println()
		fmt.Printf("Lookup (tenant: %s):\n", tenant.Slug)
		fmt.Println(" 1) Lookup by email")
		fmt.Println(" 2) Lookup by phone")
		fmt.Println(" 3) Back")
		fmt.Print("> ")

		choice, _ := a.reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		var err error
		switch choice {
		case "1":
			err = lookupByEmail(a.ctx, a.lookup, a.reader, tenant.Slug)
		case "2":
			err = lookupByPhone(a.ctx, a.lookup, a.reader, tenant.Slug)
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

func lookupByEmail(ctx context.Context, lookupService *services.LookupService, reader *bufio.Reader, tenantID string) error {
	email := prompt(reader, "Email")
	view, err := lookupService.ByEmail(ctx, tenantID, email)
	return printPersonView(view, err)
}

func lookupByPhone(ctx context.Context, lookupService *services.LookupService, reader *bufio.Reader, tenantID string) error {
	phone := prompt(reader, "Phone (E.164 format)")
	view, err := lookupService.ByPhone(ctx, tenantID, phone)
	return printPersonView(view, err)
}

func printPersonView(view *services.PersonView, err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		fmt.Println()
		fmt.Println("No matching person found.")
		fmt.Println()
		return nil
	}
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Person ID:     %s\n", view.Person.PersonID)
	fmt.Printf("National ID:   %s\n", view.Person.NationalID)
	fmt.Printf("Primary Email: %s\n", view.Person.PrimaryEmail)
	fmt.Printf("Primary Phone: %s\n", view.Person.PrimaryPhone)
	fmt.Printf("Created At:    %s\n", view.Person.CreatedAt)
	fmt.Printf("Updated At:    %s\n", view.Person.UpdatedAt)

	if len(view.Links) == 0 {
		fmt.Println("Links: none")
	} else {
		fmt.Println("Links:")
		for _, link := range view.Links {
			fmt.Printf("  - %s / %s\n", link.Source, link.ExternalID)
		}
	}
	fmt.Println()
	return nil
}
