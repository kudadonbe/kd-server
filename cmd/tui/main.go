package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kudadonbe/kd-server/internal/config"
	"github.com/kudadonbe/kd-server/internal/services"
	"github.com/kudadonbe/kd-server/internal/store"
)

const (
	defaultMongoURI      = "mongodb://localhost:27017"
	defaultMongoDatabase = "kdserver"
)

// session holds state that persists across menu navigation within one run.
type session struct {
	tenant *services.TenantSummary
}

// app bundles the services and I/O the TUI needs, so submenu functions can
// be defined as methods instead of threading a long parameter list.
type app struct {
	ctx      context.Context
	reader   *bufio.Reader
	session  *session
	admin    *services.AdminService
	ingest   *services.IngestService
	resolve  *services.ResolveService
	lookup   *services.LookupService
	review   *services.ReviewService
	assets   *services.AssetService
	identity *services.IdentityDocumentService
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)
	reader := bufio.NewReader(os.Stdin)

	if err := config.LoadDotEnv(); err != nil {
		logger.Fatalf("configuration load failed: %v", err)
	}

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

	a := &app{
		ctx:      ctx,
		reader:   reader,
		session:  &session{},
		admin:    services.NewAdminService(mongoStore),
		ingest:   services.NewIngestService(mongoStore.IngestWriter()),
		resolve:  services.NewResolveService(mongoStore),
		lookup:   services.NewLookupService(mongoStore),
		review:   services.NewReviewService(mongoStore),
		assets:   services.NewAssetService(mongoStore),
		identity: services.NewIdentityDocumentService(mongoStore),
	}

	fmt.Println("KD-Server Admin TUI (super-admin)")
	fmt.Printf("Connected to %s/%s\n\n", mongoURI, mongoDB)

	a.mainMenu()
}

func (a *app) mainMenu() {
	for {
		if a.session.tenant != nil {
			fmt.Printf("Current tenant: %s (%s)\n", a.session.tenant.Name, a.session.tenant.Slug)
		} else {
			fmt.Println("Current tenant: <none selected>")
		}
		fmt.Println("Select an option:")
		fmt.Println(" 1) Tenant & API Key Management")
		fmt.Println(" 2) Ingest & Resolve")
		fmt.Println(" 3) Lookup")
		fmt.Println(" 4) Review Queue")
		fmt.Println(" 5) Assets")
		fmt.Println(" 6) Identity Documents")
		fmt.Println(" 7) Switch tenant")
		fmt.Println(" 8) Exit")
		fmt.Print("> ")

		choice, _ := a.reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			a.tenantMenu()
		case "2":
			a.ingestResolveMenu()
		case "3":
			a.lookupMenu()
		case "4":
			a.reviewMenu()
		case "5":
			a.assetsMenu()
		case "6":
			a.identityMenu()
		case "7":
			a.switchTenant()
		case "8":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Println("Unknown option, please choose 1-8.")
			fmt.Println()
		}
	}
}

// requireTenant returns the currently selected tenant, prompting for one if
// none is set yet. It returns nil if the operator cancels or none exist.
func (a *app) requireTenant() *services.TenantSummary {
	if a.session.tenant != nil {
		return a.session.tenant
	}
	return a.switchTenant()
}

func (a *app) switchTenant() *services.TenantSummary {
	tenant, err := selectTenant(a.ctx, a.admin, a.reader, "Select tenant:")
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
		return nil
	}
	a.session.tenant = tenant
	return tenant
}

func envOrDefault(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
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
