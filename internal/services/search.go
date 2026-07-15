package services

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/kudadonbe/kd-server/internal/store"
)

// ErrNoSearchSignal is returned when a search request carries no usable signal.
var ErrNoSearchSignal = errors.New("services: at least one search signal required")

// candidateCap bounds how many index rows the store returns before Go-side scoring.
const candidateCap = 500

// Ranking weights. Strong deterministic identifiers dominate; weak partial signals
// accumulate (e.g. name-prefix + DOB + island out-ranks any single weak signal).
const (
	pointsNationalID  = 100
	pointsEmail       = 60
	pointsPhone       = 50
	pointsDateOfBirth = 40
	pointsNameExact   = 20
	pointsIsland      = 15
	pointsNamePrefix  = 10
	pointsLinkedTerm  = 8
)

// EntitySearch finds candidate persons from partial, multi-signal input and
// rebuilds the tenant's search index.
type EntitySearch interface {
	Search(ctx context.Context, tenantID string, req SearchRequest) (*SearchResult, error)
	Reindex(ctx context.Context, tenantID string) (int, error)
}

// SearchRequest is a multi-signal query. Any field may be empty; at least one must
// be present. Q is free text matched across names and all linked-record fields.
type SearchRequest struct {
	Q           string
	Name        string
	NationalID  string
	Email       string
	Phone       string
	DateOfBirth string
	Island      string
	Limit       int
	Offset      int
}

// SearchResult is a ranked, paginated candidate list.
type SearchResult struct {
	Candidates []Candidate
	Total      int
	Limit      int
	Offset     int
}

// Candidate is one scored person with an explanation of why it matched.
type Candidate struct {
	PersonID  string
	Score     int
	MatchedOn []MatchReason
	Summary   store.EntitySummary
}

// MatchReason records a single scoring award, forming the human-readable "why".
type MatchReason struct {
	Field  string
	Value  string
	Points int
}

// EntitySearchService implements EntitySearch over the Mongo entity index.
type EntitySearchService struct {
	store *store.MongoStore
}

// NewEntitySearchService builds the service from the Mongo store.
func NewEntitySearchService(mongoStore *store.MongoStore) *EntitySearchService {
	return &EntitySearchService{store: mongoStore}
}

var _ EntitySearch = (*EntitySearchService)(nil)

type normalizedQuery struct {
	tokens      []string
	nationalID  string
	email       string
	phone       string
	dateOfBirth string
	island      string
}

func (q normalizedQuery) empty() bool {
	return len(q.tokens) == 0 && q.nationalID == "" && q.email == "" &&
		q.phone == "" && q.dateOfBirth == "" && q.island == ""
}

// Search normalizes the request, filters candidates in Mongo, scores and ranks
// them in Go, then paginates.
func (s *EntitySearchService) Search(ctx context.Context, tenantID string, req SearchRequest) (*SearchResult, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, errors.New("services: tenant required")
	}

	q := normalizeQuery(req)
	if q.empty() {
		return nil, ErrNoSearchSignal
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	limit = min(limit, 100)
	offset := max(req.Offset, 0)

	rows, err := s.store.SearchEntityIndex(ctx, tenantID, store.EntitySearchQuery{
		Tokens:      q.tokens,
		NationalID:  q.nationalID,
		Email:       q.email,
		Phone:       q.phone,
		DateOfBirth: q.dateOfBirth,
		Island:      q.island,
	}, candidateCap)
	if err != nil {
		return nil, err
	}

	scored := make([]Candidate, 0, len(rows))
	for i := range rows {
		points, reasons := scoreCandidate(rows[i], q)
		if points <= 0 {
			continue
		}
		scored = append(scored, Candidate{
			PersonID:  rows[i].PersonID,
			Score:     points,
			MatchedOn: reasons,
			Summary:   rows[i].Summary,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].PersonID < scored[j].PersonID
	})

	total := len(scored)
	offset = min(offset, total)
	end := min(offset+limit, total)

	return &SearchResult{
		Candidates: scored[offset:end],
		Total:      total,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

// Reindex rebuilds the tenant's entire entity index (cold start / drift repair).
func (s *EntitySearchService) Reindex(ctx context.Context, tenantID string) (int, error) {
	return s.store.BackfillEntityIndex(ctx, strings.TrimSpace(tenantID))
}

func normalizeQuery(req SearchRequest) normalizedQuery {
	tokens := make([]string, 0, 8)
	seen := make(map[string]struct{})
	addTokens := func(text string) {
		for _, tok := range store.Tokenize(text) {
			if _, ok := seen[tok]; ok {
				continue
			}
			if len(tokens) >= 10 { // bound $or width
				return
			}
			seen[tok] = struct{}{}
			tokens = append(tokens, tok)
		}
	}
	addTokens(req.Q)
	addTokens(req.Name)

	return normalizedQuery{
		tokens:      tokens,
		nationalID:  strings.ToUpper(strings.TrimSpace(req.NationalID)),
		email:       store.NormalizeTerm(req.Email),
		phone:       strings.TrimSpace(req.Phone),
		dateOfBirth: strings.TrimSpace(req.DateOfBirth),
		island:      store.NormalizeTerm(req.Island),
	}
}

func scoreCandidate(ei store.EntityIndex, q normalizedQuery) (int, []MatchReason) {
	var points int
	reasons := make([]MatchReason, 0, 4)
	award := func(field, value string, p int) {
		points += p
		reasons = append(reasons, MatchReason{Field: field, Value: value, Points: p})
	}

	if q.nationalID != "" && ei.NationalID == q.nationalID {
		award("national_id", ei.NationalID, pointsNationalID)
	}
	if q.email != "" && containsString(ei.Emails, q.email) {
		award("email", q.email, pointsEmail)
	}
	if q.phone != "" && containsString(ei.Phones, q.phone) {
		award("phone", q.phone, pointsPhone)
	}
	if q.dateOfBirth != "" && ei.DateOfBirth == q.dateOfBirth {
		award("date_of_birth", q.dateOfBirth, pointsDateOfBirth)
	}
	if q.island != "" && containsString(ei.Islands, q.island) {
		award("island", q.island, pointsIsland)
	}

	nameSet := toSet(ei.NameTerms)
	termSet := toSet(ei.Terms)
	for _, tok := range q.tokens {
		switch {
		case nameSet[tok]:
			award("name", tok, pointsNameExact)
		case hasPrefix(ei.NameTerms, tok):
			award("name", tok, pointsNamePrefix)
		case termSet[tok]:
			award("linked", tok, pointsLinkedTerm)
		case hasPrefix(ei.Terms, tok):
			award("linked", tok, pointsLinkedTerm)
		}
	}

	return points, reasons
}

func containsString(values []string, target string) bool {
	return slices.Contains(values, target)
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}

func hasPrefix(values []string, prefix string) bool {
	for _, v := range values {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}
