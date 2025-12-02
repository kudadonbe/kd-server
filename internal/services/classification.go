package services

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed classification_data.json
var classificationFS embed.FS

// AssetCategory represents a main asset category.
type AssetCategory struct {
	Code          string             `json:"code"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	Subcategories []AssetSubcategory `json:"subcategories"`
}

// AssetSubcategory represents a subcategory within a main category.
type AssetSubcategory struct {
	Code  string      `json:"code"`
	Name  string      `json:"name"`
	Types []AssetType `json:"types"`
}

// AssetType represents a specific asset type.
type AssetType struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ClassificationData holds the complete classification hierarchy.
type ClassificationData struct {
	Categories []AssetCategory `json:"categories"`
}

// ClassificationService provides asset classification lookup.
type ClassificationService struct {
	data  *ClassificationData
	mu    sync.RWMutex
	cache map[string]*AssetCategory // category code -> category
}

// NewClassificationService creates a new classification service.
func NewClassificationService() (*ClassificationService, error) {
	svc := &ClassificationService{
		cache: make(map[string]*AssetCategory),
	}

	// Load embedded classification data
	data, err := classificationFS.ReadFile("classification_data.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read classification data: %w", err)
	}

	var classData ClassificationData
	if err := json.Unmarshal(data, &classData); err != nil {
		return nil, fmt.Errorf("failed to parse classification data: %w", err)
	}

	svc.data = &classData

	// Build cache
	for i := range svc.data.Categories {
		cat := &svc.data.Categories[i]
		svc.cache[cat.Code] = cat
	}

	return svc, nil
}

// GetAllCategories returns all asset categories.
func (s *ClassificationService) GetAllCategories(ctx context.Context) ([]AssetCategory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.data == nil {
		return nil, fmt.Errorf("classification data not loaded")
	}

	return s.data.Categories, nil
}

// GetCategory returns a specific category by code.
func (s *ClassificationService) GetCategory(ctx context.Context, categoryCode string) (*AssetCategory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cat, ok := s.cache[categoryCode]
	if !ok {
		return nil, nil // Not found
	}

	return cat, nil
}

// ValidateCategoryCode checks if a category code is valid.
func (s *ClassificationService) ValidateCategoryCode(categoryCode string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.cache[categoryCode]
	return ok
}

// ValidateTypeCode checks if a type code exists in a given category.
func (s *ClassificationService) ValidateTypeCode(categoryCode, typeCode string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cat, ok := s.cache[categoryCode]
	if !ok {
		return false
	}

	for _, sub := range cat.Subcategories {
		for _, typ := range sub.Types {
			if typ.Code == typeCode {
				return true
			}
		}
	}

	return false
}

// GetTypeName returns the name of a type code within a category.
func (s *ClassificationService) GetTypeName(categoryCode, typeCode string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cat, ok := s.cache[categoryCode]
	if !ok {
		return ""
	}

	for _, sub := range cat.Subcategories {
		for _, typ := range sub.Types {
			if typ.Code == typeCode {
				return typ.Name
			}
		}
	}

	return ""
}

// GetSubcategoryName returns the name of a subcategory code within a category.
func (s *ClassificationService) GetSubcategoryName(categoryCode, subCategoryCode string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cat, ok := s.cache[categoryCode]
	if !ok {
		return ""
	}

	for _, sub := range cat.Subcategories {
		if sub.Code == subCategoryCode {
			return sub.Name
		}
	}

	return ""
}
