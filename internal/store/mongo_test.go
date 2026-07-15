package store

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestDefaultIndexSpecs(t *testing.T) {
	specs := defaultIndexSpecs()

	expected := map[string][]expectedIndex{
		"people": {
			{
				name:   "people_tenant_person",
				keys:   bson.D{{Key: "tenantId", Value: 1}, {Key: "personId", Value: 1}},
				unique: true,
			},
			{
				name: "people_tenant_email",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "primaryEmail", Value: 1}},
			},
			{
				name: "people_tenant_phone",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "primaryPhone", Value: 1}},
			},
		},
		"links": {
			{
				name:   "links_external",
				keys:   bson.D{{Key: "tenantId", Value: 1}, {Key: "externalId", Value: 1}, {Key: "source", Value: 1}},
				unique: true,
			},
			{
				name: "links_person",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "personId", Value: 1}},
			},
		},
		"sources": {
			{
				name:   "sources_slug",
				keys:   bson.D{{Key: "tenantId", Value: 1}, {Key: "slug", Value: 1}},
				unique: true,
			},
		},
		"raw": {
			{
				name: "raw_ingest_batch",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "ingestBatch", Value: 1}},
			},
			{
				name: "raw_status",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "status", Value: 1}},
			},
		},
		"identity_documents": {
			{
				name:   "identity_documents_tenant_document",
				keys:   bson.D{{Key: "tenantId", Value: 1}, {Key: "documentId", Value: 1}},
				unique: true,
			},
			{
				name: "identity_documents_tenant_person",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "personId", Value: 1}, {Key: "updatedAt", Value: -1}},
			},
			{
				name: "identity_documents_tenant_national_id",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "nationalId", Value: 1}},
			},
		},
		"identity_document_history": {
			{
				name:   "identity_document_history_version",
				keys:   bson.D{{Key: "tenantId", Value: 1}, {Key: "documentId", Value: 1}, {Key: "version", Value: -1}},
				unique: true,
			},
			{
				name: "identity_document_history_person",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "personId", Value: 1}, {Key: "createdAt", Value: -1}},
			},
		},
		"tenants": {
			{
				name:   "tenants_slug",
				keys:   bson.D{{Key: "slug", Value: 1}},
				unique: true,
			},
		},
		"keys": {
			{
				name:   "keys_tenant_key",
				keys:   bson.D{{Key: "tenantId", Value: 1}, {Key: "keyId", Value: 1}},
				unique: true,
			},
			{
				name: "keys_hash",
				keys: bson.D{{Key: "keyHash", Value: 1}},
			},
		},
		"assets": {
			{
				name:   "assets_number",
				keys:   bson.D{{Key: "assetNo", Value: 1}},
				unique: true,
			},
			{
				name: "assets_tenant_number",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "assetNo", Value: 1}},
			},
			{
				name: "assets_number_sequence",
				keys: bson.D{
					{Key: "office", Value: 1},
					{Key: "year", Value: 1},
					{Key: "categoryNo", Value: 1},
					{Key: "typeNumber", Value: 1},
					{Key: "serialIncrement", Value: -1},
				},
			},
			{
				name: "assets_tenant_office",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "office", Value: 1}},
			},
			{
				name: "assets_tenant_category",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "categoryNo", Value: 1}},
			},
			{
				name: "assets_tenant_status",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "status", Value: 1}},
			},
			{
				name: "assets_tenant_acqdate",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "acquisitionDate", Value: 1}},
			},
		},
		"asset_links": {
			{
				name: "asset_links_asset",
				keys: bson.D{{Key: "assetNo", Value: 1}},
			},
			{
				name: "asset_links_tenant_asset",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "assetNo", Value: 1}},
			},
			{
				name: "asset_links_source_external",
				keys: bson.D{{Key: "source", Value: 1}, {Key: "externalId", Value: 1}},
			},
		},
		"asset_history": {
			{
				name: "asset_history_asset_time",
				keys: bson.D{{Key: "assetNo", Value: 1}, {Key: "timestamp", Value: -1}},
			},
			{
				name: "asset_history_tenant_event",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "eventType", Value: 1}},
			},
		},
		"ai_credentials": {
			{
				name:   "ai_credentials_tenant_provider",
				keys:   bson.D{{Key: "tenantId", Value: 1}, {Key: "provider", Value: 1}},
				unique: true,
			},
		},
		"entity_index": {
			{
				name:   "entity_index_tenant_person",
				keys:   bson.D{{Key: "tenantId", Value: 1}, {Key: "personId", Value: 1}},
				unique: true,
			},
			{
				name: "entity_index_terms",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "terms", Value: 1}},
			},
			{
				name: "entity_index_national_id",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "nationalId", Value: 1}},
			},
			{
				name: "entity_index_dob",
				keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "dateOfBirth", Value: 1}},
			},
		},
	}

	if len(specs) != len(expected) {
		t.Fatalf("unexpected collection count: got %d want %d", len(specs), len(expected))
	}

	for collection, expIndexes := range expected {
		models, ok := specs[collection]
		if !ok {
			t.Fatalf("missing collection %s", collection)
		}

		if len(models) != len(expIndexes) {
			t.Fatalf("unexpected index count for %s: got %d want %d", collection, len(models), len(expIndexes))
		}

		for i, exp := range expIndexes {
			model := models[i]

			keys, ok := model.Keys.(bson.D)
			if !ok {
				t.Fatalf("collection %s index %d keys not bson.D", collection, i)
			}

			if len(keys) != len(exp.keys) {
				t.Fatalf("collection %s index %s: unexpected key length %d want %d", collection, exp.name, len(keys), len(exp.keys))
			}

			for j, key := range keys {
				if key != exp.keys[j] {
					t.Fatalf("collection %s index %s: unexpected key element %v want %v", collection, exp.name, key, exp.keys[j])
				}
			}

			name := ""
			if model.Options != nil && model.Options.Name != nil {
				name = *model.Options.Name
			}

			if name != exp.name {
				t.Fatalf("collection %s index %d: unexpected name %s want %s", collection, i, name, exp.name)
			}

			gotUnique := false
			if model.Options != nil && model.Options.Unique != nil {
				gotUnique = *model.Options.Unique
			}

			if gotUnique != exp.unique {
				t.Fatalf("collection %s index %s: unexpected unique %t want %t", collection, exp.name, gotUnique, exp.unique)
			}
		}
	}
}

func TestDescribeKeys(t *testing.T) {
	doc := bson.D{
		{Key: "tenantId", Value: 1},
		{Key: "personId", Value: 1},
	}

	got := describeKeys(doc)
	want := "tenantId:1,personId:1"

	if got != want {
		t.Fatalf("unexpected describeKeys output: got %s want %s", got, want)
	}
}

type expectedIndex struct {
	name   string
	keys   bson.D
	unique bool
}
