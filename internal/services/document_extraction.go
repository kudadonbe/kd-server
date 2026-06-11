package services

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const maxDocumentPages = 2

var (
	nationalIDPattern = regexp.MustCompile(`(?i)\b[A-Z]\s*\d{6}\b`)
	datePattern       = regexp.MustCompile(`\b([0-3]?\d)[-/]([01]?\d)[-/]((?:19|20)\d{2})\b`)
)

// DocumentExtraction contains OCR text and field suggestions for staff review.
type DocumentExtraction struct {
	Engine          string             `json:"engine"`
	PagesProcessed  int                `json:"pages_processed"`
	RawText         string             `json:"raw_text"`
	NationalID      string             `json:"national_id,omitempty"`
	NameEnglish     string             `json:"name_english,omitempty"`
	NameDhivehi     string             `json:"name_dhivehi,omitempty"`
	Sex             string             `json:"sex,omitempty"`
	DateOfBirth     string             `json:"date_of_birth,omitempty"`
	HouseEnglish    string             `json:"house_english,omitempty"`
	HouseDhivehi    string             `json:"house_dhivehi,omitempty"`
	IslandEnglish   string             `json:"island_english,omitempty"`
	IslandDhivehi   string             `json:"island_dhivehi,omitempty"`
	CommonName      string             `json:"common_name_english,omitempty"`
	BloodGroup      string             `json:"blood_group,omitempty"`
	ExpiryDate      string             `json:"expiry_date,omitempty"`
	SerialNumber    string             `json:"serial_number,omitempty"`
	FieldConfidence map[string]float64 `json:"field_confidence"`
	Warnings        []string           `json:"warnings,omitempty"`
}

// DocumentExtractor extracts reviewable fields from identity-document files.
type DocumentExtractor interface {
	Extract(ctx context.Context, filename, contentType string, source io.Reader) (*DocumentExtraction, error)
}

// LocalDocumentExtractor runs local Tesseract and Poppler processes.
type LocalDocumentExtractor struct {
	tesseract string
	pdftoppm  string
	tessdata  string
	tsvConfig string
}

type identityCardSide string

const (
	identityCardFront identityCardSide = "front"
	identityCardBack  identityCardSide = "back"
)

type templateRegion struct {
	name       string
	x1         float64
	y1         float64
	x2         float64
	y2         float64
	language   string
	psm        string
	confidence float64
}

var frontTemplateRegions = []templateRegion{
	{name: "national_id", x1: 0.36, y1: 0.23, x2: 0.64, y2: 0.36, language: "eng", psm: "7", confidence: 0.95},
	{name: "name_english", x1: 0.07, y1: 0.36, x2: 0.60, y2: 0.54, language: "eng", psm: "6", confidence: 0.85},
	{name: "sex", x1: 0.07, y1: 0.53, x2: 0.30, y2: 0.67, language: "eng", psm: "6", confidence: 0.9},
	{name: "date_of_birth", x1: 0.27, y1: 0.53, x2: 0.61, y2: 0.67, language: "eng", psm: "6", confidence: 0.9},
	{name: "address_english", x1: 0.07, y1: 0.66, x2: 0.61, y2: 0.92, language: "eng", psm: "6", confidence: 0.75},
}

var backTemplateRegions = []templateRegion{
	{name: "serial_number", x1: 0.02, y1: 0.01, x2: 0.40, y2: 0.12, language: "eng", psm: "7", confidence: 0.9},
	{name: "common_name_english", x1: 0.38, y1: 0.47, x2: 0.96, y2: 0.70, language: "eng", psm: "6", confidence: 0.9},
	{name: "blood_group", x1: 0.38, y1: 0.74, x2: 0.68, y2: 0.95, language: "eng", psm: "6", confidence: 0.95},
	{name: "expiry_date", x1: 0.66, y1: 0.74, x2: 0.96, y2: 0.95, language: "eng", psm: "6", confidence: 0.95},
}

// NewLocalDocumentExtractor discovers locally installed OCR dependencies.
func NewLocalDocumentExtractor() (*LocalDocumentExtractor, error) {
	tesseract := findExecutable("tesseract", tesseractCandidates()...)
	if tesseract == "" {
		return nil, errors.New("services: tesseract OCR is not installed")
	}
	pdftoppm := findExecutable("pdftoppm", popplerCandidates()...)
	tessdata := findTessdata()
	if tessdata == "" {
		return nil, errors.New("services: OCR language data is not installed")
	}
	tsvConfig := findTSVConfig(tesseract)
	if tsvConfig == "" {
		return nil, errors.New("services: Tesseract TSV configuration is not installed")
	}
	return &LocalDocumentExtractor{
		tesseract: tesseract,
		pdftoppm:  pdftoppm,
		tessdata:  tessdata,
		tsvConfig: tsvConfig,
	}, nil
}

// Extract processes one uploaded file in a disposable directory.
func (e *LocalDocumentExtractor) Extract(ctx context.Context, filename, contentType string, source io.Reader) (*DocumentExtraction, error) {
	if e == nil || e.tesseract == "" {
		return nil, errors.New("services: document extractor not configured")
	}

	tempDir, err := os.MkdirTemp("", "kd-document-*")
	if err != nil {
		return nil, fmt.Errorf("services: create document workspace: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	extension := strings.ToLower(filepath.Ext(filename))
	if extension == "" {
		extension = extensionForContentType(contentType)
	}
	inputPath := filepath.Join(tempDir, "upload"+extension)
	file, err := os.OpenFile(inputPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("services: create temporary document: %w", err)
	}
	if _, err := io.Copy(file, source); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("services: copy temporary document: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("services: close temporary document: %w", err)
	}

	images, err := e.imageInputs(ctx, tempDir, inputPath, extension)
	if err != nil {
		return nil, err
	}

	pageTexts := make([]string, 0, len(images))
	confidences := make([]float64, 0, len(images))
	for _, imagePath := range images {
		preparedPath, err := prepareOCRImage(tempDir, imagePath)
		if err != nil {
			return nil, err
		}
		segments, err := splitWideOCRImage(tempDir, preparedPath)
		if err != nil {
			return nil, err
		}
		for _, segment := range segments {
			text, confidence, err := e.ocrImage(ctx, segment)
			if err != nil {
				return nil, err
			}
			pageTexts = append(pageTexts, text)
			confidences = append(confidences, confidence)
			side := classifyIdentityCardSide(text)
			templateText, templateConfidence, err := e.ocrTemplateRegions(ctx, tempDir, segment, side)
			if err != nil {
				return nil, err
			}
			if templateText != "" {
				pageTexts = append(pageTexts, templateText)
				confidences = append(confidences, templateConfidence)
			}
		}
	}

	result := parseIdentityText(strings.Join(pageTexts, "\n\n--- PAGE ---\n\n"), average(confidences))
	result.Engine = "tesseract-5"
	result.PagesProcessed = len(images)
	return result, nil
}

func classifyIdentityCardSide(text string) identityCardSide {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "common name") ||
		strings.Contains(lower, "blood group") ||
		strings.Contains(lower, "finger print") ||
		strings.Contains(lower, "expires on") {
		return identityCardBack
	}
	return identityCardFront
}

func (e *LocalDocumentExtractor) ocrTemplateRegions(
	ctx context.Context,
	tempDir string,
	imagePath string,
	side identityCardSide,
) (string, float64, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", 0, fmt.Errorf("services: open template image: %w", err)
	}
	source, _, err := image.Decode(file)
	_ = file.Close()
	if err != nil {
		return "", 0, fmt.Errorf("services: decode template image: %w", err)
	}

	regions := frontTemplateRegions
	if side == identityCardBack {
		regions = backTemplateRegions
	}
	lines := make([]string, 0, len(regions))
	confidences := make([]float64, 0, len(regions))
	for _, region := range regions {
		regionPath, err := writeTemplateRegion(tempDir, source, region)
		if err != nil {
			return "", 0, err
		}
		text, ocrConfidence, err := e.ocrRegion(ctx, regionPath, region.language, region.psm)
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		lines = append(lines, region.name+": "+strings.TrimSpace(text))
		confidences = append(confidences, min(region.confidence, ocrConfidence))
	}
	return strings.Join(lines, "\n"), average(confidences), nil
}

func writeTemplateRegion(tempDir string, source image.Image, region templateRegion) (string, error) {
	bounds := source.Bounds()
	x1 := bounds.Min.X + int(float64(bounds.Dx())*region.x1)
	y1 := bounds.Min.Y + int(float64(bounds.Dy())*region.y1)
	x2 := bounds.Min.X + int(float64(bounds.Dx())*region.x2)
	y2 := bounds.Min.Y + int(float64(bounds.Dy())*region.y2)
	if x2 <= x1 || y2 <= y1 {
		return "", errors.New("services: invalid identity template region")
	}

	const scale = 3
	width := (x2 - x1) * scale
	height := (y2 - y1) * scale
	crop := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sourceY := y1 + y/scale
		for x := 0; x < width; x++ {
			sourceX := x1 + x/scale
			red16, green16, blue16, _ := source.At(sourceX, sourceY).RGBA()
			red := uint8(red16 >> 8)
			green := uint8(green16 >> 8)
			blue := uint8(blue16 >> 8)
			maxChannel := max(red, green, blue)
			minChannel := min(red, green, blue)
			if maxChannel-minChannel > 38 && maxChannel > 75 {
				crop.SetGray(x, y, color.Gray{Y: 255})
				continue
			}
			gray := color.GrayModel.Convert(source.At(sourceX, sourceY)).(color.Gray)
			value := gray.Y
			if value > 205 {
				value = 255
			} else if value < 95 {
				value = 0
			}
			crop.SetGray(x, y, color.Gray{Y: value})
		}
	}

	outputPath := filepath.Join(tempDir, "region-"+region.name+".jpg")
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("services: create identity region: %w", err)
	}
	if err := jpeg.Encode(output, crop, &jpeg.Options{Quality: 100}); err != nil {
		_ = output.Close()
		return "", fmt.Errorf("services: encode identity region: %w", err)
	}
	if err := output.Close(); err != nil {
		return "", fmt.Errorf("services: close identity region: %w", err)
	}
	return outputPath, nil
}

func (e *LocalDocumentExtractor) ocrRegion(ctx context.Context, imagePath, language, psm string) (string, float64, error) {
	args := []string{imagePath, "stdout", "--tessdata-dir", e.tessdata, "-l", language, "--psm", psm, e.tsvConfig}
	cmd := exec.CommandContext(ctx, e.tesseract, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0, fmt.Errorf("services: OCR identity region: %w: %s", err, strings.TrimSpace(string(output)))
	}
	words, total, count, err := parseTSVWords(string(output))
	if err != nil {
		return "", 0, err
	}
	return strings.Join(words, " "), averageWithCount(total, count), nil
}

func splitWideOCRImage(tempDir, imagePath string) ([]string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("services: open prepared OCR image: %w", err)
	}
	source, _, err := image.Decode(file)
	_ = file.Close()
	if err != nil {
		return nil, fmt.Errorf("services: decode prepared OCR image: %w", err)
	}
	bounds := source.Bounds()
	if float64(bounds.Dx())/float64(bounds.Dy()) < 2.2 {
		return []string{imagePath}, nil
	}

	midpoint := bounds.Min.X + bounds.Dx()/2
	rectangles := []image.Rectangle{
		image.Rect(bounds.Min.X, bounds.Min.Y, midpoint, bounds.Max.Y),
		image.Rect(midpoint, bounds.Min.Y, bounds.Max.X, bounds.Max.Y),
	}
	paths := make([]string, 0, len(rectangles))
	for index, rectangle := range rectangles {
		crop := image.NewRGBA(image.Rect(0, 0, rectangle.Dx(), rectangle.Dy()))
		for y := rectangle.Min.Y; y < rectangle.Max.Y; y++ {
			for x := rectangle.Min.X; x < rectangle.Max.X; x++ {
				crop.Set(x-rectangle.Min.X, y-rectangle.Min.Y, source.At(x, y))
			}
		}
		outputPath := filepath.Join(tempDir, fmt.Sprintf("segment-%d-%s", index+1, filepath.Base(imagePath)))
		output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return nil, fmt.Errorf("services: create OCR segment: %w", err)
		}
		if err := jpeg.Encode(output, crop, &jpeg.Options{Quality: 95}); err != nil {
			_ = output.Close()
			return nil, fmt.Errorf("services: encode OCR segment: %w", err)
		}
		if err := output.Close(); err != nil {
			return nil, fmt.Errorf("services: close OCR segment: %w", err)
		}
		paths = append(paths, outputPath)
	}
	return paths, nil
}

func prepareOCRImage(tempDir, imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("services: open OCR image: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	source, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("services: decode OCR image: %w", err)
	}
	bounds := source.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			if !nearWhite(source.At(x, y)) {
				found = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if !found {
		return imagePath, nil
	}

	margin := 20
	minX = max(bounds.Min.X, minX-margin)
	minY = max(bounds.Min.Y, minY-margin)
	maxX = min(bounds.Max.X, maxX+margin)
	maxY = min(bounds.Max.Y, maxY+margin)
	if maxX-minX < bounds.Dx()/4 || maxY-minY < bounds.Dy()/8 {
		return imagePath, nil
	}

	crop := image.NewRGBA(image.Rect(0, 0, maxX-minX, maxY-minY))
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			crop.Set(x-minX, y-minY, source.At(x, y))
		}
	}

	outputPath := filepath.Join(tempDir, "prepared-"+filepath.Base(imagePath)+".jpg")
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("services: create prepared OCR image: %w", err)
	}
	if err := jpeg.Encode(output, crop, &jpeg.Options{Quality: 95}); err != nil {
		_ = output.Close()
		return "", fmt.Errorf("services: encode prepared OCR image: %w", err)
	}
	if err := output.Close(); err != nil {
		return "", fmt.Errorf("services: close prepared OCR image: %w", err)
	}
	return outputPath, nil
}

func nearWhite(value color.Color) bool {
	red, green, blue, _ := value.RGBA()
	return red>>8 > 245 && green>>8 > 245 && blue>>8 > 245
}

func (e *LocalDocumentExtractor) imageInputs(ctx context.Context, tempDir, inputPath, extension string) ([]string, error) {
	if extension != ".pdf" {
		return []string{inputPath}, nil
	}
	if e.pdftoppm == "" {
		return nil, errors.New("services: Poppler pdftoppm is required for PDF extraction")
	}

	outputPrefix := filepath.Join(tempDir, "page")
	cmd := exec.CommandContext(ctx, e.pdftoppm, "-f", "1", "-l", strconv.Itoa(maxDocumentPages), "-r", "300", "-jpeg", inputPath, outputPrefix)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("services: render PDF: %w: %s", err, strings.TrimSpace(string(output)))
	}
	images, err := filepath.Glob(outputPrefix + "-*.jpg")
	if err != nil || len(images) == 0 {
		return nil, errors.New("services: PDF did not produce readable pages")
	}
	sort.Strings(images)
	return images, nil
}

func (e *LocalDocumentExtractor) ocrImage(ctx context.Context, imagePath string) (string, float64, error) {
	var words []string
	var confidenceTotal float64
	var confidenceCount int
	seen := make(map[string]struct{})
	for _, psm := range []string{"3", "6", "11"} {
		args := []string{imagePath, "stdout", "--tessdata-dir", e.tessdata, "-l", "eng+div", "--psm", psm, e.tsvConfig}
		cmd := exec.CommandContext(ctx, e.tesseract, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", 0, fmt.Errorf("services: OCR document: %w: %s", err, strings.TrimSpace(string(output)))
		}
		passWords, passTotal, passCount, err := parseTSVWords(string(output))
		if err != nil {
			return "", 0, err
		}
		line := strings.Join(passWords, " ")
		if line != "" {
			if _, exists := seen[line]; !exists {
				seen[line] = struct{}{}
				words = append(words, passWords...)
			}
		}
		confidenceTotal += passTotal
		confidenceCount += passCount
	}
	if len(words) == 0 {
		return "", 0, errors.New("services: OCR produced no readable text")
	}
	return strings.Join(words, " "), averageWithCount(confidenceTotal, confidenceCount), nil
}

func parseTSVWords(output string) ([]string, float64, int, error) {
	var words []string
	var confidenceTotal float64
	var confidenceCount int
	scanner := bufio.NewScanner(strings.NewReader(output))
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		columns := strings.Split(scanner.Text(), "\t")
		if len(columns) < 12 {
			continue
		}
		word := strings.TrimSpace(columns[11])
		confidence, parseErr := strconv.ParseFloat(columns[10], 64)
		if word == "" || parseErr != nil || confidence < 0 {
			continue
		}
		words = append(words, word)
		confidenceTotal += confidence / 100
		confidenceCount++
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, 0, fmt.Errorf("services: read OCR output: %w", err)
	}
	return words, confidenceTotal, confidenceCount, nil
}

func parseIdentityText(rawText string, baseConfidence float64) *DocumentExtraction {
	result := &DocumentExtraction{
		RawText:         strings.TrimSpace(rawText),
		FieldConfidence: make(map[string]float64),
	}
	normalized := strings.Join(strings.Fields(rawText), " ")

	if value := taggedValue(rawText, "national_id"); value != "" {
		if match := nationalIDPattern.FindString(value); match != "" {
			result.NationalID = strings.ToUpper(strings.ReplaceAll(match, " ", ""))
			result.FieldConfidence["national_id"] = min(0.95, baseConfidence)
		}
	}
	if match := nationalIDPattern.FindString(normalized); match != "" {
		if result.NationalID == "" {
			result.NationalID = strings.ToUpper(strings.ReplaceAll(match, " ", ""))
			result.FieldConfidence["national_id"] = baseConfidence
		}
	}

	if value := taggedValue(rawText, "date_of_birth"); value != "" {
		if match := datePattern.FindStringSubmatch(value); len(match) > 0 {
			result.DateOfBirth = isoDate(match)
			result.FieldConfidence["date_of_birth"] = min(0.9, baseConfidence)
		}
	}
	if value := taggedValue(rawText, "expiry_date"); value != "" {
		if match := datePattern.FindStringSubmatch(value); len(match) > 0 {
			result.ExpiryDate = isoDate(match)
			result.FieldConfidence["expiry_date"] = min(0.95, baseConfidence)
		}
	}
	dates := datePattern.FindAllStringSubmatch(normalized, 2)
	isBack := strings.Contains(strings.ToLower(normalized), "common name") ||
		strings.Contains(strings.ToLower(normalized), "blood group") ||
		strings.Contains(strings.ToLower(normalized), "expires on")
	if len(dates) > 0 && !isBack && result.DateOfBirth == "" {
		result.DateOfBirth = isoDate(dates[0])
		result.FieldConfidence["date_of_birth"] = baseConfidence
	}
	if result.ExpiryDate == "" && (len(dates) > 1 || (len(dates) > 0 && isBack)) {
		result.ExpiryDate = isoDate(dates[len(dates)-1])
		result.FieldConfidence["expiry_date"] = baseConfidence
	}

	upper := strings.ToUpper(normalized)
	taggedSex := strings.ToUpper(taggedValue(rawText, "sex"))
	if regexp.MustCompile(`\bF\b`).MatchString(taggedSex) || regexp.MustCompile(`\bSEX\s*[:\-]?\s*F\b`).MatchString(upper) {
		result.Sex = "F"
		result.FieldConfidence["sex"] = baseConfidence
	} else if regexp.MustCompile(`\bM\b`).MatchString(taggedSex) || regexp.MustCompile(`\bSEX\s*[:\-]?\s*M\b`).MatchString(upper) {
		result.Sex = "M"
		result.FieldConfidence["sex"] = baseConfidence
	}

	if !isBack {
		result.NameEnglish = cleanEnglishName(taggedValue(rawText, "name_english"))
		if result.NameEnglish == "" {
			result.NameEnglish = textAfterStandaloneLabel(normalized, "Name", []string{"Sex", "Date of Birth", "Address"}, 5)
		}
		result.NameEnglish = cleanEnglishName(result.NameEnglish)
		if result.NameEnglish != "" {
			result.FieldConfidence["name_english"] = baseConfidence * 0.85
		}
	}

	result.CommonName = cleanEnglishName(taggedValue(rawText, "common_name_english"))
	if result.CommonName == "" {
		result.CommonName = textAfterLabel(normalized, "Common Name", []string{"Blood Group", "Expires on", "Expiry Date"}, 5)
	}
	result.CommonName = cleanEnglishName(result.CommonName)
	if result.CommonName != "" {
		result.FieldConfidence["common_name_english"] = baseConfidence * 0.85
		if result.NameEnglish == "" {
			result.NameEnglish = result.CommonName
			result.FieldConfidence["name_english"] = baseConfidence * 0.75
		}
	}

	bloodSource := taggedValue(rawText, "blood_group") + " " + normalized
	if match := regexp.MustCompile(`(?i)\b(?:A|B|AB|O)\s*[+-]`).FindString(bloodSource); match != "" {
		result.BloodGroup = strings.ReplaceAll(strings.ToUpper(match), " ", "")
		result.FieldConfidence["blood_group"] = baseConfidence
	}
	serialSource := taggedValue(rawText, "serial_number") + " " + normalized
	if match := regexp.MustCompile(`(?i)\bSN\s*\d{5,12}\b`).FindString(serialSource); match != "" {
		result.SerialNumber = strings.ToUpper(strings.ReplaceAll(match, " ", ""))
		result.FieldConfidence["serial_number"] = baseConfidence
	}

	address := taggedValue(rawText, "address_english")
	if address == "" {
		address = textAfterLabel(normalized, "Address", []string{"Blood Group", "Expiry Date", "Common Name"}, 8)
	}
	if address != "" {
		address = cleanEnglishAddress(address)
	}
	if address != "" {
		parts := strings.Fields(address)
		if len(parts) > 1 {
			split := len(parts) / 2
			result.HouseEnglish = strings.Join(parts[:split], " ")
			result.IslandEnglish = strings.Join(parts[split:], " ")
		} else {
			result.HouseEnglish = address
		}
		result.FieldConfidence["address_english"] = baseConfidence * 0.65
	}

	if result.NationalID == "" {
		result.Warnings = append(result.Warnings, "National ID was not detected automatically.")
	}
	if result.NameEnglish == "" {
		result.Warnings = append(result.Warnings, "English name was not detected automatically.")
	}
	result.Warnings = append(result.Warnings, "Review every field against the original document before saving.")
	return result
}

func cleanEnglishAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	allowed := regexp.MustCompile(`^[A-Za-z0-9.' -]{3,100}$`)
	if !allowed.MatchString(value) {
		return ""
	}
	letters := 0
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' {
			letters++
		}
	}
	if letters < len(value)/2 {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func taggedValue(text, tag string) string {
	prefix := tag + ":"
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}

func cleanEnglishName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !regexp.MustCompile(`^[A-Za-z][A-Za-z' -]{2,79}$`).MatchString(value) {
		return ""
	}
	words := strings.Fields(value)
	if len(words) < 2 || len(words) > 5 {
		return ""
	}
	return strings.Join(words, " ")
}

func textAfterLabel(text, label string, stopLabels []string, maxWords int) string {
	index := strings.Index(strings.ToLower(text), strings.ToLower(label))
	if index < 0 {
		return ""
	}
	value := strings.TrimSpace(text[index+len(label):])
	value = strings.TrimLeft(value, ":;- ")
	lowerValue := strings.ToLower(value)
	end := len(value)
	for _, stop := range stopLabels {
		if stopIndex := strings.Index(lowerValue, strings.ToLower(stop)); stopIndex >= 0 && stopIndex < end {
			end = stopIndex
		}
	}
	words := strings.Fields(strings.TrimSpace(value[:end]))
	if len(words) > maxWords {
		words = words[:maxWords]
	}
	return strings.Join(words, " ")
}

func textAfterStandaloneLabel(text, label string, stopLabels []string, maxWords int) string {
	pattern := regexp.MustCompile(`(?i)(?:^|\s)` + regexp.QuoteMeta(label) + `\s*[:;-]?\s+`)
	location := pattern.FindStringIndex(text)
	if location == nil {
		return ""
	}
	value := strings.TrimSpace(text[location[1]:])
	lowerValue := strings.ToLower(value)
	end := len(value)
	for _, stop := range stopLabels {
		if stopIndex := strings.Index(lowerValue, strings.ToLower(stop)); stopIndex >= 0 && stopIndex < end {
			end = stopIndex
		}
	}
	words := strings.Fields(strings.TrimSpace(value[:end]))
	if len(words) > maxWords {
		words = words[:maxWords]
	}
	return strings.Join(words, " ")
}

func isoDate(match []string) string {
	if len(match) < 4 {
		return ""
	}
	day, dayErr := strconv.Atoi(match[1])
	month, monthErr := strconv.Atoi(match[2])
	if dayErr != nil || monthErr != nil {
		return ""
	}
	return fmt.Sprintf("%s-%02d-%02d", match[3], month, day)
}

func extensionForContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "application/pdf":
		return ".pdf"
	case "image/png":
		return ".png"
	default:
		return ".jpg"
	}
}

func average(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	return averageWithCount(total, len(values))
}

func averageWithCount(total float64, count int) float64 {
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func findExecutable(name string, candidates ...string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func tesseractCandidates() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	return []string{
		`C:\Program Files\Tesseract-OCR\tesseract.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Tesseract-OCR", "tesseract.exe"),
	}
}

func popplerCandidates() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(
		os.Getenv("LOCALAPPDATA"),
		"Microsoft", "WinGet", "Packages",
		"oschwartz10612.Poppler_*",
		"poppler-*", "Library", "bin", "pdftoppm.exe",
	))
	return matches
}

func findTessdata() string {
	candidates := []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "kd-server", "tessdata"),
		`C:\Program Files\Tesseract-OCR\tessdata`,
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "eng.traineddata")); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(candidate, "div.traineddata")); err == nil {
			return candidate
		}
	}
	return ""
}

func findTSVConfig(tesseractPath string) string {
	candidates := []string{
		filepath.Join(filepath.Dir(tesseractPath), "tessdata", "configs", "tsv"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Tesseract-OCR", "tessdata", "configs", "tsv"),
		`C:\Program Files\Tesseract-OCR\tessdata\configs\tsv`,
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
