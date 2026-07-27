package main

import "testing"

func TestParseIngestFile(t *testing.T) {
	data := []byte(`{
		"source": {"slug": "schoolsync", "name": "SchoolSync"},
		"records": [
			{"email": "person@example.com", "national_id": "A123456"}
		]
	}`)

	command, err := parseIngestFile(data)
	if err != nil {
		t.Fatalf("parseIngestFile returned error: %v", err)
	}

	if command.Source.Slug != "schoolsync" {
		t.Errorf("expected source slug %q, got %q", "schoolsync", command.Source.Slug)
	}
	if command.Source.Name != "SchoolSync" {
		t.Errorf("expected source name %q, got %q", "SchoolSync", command.Source.Name)
	}
	if len(command.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(command.Records))
	}
	if command.Records[0]["email"] != "person@example.com" {
		t.Errorf("expected email field to be preserved, got %v", command.Records[0]["email"])
	}
}

func TestParseIngestFileInvalidJSON(t *testing.T) {
	_, err := parseIngestFile([]byte(`not json`))
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestParseIngestFileRejectsUnknownFields(t *testing.T) {
	data := []byte(`{"source": {"slug": "x"}, "records": [], "unexpected": true}`)
	_, err := parseIngestFile(data)
	if err == nil {
		t.Fatal("expected an error for unknown top-level field, got nil")
	}
}
