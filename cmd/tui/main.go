package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/store"
)

const (
	defaultMongoURI      = "mongodb://localhost:27017"
	defaultMongoDatabase = "kdserver"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)
	reader := bufio.NewReader(os.Stdin)

	mongoURI := envOrDefault("MONGO_URI", defaultMongoURI)
	mongoDB := envOrDefault("MONGO_DB", defaultMongoDatabase)

	ctx := context.Background()
	mongoStore, err := store.Connect(ctx, store.Config{
		URI:      mongoURI,
		Database: mongoDB,
		Logger:   logger,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		logger.Fatalf("failed to connect to mongo: %v", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoStore.Close(closeCtx)
	}()

	if err := mongoStore.EnsureIndexes(ctx); err != nil {
		logger.Fatalf("failed to ensure indexes: %v", err)
	}

	fmt.Println("KD-Server Admin TUI")
	fmt.Printf("Connected to %s/%s\n\n", mongoURI, mongoDB)

	for {
		fmt.Println("Select an option:")
		fmt.Println(" 1) List tenants")
		fmt.Println(" 2) Create tenant")
		fmt.Println(" 3) Issue API key")
		fmt.Println(" 4) Exit")
		fmt.Print("> ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			if err := listTenants(ctx, mongoStore); err != nil {
				fmt.Printf("Error: %v\n\n", err)
			}
		case "2":
			if err := createTenant(ctx, mongoStore, reader); err != nil {
				fmt.Printf("Error: %v\n\n", err)
			}
		case "3":
			if err := issueKey(ctx, mongoStore, reader); err != nil {
				fmt.Printf("Error: %v\n\n", err)
			}
		case "4":
			fmt.Println("Goodbye!")
			return
        default:
            fmt.Println("Unknown option, please choose 1-4.")
            fmt.Println()
		}
	}
}

func envOrDefault(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func listTenants(ctx context.Context, mongoStore *store.MongoStore) error {
	tenants, err := mongoStore.ListTenants(ctx)
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
		fmt.Printf("%-4d %-20s %-25s %-20s\n",
			idx+1,
			tenant.Slug,
			tenant.Name,
			tenant.CreatedAt.Format(time.RFC3339),
		)
	}
    fmt.Println()
    return nil
}

func createTenant(ctx context.Context, mongoStore *store.MongoStore, reader *bufio.Reader) error {
    fmt.Println()
    fmt.Println("Create Tenant")
	name := prompt(reader, "Friendly name (e.g. Acme Co)")
	defaultSlug := slugify(name)
	slug := promptDefault(reader, "Slug (lowercase, no spaces)", defaultSlug)

	tenant, err := mongoStore.CreateTenant(ctx, store.CreateTenantInput{
		Slug: slug,
		Name: name,
	})
	if err != nil {
		return err
	}

    fmt.Println()
    fmt.Printf("Tenant created: %s (%s)\n", tenant.Name, tenant.Slug)
    fmt.Println()
	return nil
}

func issueKey(ctx context.Context, mongoStore *store.MongoStore, reader *bufio.Reader) error {
	tenants, err := mongoStore.ListTenants(ctx)
	if err != nil {
		return err
	}
    if len(tenants) == 0 {
        fmt.Println()
        fmt.Println("No tenants available. Create a tenant first.")
        fmt.Println()
        return nil
    }

    fmt.Println()
    fmt.Println("Select tenant for API key:")
	for idx, tenant := range tenants {
		fmt.Printf(" %d) %s (%s)\n", idx+1, tenant.Name, tenant.Slug)
	}
	fmt.Print("> ")

	selectionStr, _ := reader.ReadString('\n')
	selectionStr = strings.TrimSpace(selectionStr)
	selection, err := strconv.Atoi(selectionStr)
	if err != nil || selection < 1 || selection > len(tenants) {
		return errors.New("invalid selection")
	}
	tenant := tenants[selection-1]

	label := promptDefault(reader, "Label (optional)", fmt.Sprintf("%s key", tenant.Name))

	issued, err := mongoStore.IssueAPIKey(ctx, tenant.Slug, label)
	if err != nil {
		return err
	}

    fmt.Println()
    fmt.Println("API Key issued!")
	fmt.Printf("Tenant: %s (%s)\n", issued.Tenant.Name, issued.Tenant.Slug)
	fmt.Printf("Key ID: %s\n", issued.KeyID)
	fmt.Printf("Label: %s\n", issued.Label)
    fmt.Println()
    fmt.Println("Secret (store this securely, it will not be shown again):")
	fmt.Println(issued.Secret)

    fmt.Println()
    fmt.Println(".env snippet:")
	fmt.Printf("KD_TENANT=%s\n", issued.Tenant.Slug)
    fmt.Printf("KD_API_KEY=%s\n", issued.Secret)
    fmt.Println()
	return nil
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func promptDefault(reader *bufio.Reader, label, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultValue
	}
	return text
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
