package store

import (
	"context"
	"testing"
	"time"
)

func TestTenantLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Parallel()

	ctx := context.Background()
	dbName := "kdserver_test_tenant_" + time.Now().Format("20060102_150405")

	mongoStore, err := Connect(ctx, Config{
		URI:      defaultURI,
		Database: dbName,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoStore.db.Drop(dropCtx)

		closeCtx, cancelClose := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelClose()
		_ = mongoStore.Close(closeCtx)
	}()

	if err := mongoStore.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	tenant, err := mongoStore.CreateTenant(ctx, CreateTenantInput{
		Slug: "acme-co",
		Name: "Acme Co",
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	if tenant.Slug != "acme-co" {
		t.Fatalf("unexpected slug: %s", tenant.Slug)
	}

	list, err := mongoStore.ListTenants(ctx)
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 tenant, got %d", len(list))
	}

	issued, err := mongoStore.IssueAPIKey(ctx, tenant.Slug, "initial key")
	if err != nil {
		t.Fatalf("issue key: %v", err)
	}
	if issued.Secret == "" || issued.KeyID == "" {
		t.Fatalf("expected secret and key id")
	}
	if issued.Tenant.Slug != tenant.Slug {
		t.Fatalf("tenant mismatch")
	}
}
