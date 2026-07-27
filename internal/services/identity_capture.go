package services

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/kudadonbe/kd-server/internal/store"
)

// Capture trust levels. Trusted captures come from records:write callers
// (API key / staff); untrusted captures come from public extract-only users and
// are quarantined — they only ever land in the review queue.
const (
	CaptureTrustTrusted   = "trusted"
	CaptureTrustUntrusted = "untrusted"

	// captureTypeIdentity marks a raw review record as an identity-document
	// capture, so review-accept knows to apply it as a document (not a plain
	// ingest record).
	captureTypeIdentity = "identity"
)

// ChangedField is one field the traced document adds or contradicts versus what
// the server currently holds. Values are field labels, not raw PII dumps —
// callers must not log Old/New.
type ChangedField struct {
	Field string `json:"field"`
	Old   string `json:"old,omitempty"`
	New   string `json:"new"`
}

// IdentityDelta is the result of diffing a traced identity document against the
// person's current record.
type IdentityDelta struct {
	PersonID          string                  // resolved person, empty if none matched
	IsNewPerson       bool                    // no existing person matched the identifiers
	LatestVerified    bool                    // the person's newest document is verified
	Changed           []ChangedField          // fields the trace adds or contradicts
	ConflictsVerified bool                    // there is a delta against a verified record
	Current           *store.IdentityDocument // the person's newest document, nil if none
}

// HasNewInfo reports whether the trace carries anything the server does not
// already hold.
func (d IdentityDelta) HasNewInfo() bool {
	return d.IsNewPerson || len(d.Changed) > 0
}

// IdentityCaptureStore is the persistence the capture flow composes: resolve the
// person, read their latest document, and queue new info for review.
type IdentityCaptureStore interface {
	FindPersonByIdentifiers(ctx context.Context, tenantID string, ids store.PersonIdentifiers) (*store.Person, error)
	ListIdentityDocuments(ctx context.Context, tenantID, personID string, limit int) ([]store.IdentityDocument, error)
	InsertRawReview(ctx context.Context, tenantID, sourceSlug string, payload map[string]any) (string, error)
}

// IdentityCaptureService diffs traced identity data against the current record
// and, when it carries new information, queues that delta for human review. It
// never overwrites stored data and never reveals what it found to the caller.
type IdentityCaptureService struct {
	store IdentityCaptureStore
}

// NewIdentityCaptureService builds the capture service.
func NewIdentityCaptureService(s IdentityCaptureStore) *IdentityCaptureService {
	return &IdentityCaptureService{store: s}
}

// Diff resolves the person by deterministic identifiers, loads their latest
// identity document, and returns the fields the trace would add or contradict.
// A trace with no national ID cannot be safely resolved, so it yields no delta.
func (s *IdentityCaptureService) Diff(ctx context.Context, tenantID, phone string, doc store.IdentityDocument) (*IdentityDelta, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("services: identity capture store not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, errors.New("services: tenant required")
	}

	normalizeIdentityDocument(&doc)
	phone = strings.TrimSpace(phone)
	if doc.NationalID == "" {
		// Without a deterministic identifier we cannot resolve a person or
		// meaningfully diff — treat as no new info rather than guess.
		return &IdentityDelta{}, nil
	}

	ids := store.PersonIdentifiers{NationalID: doc.NationalID, Phone: phone}
	person, err := s.store.FindPersonByIdentifiers(ctx, tenantID, ids)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return &IdentityDelta{IsNewPerson: true, Changed: nonEmptyFields(doc)}, nil
	case err != nil:
		return nil, err
	}

	docs, err := s.store.ListIdentityDocuments(ctx, tenantID, person.PersonID, 1)
	if err != nil {
		return nil, err
	}
	delta := &IdentityDelta{PersonID: person.PersonID}
	if len(docs) == 0 {
		// Person exists (e.g. from ingest) but has no identity document yet:
		// everything the trace holds is new.
		delta.Changed = nonEmptyFields(doc)
		return delta, nil
	}

	current := docs[0]
	delta.Current = &current
	delta.LatestVerified = strings.EqualFold(current.VerificationStatus, VerificationStatusVerified)
	delta.Changed = diffIdentityFields(current, doc)
	delta.ConflictsVerified = delta.LatestVerified && len(delta.Changed) > 0
	return delta, nil
}

// Capture diffs the trace and, when it carries new info, writes the delta to the
// review queue. It returns the review record id (empty when nothing was queued).
// Best-effort by design: callers treat an error as non-fatal.
func (s *IdentityCaptureService) Capture(ctx context.Context, tenantID, sourceSlug, trust, phone string, doc store.IdentityDocument) (rawID string, delta *IdentityDelta, err error) {
	delta, err = s.Diff(ctx, tenantID, phone, doc)
	if err != nil {
		return "", nil, err
	}
	if !delta.HasNewInfo() {
		return "", delta, nil
	}

	normalizeIdentityDocument(&doc)
	doc.TenantID = strings.TrimSpace(tenantID)
	payload := map[string]any{
		"capture_type":       captureTypeIdentity,
		"trust":              trust,
		"resolved_person_id": delta.PersonID,
		"is_new_person":      delta.IsNewPerson,
		"conflicts_verified": delta.ConflictsVerified,
		"changed_fields":     changedFieldLabels(delta.Changed),
		"phone":              phone,
		"document":           doc,
	}
	rawID, err = s.store.InsertRawReview(ctx, tenantID, sourceSlug, payload)
	if err != nil {
		return "", delta, err
	}
	return rawID, delta, nil
}

// diffIdentityFields returns the traced fields that differ from the current
// document. Only non-empty traced values count — an OCR omission never erases
// stored data.
func diffIdentityFields(current, traced store.IdentityDocument) []ChangedField {
	var changed []ChangedField
	add := func(field, oldVal, newVal string) {
		newVal = strings.TrimSpace(newVal)
		oldVal = strings.TrimSpace(oldVal)
		if newVal == "" || strings.EqualFold(newVal, oldVal) {
			return
		}
		changed = append(changed, ChangedField{Field: field, Old: oldVal, New: newVal})
	}

	add("name_english", current.Name.English, traced.Name.English)
	add("name_dhivehi", current.Name.Dhivehi, traced.Name.Dhivehi)
	add("common_name_english", current.CommonName.English, traced.CommonName.English)
	add("common_name_dhivehi", current.CommonName.Dhivehi, traced.CommonName.Dhivehi)
	add("sex", current.Sex, traced.Sex)
	add("date_of_birth", current.DateOfBirth, traced.DateOfBirth)
	add("house_english", current.Address.House.English, traced.Address.House.English)
	add("house_dhivehi", current.Address.House.Dhivehi, traced.Address.House.Dhivehi)
	add("island_english", current.Address.Island.English, traced.Address.Island.English)
	add("island_dhivehi", current.Address.Island.Dhivehi, traced.Address.Island.Dhivehi)
	add("expiry_date", current.ExpiryDate, traced.ExpiryDate)
	add("serial_number", current.SerialNumber, traced.SerialNumber)
	add("blood_group", current.BloodGroup, traced.BloodGroup)
	return changed
}

// nonEmptyFields treats every populated field of a trace as new (used when there
// is no existing document to diff against).
func nonEmptyFields(traced store.IdentityDocument) []ChangedField {
	return diffIdentityFields(store.IdentityDocument{}, traced)
}

// changedFieldLabels reduces a delta to just the field names, so the review
// record can advertise WHAT changed without a second copy of the PII values.
func changedFieldLabels(changed []ChangedField) []string {
	labels := make([]string, 0, len(changed))
	for _, c := range changed {
		labels = append(labels, c.Field)
	}
	return labels
}
