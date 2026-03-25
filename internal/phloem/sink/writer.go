package sink

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
	pb "gopedia/core/proto/gen/go"
	"gopedia/internal/phloem/nlpworker"
	"gopedia/internal/phloem/embedder"
	"gopedia/internal/phloem/types"
)

// DefaultSink writes to PostgreSQL and Qdrant (L1 doc summary + L2 per chunk).
type DefaultSink struct {
	pg     *pgxpool.Pool
	qdrant *qdrant.Client
	embed  embedder.Embedder
	redis  *redis.Client
	cfg    SinkConfig
}

// NewDefaultSink creates a sink. Any of PG/Qdrant/Embedder can be nil; those steps are skipped.
func NewDefaultSink(cfg SinkConfig) *DefaultSink {
	return &DefaultSink{
		pg:     cfg.PGPool,
		qdrant: cfg.Qdrant,
		embed:  cfg.Embedder,
		redis:  cfg.Redis,
		cfg:    cfg,
	}
}

// Write persists one document: PG documents + knowledge_l1/2/3 (UUID keys), then Qdrant vectors.
// Returns documents.id as a UUID string (canonical doc_id for IngestResponse and payloads).
func (s *DefaultSink) Write(ctx context.Context, msg *pb.RhizomeMessage, chunks []types.Chunk) (string, error) {
	version := 1
	sourceType := getEnv("GOPEDIA_SOURCE_TYPE", "md")

	allChunks := chunks
	l2ChildHash := computeL2ChildHash(allChunks)
	var l3Items []l3ToEmbed
	docUUIDStr := uuid.New().String()
	// knowledge_l1.id for Qdrant/TypeDB "document node" and NLP; set after L1 insert.
	var headL1ID uuid.UUID

	if s.pg != nil {
		versionID, err := ensurePipelineVersion(ctx, s.pg)
		if err != nil {
			return "", fmt.Errorf("postgres pipeline_version: %w", err)
		}

		var existingDocID uuid.UUID
		qerr := s.pg.QueryRow(ctx, `SELECT id FROM documents WHERE machine_id = $1`, msg.MachineId).Scan(&existingDocID)
		if qerr == nil {
			// Skip full re-ingest when head L1 already reflects this TOC/hash and has L2+L3.
			headL1SQL := `COALESCE(d.current_l1_id, (
				SELECT k2.id FROM knowledge_l1 k2 WHERE k2.document_id = d.id ORDER BY k2.created_at DESC NULLS LAST LIMIT 1
			))`
			var headMatches bool
			_ = s.pg.QueryRow(ctx,
				`SELECT EXISTS (
					SELECT 1 FROM documents d
					JOIN knowledge_l1 k ON k.document_id = d.id AND k.id = `+headL1SQL+`
					WHERE d.machine_id = $1 AND k.l2_child_hash = $2
				)`, msg.MachineId, l2ChildHash,
			).Scan(&headMatches)
			if headMatches {
				var hasL2 bool
				var hasL3 bool
				qL2 := `SELECT EXISTS (
					SELECT 1 FROM documents d
					JOIN knowledge_l1 k ON k.document_id = d.id AND k.id = ` + headL1SQL + `
					JOIN knowledge_l2 l2 ON l2.l1_id = k.id
					WHERE d.machine_id = $1)`
				_ = s.pg.QueryRow(ctx, qL2, msg.MachineId).Scan(&hasL2)
				qL3 := `SELECT EXISTS (
					SELECT 1
					  FROM documents d
					  JOIN knowledge_l1 k ON k.document_id = d.id AND k.id = ` + headL1SQL + `
					  JOIN knowledge_l2 l2 ON l2.l1_id = k.id
					  JOIN knowledge_l3 l3 ON l3.l2_id = l2.id
					 WHERE d.machine_id = $1
				)`
				_ = s.pg.QueryRow(ctx, qL3, msg.MachineId).Scan(&hasL3)
				if hasL2 && hasL3 {
					return existingDocID.String(), nil
				}
			}
		}

		metaJSON, _ := json.Marshal(msg.SourceMetadata)
		projectID := ingestProjectIDPtr()
		var docUUID uuid.UUID
		err = s.pg.QueryRow(ctx,
			`INSERT INTO documents (machine_id, title, source_metadata, version, version_id, created_at, project_id, source_type)
			 VALUES ($1, $2, $3, $4, $5, now(), $6, $7)
			 ON CONFLICT (machine_id) DO UPDATE SET
			   title = EXCLUDED.title,
			   source_metadata = EXCLUDED.source_metadata,
			   version = documents.version + 1,
			   version_id = EXCLUDED.version_id,
			   project_id = COALESCE(EXCLUDED.project_id, documents.project_id),
			   source_type = EXCLUDED.source_type
			 RETURNING id`,
			msg.MachineId, msg.Title, metaJSON, version, versionID, projectID, sourceType,
		).Scan(&docUUID)
		if err != nil {
			return "", fmt.Errorf("postgres documents: %w", err)
		}
		docUUIDStr = docUUID.String()

		l2Summaries := make([]string, 0, len(allChunks))
		for _, c := range allChunks {
			l2Summaries = append(l2Summaries, summarizeL2(msg.Title, c.Path, c.Text))
		}
		l1Summary := summarizeL1(msg.Title, l2Summaries)
		tocJSON, tocErr := tocJSONFromChunks(allChunks)
		if tocErr != nil {
			tocJSON = []byte("[]")
		}
		var l1UUID uuid.UUID
		err = s.pg.QueryRow(ctx,
			`INSERT INTO knowledge_l1 (title, source_metadata, version_id, summary, summary_hash, created_at, project_id, parent_id, source_type, toc, l2_child_hash, modified_at, document_id)
			 VALUES ($1, $2, $3, $4, $5, now(), $6, $7, $8, $9::jsonb, $10, now(), $11)
			 RETURNING id`,
			msg.Title, metaJSON, versionID, []byte(l1Summary), contentHashBin(l1Summary),
			projectID, nil, sourceType, tocJSON, l2ChildHash, docUUID,
		).Scan(&l1UUID)
		if err != nil {
			return docUUIDStr, fmt.Errorf("postgres knowledge_l1: %w", err)
		}
		headL1ID = l1UUID
		if _, err := s.pg.Exec(ctx, `UPDATE documents SET current_l1_id = $1 WHERE id = $2`, l1UUID, docUUID); err != nil {
			return docUUIDStr, fmt.Errorf("postgres documents current_l1_id: %w", err)
		}

		changed := make([]types.Chunk, 0, len(chunks))
		for i, c := range chunks {
			ver := c.Version
			if ver == 0 {
				ver = 1
			}
			sectionHashBin := contentHashBin(c.Text)
			var prevHashBin []byte
			var prevVersion int
			err = s.pg.QueryRow(ctx,
				`SELECT summary_hash, version
				   FROM knowledge_l2
				  WHERE l1_id = $1 AND section_id = $2
				  ORDER BY created_at DESC
				  LIMIT 1`,
				l1UUID, c.SectionID,
			).Scan(&prevHashBin, &prevVersion)
			if err == nil && len(prevHashBin) == sha256.Size && string(prevHashBin) == string(sectionHashBin) {
				continue
			}
			nextVersion := prevVersion + 1
			if prevVersion == 0 {
				nextVersion = ver
			}
			l2Summary := summarizeL2(msg.Title, c.Path, c.Text)
			var l2ID uuid.UUID
			l2Sort := (i + 1) * 1000
			err = s.pg.QueryRow(ctx,
				`INSERT INTO knowledge_l2 (l1_id, summary, version, sort_order, section_id, version_id, summary_bin, summary_hash, created_at, modified_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), now())
				 RETURNING id`,
				l1UUID, l2Summary, nextVersion, l2Sort, c.SectionID, versionID, []byte(l2Summary), sectionHashBin,
			).Scan(&l2ID)
			if err != nil {
				return docUUIDStr, fmt.Errorf("postgres knowledge_l2 %s: %w", c.SectionID, err)
			}

			sentences := splitSentencesEnglish(stripMarkdownHeadings(c.Text))
			if addr := os.Getenv("GOPEDIA_NLP_WORKER_GRPC_ADDR"); addr != "" {
				resp, err := nlpworker.New(addr).ProcessL2(ctx, &pb.NLPRequest{
					VersionId: versionID,
					L1Id:      headL1ID.String(),
					L2Id:      l2ID.String(),
					MachineId: msg.MachineId,
					Text:      stripMarkdownHeadings(c.Text),
				})
				if err != nil {
					slog.Warn("nlp worker call failed, falling back to local splitter", "err", err)
				} else if resp != nil && len(resp.Sentences) > 0 {
					sentences = resp.Sentences
				}
			}
			inserted, err := insertL3Sentences(ctx, s.pg, l2ID, sentences, versionID)
			if err != nil {
				return docUUIDStr, fmt.Errorf("postgres knowledge_l3 %s: %w", c.SectionID, err)
			}
			if len(inserted) > 0 {
				l3h := computeL3ChildHash(inserted)
				_, _ = s.pg.Exec(ctx, `UPDATE knowledge_l2 SET l3_child_hash = $1, modified_at = now() WHERE id = $2`, l3h, l2ID)
			}
			for _, ins := range inserted {
				l3Items = append(l3Items, l3ToEmbed{
					l1ID:      headL1ID,
					l3ID:      ins.l3ID,
					l2ID:      l2ID,
					sectionID: c.SectionID,
					text:      ins.text,
					versionID: versionID,
				})
			}
			changed = append(changed, c)
		}
		chunks = changed
	}

	collection := getEnv("QDRANT_COLLECTION", "gopedia_markdown")
	if s.cfg.CollectionForDomain != nil {
		if d := msg.SourceMetadata["domain"]; d != "" {
			collection = s.cfg.CollectionForDomain(d)
		}
	}

	if s.qdrant != nil && s.embed != nil {
		l1PayloadID := docUUIDStr
		if headL1ID != uuid.Nil {
			l1PayloadID = headL1ID.String()
		}
		l1Text := msg.Title
		if len(msg.Content) > 500 {
			l1Text += "\n" + msg.Content[:500]
		} else {
			l1Text += "\n" + msg.Content
		}
		vec, err := s.embed.Embed(ctx, l1Text)
		if err != nil {
			slog.Warn("embed L1 failed, skipping Qdrant L1", "err", err)
		} else {
			l1PointID := l1PayloadID + "_l1"
			if err := s.upsertPoint(ctx, collection, l1PointID, vec, l1PayloadID, version, sourceType, projectIDForPayload()); err != nil {
				return docUUIDStr, fmt.Errorf("qdrant L1: %w", err)
			}
		}

		for _, item := range l3Items {
			vec, err := s.embed.Embed(ctx, item.text)
			if err != nil {
				slog.Warn("embed L3 failed", "l3_id", item.l3ID, "err", err)
				continue
			}
			pointID := fmt.Sprintf("%s_l3_%s", docUUIDStr, item.l3ID.String())
			storedID, err := s.upsertL3Point(ctx, collection, pointID, vec, item)
			if err != nil {
				return docUUIDStr, fmt.Errorf("qdrant L3 %s: %w", item.l3ID, err)
			}
			if s.pg != nil {
				_, _ = s.pg.Exec(ctx, `UPDATE knowledge_l3 SET qdrant_point_id = $1 WHERE id = $2`, storedID, item.l3ID)
			}
		}
	}
	return docUUIDStr, nil
}

// upsertPoint writes an L1 document-vector point. Filter/ops keys use l1_id only (not documents.machine_id).
func (s *DefaultSink) upsertPoint(ctx context.Context, collection, pointID string, vector []float32, l1ID string, version int, sourceType string, projectID int64) error {
	qid := qdrantUUID(pointID)
	payload := map[string]interface{}{
		"l1_id":       l1ID,
		"version":     version,
		"source_type": sourceType,
		"project_id":  projectID,
	}
	points := []*qdrant.PointStruct{{
		Id:      qdrant.NewID(qid),
		Vectors: qdrant.NewVectors(vector...),
		Payload: payloadMapToPayload(payload),
	}}
	_, err := s.qdrant.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         points,
	})
	return err
}

func (s *DefaultSink) upsertL3Point(ctx context.Context, collection, pointID string, vector []float32, item l3ToEmbed) (string, error) {
	qid := qdrantUUID(pointID)
	keywordIDs := s.tuberKeywordIDs(ctx, item.text)
	st := sourceTypeForPayload()
	pid := projectIDForPayload()
	payload := map[string]interface{}{
		"l1_id":       item.l1ID.String(),
		"l2_id":       item.l2ID.String(),
		"l3_id":       item.l3ID.String(),
		"section_id":  item.sectionID,
		"version_id":  item.versionID,
		"keyword_ids": keywordIDs,
		"source_type": st,
		"project_id":  pid,
	}
	points := []*qdrant.PointStruct{{
		Id:      qdrant.NewID(qid),
		Vectors: qdrant.NewVectors(vector...),
		Payload: payloadMapToPayload(payload),
	}}
	_, err := s.qdrant.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         points,
	})
	return qid, err
}

func (s *DefaultSink) tuberKeywordIDs(ctx context.Context, text string) []int64 {
	if s.pg == nil {
		return nil
	}
	keywords := extractKeywords(text)
	if len(keywords) == 0 {
		return nil
	}
	out := make([]int64, 0, len(keywords))
	for _, kw := range keywords {
		id, err := tuberGetOrCreate(ctx, s.pg, s.redis, kw)
		if err != nil {
			slog.Warn("tuber get/create failed", "kw", kw, "err", err)
			continue
		}
		out = append(out, id)
	}
	return out
}

type l3ToEmbed struct {
	l1ID      uuid.UUID
	l3ID      uuid.UUID
	l2ID      uuid.UUID
	sectionID string
	text      string
	versionID int64
}

func payloadMapToPayload(m map[string]interface{}) map[string]*qdrant.Value {
	out := make(map[string]*qdrant.Value)
	for k, v := range m {
		switch x := v.(type) {
		case string:
			out[k] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: x}}
		case int64:
			out[k] = &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: x}}
		case int:
			out[k] = &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(x)}}
		case []int64:
			lv := &qdrant.ListValue{}
			for _, n := range x {
				lv.Values = append(lv.Values, &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: n}})
			}
			out[k] = &qdrant.Value{Kind: &qdrant.Value_ListValue{ListValue: lv}}
		case []string:
			lv := &qdrant.ListValue{}
			for _, s := range x {
				lv.Values = append(lv.Values, &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: s}})
			}
			out[k] = &qdrant.Value{Kind: &qdrant.Value_ListValue{ListValue: lv}}
		}
	}
	return out
}

func tocJSONFromChunks(chunks []types.Chunk) ([]byte, error) {
	type entry struct {
		ID string  `json:"id"`
		T  string  `json:"t"`
		D  int     `json:"d"`
		P  *string `json:"p,omitempty"`
	}
	out := make([]entry, 0, len(chunks))
	for _, c := range chunks {
		d := strings.Count(c.Path, ">") + 1
		if d < 1 {
			d = 1
		}
		t := strings.TrimSpace(c.Path)
		if idx := strings.LastIndex(t, " > "); idx >= 0 {
			t = strings.TrimSpace(t[idx+len(" > "):])
		}
		if t == "" {
			t = c.SectionID
		}
		out = append(out, entry{ID: c.SectionID, T: t, D: d})
	}
	return json.Marshal(out)
}

func computeL2ChildHash(chunks []types.Chunk) []byte {
	parts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		parts = append(parts, fmt.Sprintf("%s|%s|%s", c.SectionID, c.Path, contentHashHex(c.Text)))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return sum[:]
}

func computeL3ChildHash(rows []l3Insert) []byte {
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, fmt.Sprintf("%s|%s", r.l3ID.String(), contentHashHex(r.text)))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return sum[:]
}

func sourceTypeForPayload() string {
	return getEnv("GOPEDIA_SOURCE_TYPE", "md")
}

func projectIDForPayload() int64 {
	s := os.Getenv("GOPEDIA_PROJECT_ID")
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func ingestProjectIDPtr() interface{} {
	s := os.Getenv("GOPEDIA_PROJECT_ID")
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return v
}

func contentHashHex(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func contentHashBin(content string) []byte {
	h := sha256.Sum256([]byte(content))
	return h[:]
}

// qdrantUUID converts an arbitrary string into a deterministic RFC4122 UUIDv5-like value.
// The Qdrant Go client expects string point IDs to be valid UUIDs.
func qdrantUUID(s string) string {
	sum := sha256.Sum256([]byte(s)) // 32 bytes
	b := sum[:16]
	// Version 5 (0101) and variant RFC4122 (10xx).
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[0], b[1], b[2], b[3],
		b[4], b[5],
		b[6], b[7],
		b[8], b[9],
		b[10], b[11], b[12], b[13], b[14], b[15],
	)
}

func ensurePipelineVersion(ctx context.Context, pg *pgxpool.Pool) (int64, error) {
	// Minimal implementation: store a stable-ish name and minimal metadata.
	embedModel := os.Getenv("OPENAI_EMBEDDING_MODEL")
	name := "v1"
	if embedModel != "" {
		name = "v1-" + embedModel
	}

	byteaMeta := map[string]any{
		"encoding": "protobuf",
	}
	preMeta := map[string]any{
		"OPENAI_EMBEDDING_MODEL": embedModel,
		"OPENAI_API_KEY_SET":    os.Getenv("OPENAI_API_KEY") != "",
		"QDRANT_COLLECTION":     os.Getenv("QDRANT_COLLECTION"),
	}
	byteaMetaJSON, _ := json.Marshal(byteaMeta)
	preMetaJSON, _ := json.Marshal(preMeta)

	var id int64
	err := pg.QueryRow(ctx,
		`INSERT INTO pipeline_version (name, bytea_metadata, preprocessing_metadata)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		name, byteaMetaJSON, preMetaJSON,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	// If insert fails (e.g. due to permissions), fall back to "latest by name" lookup.
	err = pg.QueryRow(ctx, `SELECT id FROM pipeline_version WHERE name = $1 ORDER BY id DESC LIMIT 1`, name).Scan(&id)
	return id, err
}

var sentenceSplitRe = regexp.MustCompile(`[.!?]+`)
var keywordRe = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_-]{2,}`)

func stripMarkdownHeadings(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func splitSentencesEnglish(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Normalize whitespace a bit.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Very simple sentence splitter: split on punctuation and newlines.
	rawParts := sentenceSplitRe.Split(s, -1)
	out := make([]string, 0, len(rawParts))
	for _, p := range rawParts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func extractKeywords(s string) []string {
	s = strings.ToLower(s)
	matches := keywordRe.FindAllString(s, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func tuberGetOrCreate(ctx context.Context, pg *pgxpool.Pool, rdb *redis.Client, keyword string) (int64, error) {
	kw := strings.TrimSpace(strings.ToLower(keyword))
	if kw == "" {
		return 0, fmt.Errorf("empty keyword")
	}
	cacheKey := "kw:" + kw
	if rdb != nil {
		if v, err := rdb.Get(ctx, cacheKey).Result(); err == nil && v != "" {
			var id int64
			_, _ = fmt.Sscan(v, &id)
			if id != 0 {
				return id, nil
			}
		}
	}

	// Deterministic machine_id derived from the keyword for v1.
	sum := sha256.Sum256([]byte("kw:" + kw))
	id := int64(uint64(sum[0])<<56 | uint64(sum[1])<<48 | uint64(sum[2])<<40 | uint64(sum[3])<<32 | uint64(sum[4])<<24 | uint64(sum[5])<<16 | uint64(sum[6])<<8 | uint64(sum[7]))
	if id < 0 {
		id = -id
	}
	_, err := pg.Exec(ctx,
		`INSERT INTO keyword_so (machine_id, canonical_name, wikidata_id, aliases, lang, content_hash_bin, created_at, modified_at)
		 VALUES ($1, $2, '', '{}', 'en', $3, now(), now())
		 ON CONFLICT (machine_id) DO UPDATE SET modified_at = EXCLUDED.modified_at`,
		id, kw, sum[:],
	)
	if err != nil {
		return 0, err
	}
	if rdb != nil {
		_ = rdb.Set(ctx, cacheKey, fmt.Sprintf("%d", id), 0).Err()
	}
	return id, nil
}

type l3Insert struct {
	l3ID uuid.UUID
	text string
}

func insertL3Sentences(ctx context.Context, pg *pgxpool.Pool, l2ID uuid.UUID, sentences []string, versionID int64) ([]l3Insert, error) {
	var out []l3Insert
	var prev *uuid.UUID
	insIdx := 0
	for _, sent := range sentences {
		hHex := contentHashHex(sent)
		var dummy int
		dupErr := pg.QueryRow(ctx,
			`SELECT 1 FROM knowledge_l3 WHERE l2_id = $1 AND content_hash = $2 LIMIT 1`,
			l2ID, hHex,
		).Scan(&dummy)
		if dupErr == nil {
			continue
		}
		if dupErr != nil && !errors.Is(dupErr, pgx.ErrNoRows) {
			return nil, dupErr
		}
		insIdx++
		sortOrder := insIdx * 1000
		var l3ID uuid.UUID
		var parentArg interface{}
		if prev == nil {
			parentArg = nil
		} else {
			parentArg = *prev
		}
		insErr := pg.QueryRow(ctx,
			`INSERT INTO knowledge_l3 (l2_id, content, content_hash, version, version_id, sort_order, parent_id, created_at, modified_at)
			 VALUES ($1, $2, $3, 1, $4, $5, $6, now(), now())
			 RETURNING id`,
			l2ID, sent, hHex, versionID, sortOrder, parentArg,
		).Scan(&l3ID)
		if insErr != nil {
			return nil, insErr
		}
		out = append(out, l3Insert{l3ID: l3ID, text: sent})
		pid := l3ID
		prev = &pid
	}
	return out, nil
}

func summarizeL2(docTitle, parentPath, l2Text string) string {
	// Placeholder implementation (deterministic) until LLM summarizer is integrated:
	// - include light context
	// - keep it short
	body := strings.TrimSpace(stripMarkdownHeadings(l2Text))
	sents := splitSentencesEnglish(body)
	if len(sents) > 2 {
		sents = sents[:2]
	}
	sum := strings.Join(sents, ". ")
	sum = strings.TrimSpace(sum)
	if sum == "" {
		sum = body
	}
	if len(sum) > 400 {
		sum = sum[:400]
	}
	if docTitle != "" || parentPath != "" {
		prefix := strings.TrimSpace(docTitle)
		if parentPath != "" {
			if prefix != "" {
				prefix += " | "
			}
			prefix += parentPath
		}
		if prefix != "" {
			return prefix + " — " + sum
		}
	}
	return sum
}

func summarizeL1(docTitle string, l2Summaries []string) string {
	// Placeholder reduce: join section summaries and cap length.
	var b strings.Builder
	if docTitle != "" {
		b.WriteString(docTitle)
		b.WriteString(": ")
	}
	for i, s := range l2Summaries {
		if s == "" {
			continue
		}
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(s)
		if b.Len() > 1200 {
			break
		}
	}
	out := b.String()
	if len(out) > 1200 {
		out = out[:1200]
	}
	return strings.TrimSpace(out)
}
