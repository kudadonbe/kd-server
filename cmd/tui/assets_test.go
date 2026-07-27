package main

import "testing"

func TestParseAssetIngestFile(t *testing.T) {
	data := []byte(`{
		"source": {"slug": "asset-office", "name": "Asset Office"},
		"records": [
			{
				"office": "B15",
				"categoryNo": "02",
				"typeNumber": "073",
				"description1": "Office chair",
				"location": "HQ - Floor 2",
				"quantity": 1,
				"isPurchasedNew": true,
				"acquisitionDate": "2024-01-15",
				"originalValue": 1200.50,
				"currency": "MVR",
				"requestedBy": {"userId": "u1", "name": "Ali", "designation": "Officer", "date": "2024-01-10"}
			}
		]
	}`)

	sourceSlug, records, err := parseAssetIngestFile(data)
	if err != nil {
		t.Fatalf("parseAssetIngestFile returned error: %v", err)
	}

	if sourceSlug != "asset-office" {
		t.Errorf("expected source slug %q, got %q", "asset-office", sourceSlug)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Office != "B15" || rec.CategoryNo != "02" || rec.TypeNumber != "073" {
		t.Errorf("unexpected classification fields: %+v", rec)
	}
	if rec.AcquisitionDate.Format("2006-01-02") != "2024-01-15" {
		t.Errorf("expected acquisitionDate 2024-01-15, got %s", rec.AcquisitionDate)
	}
	if rec.RequestedBy == nil || rec.RequestedBy.Name != "Ali" {
		t.Errorf("expected requestedBy to be mapped, got %+v", rec.RequestedBy)
	}
}

func TestParseAssetIngestFileInvalidAcquisitionDate(t *testing.T) {
	data := []byte(`{
		"source": {"slug": "asset-office"},
		"records": [
			{"office": "B15", "categoryNo": "02", "typeNumber": "073", "acquisitionDate": "15-01-2024"}
		]
	}`)

	_, _, err := parseAssetIngestFile(data)
	if err == nil {
		t.Fatal("expected an error for invalid acquisitionDate format, got nil")
	}
}

func TestParseAssetIngestFileInvalidJSON(t *testing.T) {
	_, _, err := parseAssetIngestFile([]byte(`not json`))
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}
