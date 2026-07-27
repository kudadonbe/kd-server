package main

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/kudadonbe/kd-server/internal/services"
)

func (a *app) reviewMenu() {
	tenant := a.requireTenant()
	if tenant == nil {
		return
	}

	for {
		fmt.Println()
		fmt.Printf("Review Queue (tenant: %s):\n", tenant.Slug)
		fmt.Println(" 1) List needs-review items")
		fmt.Println(" 2) Decide (accept/reject)")
		fmt.Println(" 3) Back")
		fmt.Print("> ")

		choice, _ := a.reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		var err error
		switch choice {
		case "1":
			err = listReview(a.ctx, a.review, a.reader, tenant.Slug)
		case "2":
			err = decideReview(a.ctx, a.review, a.reader, tenant.Slug)
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

func listReview(ctx context.Context, reviewService *services.ReviewService, reader *bufio.Reader, tenantID string) error {
	limitStr := promptDefault(reader, "Limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return fmt.Errorf("invalid limit: %q", limitStr)
	}

	items, err := reviewService.List(ctx, tenantID, "", limit)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println()
		fmt.Println("No items awaiting review.")
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Printf("%-26s %-20s %-15s %s\n", "Raw ID", "Source", "Status", "Payload keys")
	fmt.Println(strings.Repeat("-", 90))
	for _, item := range items {
		keys := make([]string, 0, len(item.Payload))
		for k := range item.Payload {
			keys = append(keys, k)
		}
		fmt.Printf("%-26s %-20s %-15s %s\n", item.ID, item.SourceSlug, item.Status, strings.Join(keys, ", "))
	}
	fmt.Println()
	return nil
}

func decideReview(ctx context.Context, reviewService *services.ReviewService, reader *bufio.Reader, tenantID string) error {
	rawID := prompt(reader, "Raw record ID")
	decision := promptDefault(reader, "Decision (accept/reject)", "accept")

	if err := reviewService.Decide(ctx, tenantID, rawID, decision, "tui-admin"); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Record %s marked as %s.\n\n", rawID, decision)
	return nil
}
