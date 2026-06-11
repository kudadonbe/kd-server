package services

import (
	"strings"
	"testing"
)

func TestParseIdentityText(t *testing.T) {
	t.Parallel()

	result := parseIdentityText(`
		REPUBLIC OF MALDIVES
		NATIONAL IDENTITY CARD
		Number: A 123456
		Name Sample Person
		Sex F
		Date of Birth 7/4/2001
		Address Example House K. Male
	`, 0.9)

	if result.NationalID != "A123456" {
		t.Fatalf("unexpected national ID: %s", result.NationalID)
	}
	if result.NameEnglish != "Sample Person" {
		t.Fatalf("unexpected name: %s", result.NameEnglish)
	}
	if result.Sex != "F" {
		t.Fatalf("unexpected sex: %s", result.Sex)
	}
	if result.DateOfBirth != "2001-04-07" {
		t.Fatalf("unexpected date of birth: %s", result.DateOfBirth)
	}
	if result.FieldConfidence["national_id"] != 0.9 {
		t.Fatalf("unexpected national ID confidence: %v", result.FieldConfidence["national_id"])
	}
	if !strings.Contains(result.RawText, "REPUBLIC OF MALDIVES") {
		t.Fatalf("raw text was not retained")
	}
}

func TestParseIdentityTextWarnsWhenFieldsMissing(t *testing.T) {
	t.Parallel()

	result := parseIdentityText("unreadable scan", 0.2)
	if len(result.Warnings) < 3 {
		t.Fatalf("expected missing-field and review warnings, got %#v", result.Warnings)
	}
}

func TestParseIdentityBackText(t *testing.T) {
	t.Parallel()

	result := parseIdentityText(
		"Signature / Finger Print Common Name Sample Person Blood Group A+ Expires on 18/04/2027",
		0.8,
	)
	if result.NameEnglish != "Sample Person" || result.CommonName != "Sample Person" {
		t.Fatalf("unexpected common name mapping: %#v", result)
	}
	if result.DateOfBirth != "" {
		t.Fatalf("back expiry was incorrectly mapped as date of birth: %s", result.DateOfBirth)
	}
	if result.ExpiryDate != "2027-04-18" || result.BloodGroup != "A+" {
		t.Fatalf("unexpected back fields: %#v", result)
	}
}

func TestParseTemplateRegionText(t *testing.T) {
	t.Parallel()

	result := parseIdentityText(`
national_id: A123456
name_english: Sample Person
sex: M
date_of_birth: 07/04/2001
address_english: Example House K. Male
common_name_english: Sample Person
blood_group: A+
expiry_date: 18/04/2027
serial_number: SN1234567
`, 0.88)

	if result.NationalID != "A123456" || result.NameEnglish != "Sample Person" {
		t.Fatalf("unexpected front template fields: %#v", result)
	}
	if result.Sex != "M" || result.DateOfBirth != "2001-04-07" {
		t.Fatalf("unexpected demographic fields: %#v", result)
	}
	if result.CommonName != "Sample Person" || result.BloodGroup != "A+" {
		t.Fatalf("unexpected back template fields: %#v", result)
	}
	if result.ExpiryDate != "2027-04-18" || result.SerialNumber != "SN1234567" {
		t.Fatalf("unexpected document fields: %#v", result)
	}
}

func TestExtensionForContentType(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"application/pdf":       ".pdf",
		"image/png":             ".png",
		"image/jpeg; charset=x": ".jpg",
	}
	for contentType, want := range tests {
		if got := extensionForContentType(contentType); got != want {
			t.Fatalf("extensionForContentType(%q) = %q, want %q", contentType, got, want)
		}
	}
}

func TestNewLocalDocumentExtractorFindsInstalledConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("requires local OCR installation")
	}

	extractor, err := NewLocalDocumentExtractor()
	if err != nil {
		t.Fatalf("NewLocalDocumentExtractor returned error: %v", err)
	}
	if extractor.tsvConfig == "" {
		t.Fatal("expected Tesseract TSV configuration path")
	}
}
