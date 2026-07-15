package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// entityTermCap bounds how many search terms a single person's index row holds,
// so a pathologically wide link payload can't blow up the multikey index.
const entityTermCap = 500

// EntityIndex is the denormalized, per-person search projection. It is derived
// data — rebuilt from people + identity_documents + identity_document_history +
// links on every write — so search runs a single index-backed query instead of
// scanning arbitrary link payloads.
type EntityIndex struct {
	ID          primitive.ObjectID `bson:"_id"`
	TenantID    string             `bson:"tenantId"`
	PersonID    string             `bson:"personId"`
	NationalID  string             `bson:"nationalId,omitempty"`
	DateOfBirth string             `bson:"dateOfBirth,omitempty"`
	Islands     []string           `bson:"islands,omitempty"`
	Emails      []string           `bson:"emails,omitempty"`
	Phones      []string           `bson:"phones,omitempty"`
	Sources     []string           `bson:"sources,omitempty"`
	Terms       []string           `bson:"terms"`               // multikey searchable tokens (superset)
	NameTerms   []string           `bson:"nameTerms,omitempty"` // name-derived tokens (scored higher)
	Summary     EntitySummary      `bson:"summary"`
	UpdatedAt   time.Time          `bson:"updatedAt"`
}

// EntitySummary is the human-facing snippet returned with each search candidate.
type EntitySummary struct {
	Name         LocalizedText `bson:"name" json:"name"`
	CommonName   LocalizedText `bson:"commonName,omitempty" json:"common_name,omitempty"`
	NationalID   string        `bson:"nationalId,omitempty" json:"national_id,omitempty"`
	DateOfBirth  string        `bson:"dateOfBirth,omitempty" json:"date_of_birth,omitempty"`
	Island       LocalizedText `bson:"island,omitempty" json:"island,omitempty"`
	PrimaryEmail string        `bson:"primaryEmail,omitempty" json:"primary_email,omitempty"`
	PrimaryPhone string        `bson:"primaryPhone,omitempty" json:"primary_phone,omitempty"`
	Sources      []string      `bson:"sources,omitempty" json:"sources,omitempty"`
}

// EntitySearchQuery is the normalized filter the store understands. The service
// layer builds it from a search request; the store only runs the Mongo query.
type EntitySearchQuery struct {
	Tokens      []string // normalized free-text/name tokens (exact + prefix branches)
	NationalID  string
	Email       string
	Phone       string
	DateOfBirth string
	Island      string
}

func (q EntitySearchQuery) empty() bool {
	return len(q.Tokens) == 0 && q.NationalID == "" && q.Email == "" &&
		q.Phone == "" && q.DateOfBirth == "" && q.Island == ""
}

// SearchEntityIndex returns up to candidateCap candidate rows matching ANY of the
// supplied signals, tenant-scoped. Ordering is updatedAt desc to bound the set
// deterministically; final ranking happens in the service layer.
func (s *MongoStore) SearchEntityIndex(ctx context.Context, tenantID string, q EntitySearchQuery, candidateCap int) ([]EntityIndex, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store: mongo not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, errors.New("store: tenant required")
	}
	if q.empty() {
		return nil, nil
	}
	if candidateCap <= 0 {
		candidateCap = 500
	}

	branches := make([]bson.M, 0, len(q.Tokens)+6)
	if len(q.Tokens) > 0 {
		branches = append(branches, bson.M{"terms": bson.M{"$in": q.Tokens}})
		for _, tok := range q.Tokens {
			branches = append(branches, bson.M{"terms": primitive.Regex{Pattern: "^" + regexp.QuoteMeta(tok)}})
		}
	}
	if q.NationalID != "" {
		branches = append(branches, bson.M{"nationalId": q.NationalID})
	}
	if q.Email != "" {
		branches = append(branches, bson.M{"emails": q.Email})
	}
	if q.Phone != "" {
		branches = append(branches, bson.M{"phones": q.Phone})
	}
	if q.DateOfBirth != "" {
		branches = append(branches, bson.M{"dateOfBirth": q.DateOfBirth})
	}
	if q.Island != "" {
		branches = append(branches, bson.M{"islands": q.Island})
	}

	filter := bson.M{"tenantId": tenantID, "$or": branches}
	cursor, err := s.db.Collection("entity_index").Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}).SetLimit(int64(candidateCap)),
	)
	if err != nil {
		return nil, fmt.Errorf("store: search entity index: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	results := make([]EntityIndex, 0)
	for cursor.Next(ctx) {
		var ei EntityIndex
		if err := cursor.Decode(&ei); err != nil {
			return nil, fmt.Errorf("store: decode entity index: %w", err)
		}
		results = append(results, ei)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("store: entity index cursor: %w", err)
	}
	return results, nil
}

// RebuildEntityIndex recomputes and upserts the entity_index row for one person.
// It is derived data: callers should treat a failure as non-fatal (log and move
// on); BackfillEntityIndex reconciles drift.
func (s *MongoStore) RebuildEntityIndex(ctx context.Context, tenantID, personID string) error {
	if s == nil || s.db == nil {
		return errors.New("store: mongo not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	personID = strings.TrimSpace(personID)
	if tenantID == "" || personID == "" {
		return errors.New("store: tenant and person required")
	}

	var person Person
	err := s.db.Collection("people").FindOne(ctx, bson.M{"tenantId": tenantID, "personId": personID}).Decode(&person)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil // nothing to index yet
		}
		return fmt.Errorf("store: entity index load person: %w", err)
	}

	terms := newTermSet(entityTermCap)
	nameTerms := newTermSet(200)
	var islands, emails, phones, sources orderedSet

	nationalID := strings.ToUpper(strings.TrimSpace(person.NationalID))
	if nationalID != "" {
		terms.add(nationalID)
	}
	if email := normTerm(person.PrimaryEmail); email != "" {
		emails.add(email)
		terms.add(email)
	}
	if phone := strings.TrimSpace(person.PrimaryPhone); phone != "" {
		phones.add(phone)
		terms.add(normTerm(phone))
	}
	flattenInto(person.Attributes, terms)

	// Current identity documents (most-recent first). The newest supplies the summary.
	docs, err := s.ListIdentityDocuments(ctx, tenantID, personID, 100)
	if err != nil {
		return fmt.Errorf("store: entity index list documents: %w", err)
	}
	var summaryDoc *IdentityDocument
	dob := ""
	for i := range docs {
		d := &docs[i]
		if summaryDoc == nil {
			summaryDoc = d
			dob = strings.TrimSpace(d.DateOfBirth)
		}
		addNameTerms(d.Name, terms, nameTerms)
		addNameTerms(d.CommonName, terms, nameTerms)
		addIslandTerms(d.Address.Island, terms, &islands)
		terms.addText(d.Address.House.English)
		terms.addText(d.Address.House.Dhivehi)
		if docNat := strings.ToUpper(strings.TrimSpace(d.NationalID)); docNat != "" {
			if nationalID == "" {
				nationalID = docNat
			}
			terms.add(docNat)
		}
		terms.addText(d.SerialNumber)
		if v := strings.TrimSpace(d.DateOfBirth); v != "" {
			terms.add(normTerm(v))
		}
	}

	// Historical addresses (old islands/houses) come only from immutable snapshots.
	if err := s.appendHistoryAddresses(ctx, tenantID, personID, terms, &islands); err != nil {
		return err
	}

	// Linked source records (vehicle/property/business/...) — flatten scalar leaves.
	links, err := s.linksForPerson(ctx, tenantID, personID)
	if err != nil {
		return err
	}
	for i := range links {
		if src := strings.TrimSpace(links[i].Source); src != "" {
			sources.add(src)
		}
		flattenInto(links[i].Payload, terms)
	}

	summary := buildEntitySummary(person, summaryDoc, sources.values())

	now := time.Now().UTC()
	set := bson.M{
		"tenantId":    tenantID,
		"personId":    personID,
		"nationalId":  nationalID,
		"dateOfBirth": dob,
		"islands":     islands.values(),
		"emails":      emails.values(),
		"phones":      phones.values(),
		"sources":     sources.values(),
		"terms":       terms.values(),
		"nameTerms":   nameTerms.values(),
		"summary":     summary,
		"updatedAt":   now,
	}
	_, err = s.db.Collection("entity_index").UpdateOne(
		ctx,
		bson.M{"tenantId": tenantID, "personId": personID},
		bson.M{"$set": set, "$setOnInsert": bson.M{"_id": primitive.NewObjectID()}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("store: upsert entity index: %w", err)
	}
	return nil
}

// rebuildEntityIndexBestEffort refreshes a person's search index without failing
// the primary write: the index is derived data, so a rebuild error is logged and
// swallowed. BackfillEntityIndex (POST /v1/search/reindex) reconciles any drift.
func (s *MongoStore) rebuildEntityIndexBestEffort(ctx context.Context, tenantID, personID string) {
	if err := s.RebuildEntityIndex(ctx, tenantID, personID); err != nil && s.logger != nil {
		s.logger.Printf("entity index rebuild failed tenantId=%q personId=%q err=%v", tenantID, personID, err)
	}
}

// BackfillEntityIndex rebuilds the entity_index for every person in a tenant and
// returns the count processed. Used for cold-start population and drift repair.
func (s *MongoStore) BackfillEntityIndex(ctx context.Context, tenantID string) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store: mongo not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return 0, errors.New("store: tenant required")
	}

	cursor, err := s.db.Collection("people").Find(
		ctx,
		bson.M{"tenantId": tenantID},
		options.Find().SetProjection(bson.M{"personId": 1}),
	)
	if err != nil {
		return 0, fmt.Errorf("store: backfill list people: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	count := 0
	for cursor.Next(ctx) {
		var row struct {
			PersonID string `bson:"personId"`
		}
		if err := cursor.Decode(&row); err != nil {
			return count, fmt.Errorf("store: backfill decode: %w", err)
		}
		if row.PersonID == "" {
			continue
		}
		if err := s.RebuildEntityIndex(ctx, tenantID, row.PersonID); err != nil {
			return count, err
		}
		count++
	}
	if err := cursor.Err(); err != nil {
		return count, fmt.Errorf("store: backfill cursor: %w", err)
	}
	return count, nil
}

// appendHistoryAddresses pulls distinct island/house values from every identity
// document snapshot, capturing old addresses that no longer appear on the current
// document.
func (s *MongoStore) appendHistoryAddresses(ctx context.Context, tenantID, personID string, terms *termSet, islands *orderedSet) error {
	cursor, err := s.db.Collection("identity_document_history").Find(ctx, bson.M{
		"tenantId": tenantID,
		"personId": personID,
	})
	if err != nil {
		return fmt.Errorf("store: entity index history: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	for cursor.Next(ctx) {
		var h IdentityDocumentHistory
		if err := cursor.Decode(&h); err != nil {
			return fmt.Errorf("store: decode history: %w", err)
		}
		addIslandTerms(h.Snapshot.Address.Island, terms, islands)
		terms.addText(h.Snapshot.Address.House.English)
		terms.addText(h.Snapshot.Address.House.Dhivehi)
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("store: history cursor: %w", err)
	}
	return nil
}

func (s *MongoStore) linksForPerson(ctx context.Context, tenantID, personID string) ([]Link, error) {
	cursor, err := s.db.Collection("links").Find(ctx, bson.M{
		"tenantId": tenantID,
		"personId": personID,
	})
	if err != nil {
		return nil, fmt.Errorf("store: entity index links: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var links []Link
	for cursor.Next(ctx) {
		var link Link
		if err := cursor.Decode(&link); err != nil {
			return nil, fmt.Errorf("store: decode link: %w", err)
		}
		links = append(links, link)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("store: links cursor: %w", err)
	}
	return links, nil
}

func buildEntitySummary(person Person, doc *IdentityDocument, sources []string) EntitySummary {
	summary := EntitySummary{
		PrimaryEmail: person.PrimaryEmail,
		PrimaryPhone: person.PrimaryPhone,
		NationalID:   strings.TrimSpace(person.NationalID),
		Sources:      sources,
	}
	if doc != nil {
		summary.Name = doc.Name
		summary.CommonName = doc.CommonName
		summary.DateOfBirth = strings.TrimSpace(doc.DateOfBirth)
		summary.Island = doc.Address.Island
		if summary.NationalID == "" {
			summary.NationalID = strings.TrimSpace(doc.NationalID)
		}
	}
	return summary
}

// ── term helpers ────────────────────────────────────────────────────────────

var nonWord = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// Tokenize splits free text into normalized search tokens exactly as the entity
// index is built, so the query path and the stored terms stay in lockstep.
func Tokenize(s string) []string { return tokenizeText(s) }

// NormalizeTerm lowercases and trims a term, matching index normalization.
func NormalizeTerm(s string) string { return normTerm(s) }

func normTerm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// tokenizeText lowercases and splits on non-letter/non-number runs. Thaana letters
// are \p{L}, so Dhivehi tokenizes into words correctly.
func tokenizeText(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := nonWord.Split(strings.ToLower(s), -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// termSet is a capped, de-duplicated, insertion-ordered set of normalized terms.
type termSet struct {
	seen map[string]struct{}
	list []string
	cap  int
}

func newTermSet(capacity int) *termSet {
	return &termSet{seen: make(map[string]struct{}), cap: capacity}
}

func (t *termSet) add(s string) {
	s = normTerm(s)
	if s == "" {
		return
	}
	if _, ok := t.seen[s]; ok {
		return
	}
	if t.cap > 0 && len(t.list) >= t.cap {
		return
	}
	t.seen[s] = struct{}{}
	t.list = append(t.list, s)
}

func (t *termSet) addText(s string) {
	for _, tok := range tokenizeText(s) {
		t.add(tok)
	}
}

func (t *termSet) values() []string { return t.list }

// orderedSet keeps de-duplicated values in insertion order, preserving the exact
// string given (used for islands/emails/phones/sources).
type orderedSet struct {
	seen map[string]struct{}
	list []string
}

func (o *orderedSet) add(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	if o.seen == nil {
		o.seen = make(map[string]struct{})
	}
	if _, ok := o.seen[s]; ok {
		return
	}
	o.seen[s] = struct{}{}
	o.list = append(o.list, s)
}

func (o *orderedSet) values() []string { return o.list }

func addNameTerms(text LocalizedText, terms, nameTerms *termSet) {
	for _, tok := range tokenizeText(text.English) {
		terms.add(tok)
		nameTerms.add(tok)
	}
	for _, tok := range tokenizeText(text.Dhivehi) {
		terms.add(tok)
		nameTerms.add(tok)
	}
}

func addIslandTerms(island LocalizedText, terms *termSet, islands *orderedSet) {
	if en := normTerm(island.English); en != "" {
		islands.add(en)
		terms.add(en)
		terms.addText(island.English)
	}
	if dv := normTerm(island.Dhivehi); dv != "" {
		islands.add(dv)
		terms.add(dv)
		terms.addText(island.Dhivehi)
	}
}

// flattenInto walks a payload/attributes value and adds every scalar leaf's tokens
// to the term set. Nested docs/arrays are recursed; maps/slices themselves are not
// stored. This is why an arbitrary new linked record type becomes searchable with
// no code change.
func flattenInto(v any, terms *termSet) {
	switch val := v.(type) {
	case nil:
		return
	case primitive.M:
		for _, e := range val {
			flattenInto(e, terms)
		}
	case map[string]any:
		for _, e := range val {
			flattenInto(e, terms)
		}
	case primitive.A:
		for _, e := range val {
			flattenInto(e, terms)
		}
	case []any:
		for _, e := range val {
			flattenInto(e, terms)
		}
	default:
		if s := scalarString(v); s != "" {
			terms.addText(s)
		}
	}
}

// scalarString coerces a scalar (string/number/bool) to a trimmed string; anything
// else yields "".
func scalarString(v any) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case bool:
		return strconv.FormatBool(val)
	case int:
		return strconv.Itoa(val)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return ""
	}
}
