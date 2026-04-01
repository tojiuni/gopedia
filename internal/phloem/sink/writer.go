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
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
	pb "gopedia/core/proto/gen/go"
	"gopedia/internal/phloem/chunker"
	"gopedia/internal/phloem/codesplitter"
	"gopedia/internal/phloem/embedder"
	"gopedia/internal/phloem/nlpworker"
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
	sourceType := msg.SourceMetadata["source_type"]
	if sourceType == "" {
		sourceType = getEnv("GOPEDIA_SOURCE_TYPE", "md")
	}

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

		// IMP-01: title-based duplicate prevention — check before machine_id lookup.
		var titleDocID uuid.UUID
		var titleL2Hash []byte
		titleErr := s.pg.QueryRow(ctx,
			`SELECT k.document_id, k.l2_child_hash
			   FROM knowledge_l1 k
			  WHERE k.title = $1 AND k.source_type = $2
			  ORDER BY k.created_at DESC
			  LIMIT 1`,
			msg.Title, sourceType,
		).Scan(&titleDocID, &titleL2Hash)
		if titleErr == nil && string(titleL2Hash) == string(l2ChildHash) {
			// Same title and same content hash — return the existing document UUID.
			var existingUUIDByTitle uuid.UUID
			if lookupErr := s.pg.QueryRow(ctx,
				`SELECT id FROM documents WHERE id = $1`, titleDocID,
			).Scan(&existingUUIDByTitle); lookupErr == nil {
				return existingUUIDByTitle.String(), nil
			}
		}
		// titleErr == nil but hash differs → fall through to normal upsert (version bump).

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
		projectID := ingestProjectIDPtrFromMetadata(msg.SourceMetadata)
		// IMP-07: capture the gopedia version that performed this ingest.
		ingestVersion := getEnv("GOPEDIA_VERSION", "dev")
		var docUUID uuid.UUID
		err = s.pg.QueryRow(ctx,
			`INSERT INTO documents (machine_id, title, source_metadata, version, version_id, created_at, project_id, source_type, ingest_version)
			 VALUES ($1, $2, $3, $4, $5, now(), $6, $7, $8)
			 ON CONFLICT (machine_id) DO UPDATE SET
			   title = EXCLUDED.title,
			   source_metadata = EXCLUDED.source_metadata,
			   version = documents.version + 1,
			   version_id = EXCLUDED.version_id,
			   project_id = COALESCE(EXCLUDED.project_id, documents.project_id),
			   source_type = EXCLUDED.source_type,
			   ingest_version = EXCLUDED.ingest_version
			 RETURNING id`,
			msg.MachineId, msg.Title, metaJSON, version, versionID, projectID, sourceType, ingestVersion,
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
		secMap := make(map[string]uuid.UUID)
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
				var existing uuid.UUID
				if err2 := s.pg.QueryRow(ctx,
					`SELECT id FROM knowledge_l2 WHERE l1_id=$1 AND section_id=$2 ORDER BY created_at DESC LIMIT 1`,
					l1UUID, c.SectionID,
				).Scan(&existing); err2 == nil {
					secMap[c.SectionID] = existing
				}
				continue
			}
			nextVersion := prevVersion + 1
			if prevVersion == 0 {
				nextVersion = ver
			}
			l2Summary := summarizeL2(msg.Title, c.Path, c.Text)
			var l2ID uuid.UUID
			l2Sort := (i + 1) * 1000

			var parentArg interface{}
			if ps := c.ParentSectionID; ps != "" {
				if pu, ok := secMap[ps]; ok {
					parentArg = pu
				}
			}

			l2MetaJSON, metaErr := chunker.ChunkSourceMetadataJSON(c)
			if metaErr != nil {
				return docUUIDStr, fmt.Errorf("knowledge_l2 source_metadata %s: %w", c.SectionID, metaErr)
			}

			err = s.pg.QueryRow(ctx,
				`INSERT INTO knowledge_l2 (l1_id, parent_id, summary, version, sort_order, section_id, version_id, summary_bin, summary_hash, source_metadata, created_at, modified_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, now(), now())
				 RETURNING id`,
				l1UUID, parentArg, l2Summary, nextVersion, l2Sort, c.SectionID, versionID, []byte(l2Summary), sectionHashBin, l2MetaJSON,
			).Scan(&l2ID)
			if err != nil {
				return docUUIDStr, fmt.Errorf("postgres knowledge_l2 %s: %w", c.SectionID, err)
			}
			secMap[c.SectionID] = l2ID

			headingLine := extractFirstMarkdownHeadingLine(c.Text)
			var titleL3ID uuid.UUID
			hasTitleL3 := false
			if headingLine != "" {
				hHex := contentHashHex(headingLine)
				err = s.pg.QueryRow(ctx,
					`INSERT INTO knowledge_l3 (l2_id, content, content_hash, version, version_id, sort_order, parent_id, source_metadata, created_at, modified_at)
					 VALUES ($1, $2, $3, 1, $4, 0, NULL, '{}'::jsonb, now(), now())
					 RETURNING id`,
					l2ID, headingLine, hHex, versionID,
				).Scan(&titleL3ID)
				if err != nil {
					return docUUIDStr, fmt.Errorf("postgres knowledge_l3 title %s: %w", c.SectionID, err)
				}
				if _, err = s.pg.Exec(ctx, `UPDATE knowledge_l2 SET title_id = $1, modified_at = now() WHERE id = $2`, titleL3ID, l2ID); err != nil {
					return docUUIDStr, fmt.Errorf("postgres knowledge_l2 title_id %s: %w", c.SectionID, err)
				}
				hasTitleL3 = true
			}

			var chainAfter *uuid.UUID
			if hasTitleL3 {
				chainAfter = &titleL3ID
			}

			secType := c.SectionType
			if secType == "" {
				secType = types.SectionTypeHeading
			}

			// Code domain: use pre-computed L3Lines (1 line = 1 L3, tree-structured).
			if len(c.L3Lines) > 0 {
				all, codeAnchors, codeErr := insertCodeL3Lines(ctx, s.pg, l2ID, c.L3Lines, versionID)
				if codeErr != nil {
					return docUUIDStr, fmt.Errorf("postgres knowledge_l3 code %s: %w", c.SectionID, codeErr)
				}
				if len(all) > 0 {
					l3h := computeL3ChildHash(all)
					_, _ = s.pg.Exec(ctx, `UPDATE knowledge_l2 SET l3_child_hash = $1, modified_at = now() WHERE id = $2`, l3h, l2ID)
				}
				for _, ins := range codeAnchors {
					l3Items = append(l3Items, l3ToEmbed{
						l1ID:        headL1ID,
						l3ID:        ins.l3ID,
						l2ID:        l2ID,
						sectionID:   c.SectionID,
						sectionType: secType,
						text:        ins.text,
						versionID:   versionID,
						sourceType:  sourceType,
					})
				}
				changed = append(changed, c)
				continue
			}

			var sentences []string
			switch secType {
			case types.SectionTypeCode:
				lang := ""
				if c.SourceMetadata != nil {
					if v, ok := c.SourceMetadata["language"].(string); ok {
						lang = v
					}
				}
				sentences = codesplitter.SplitToL3(c.Text, lang)
			case types.SectionTypeImage:
				if t := strings.TrimSpace(c.Text); t != "" {
					sentences = []string{t}
				}
			case types.SectionTypeTable:
				sentences = splitMarkdownTableDataRows(c.Text)
			default:
				sentences = splitSentencesEnglish(stripMarkdownHeadings(c.Text))
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
			}

			semanticSplit := c.SemanticL3Split && (secType == types.SectionTypeHeading || secType == types.SectionTypeOrdered)
			sentences = expandSemanticL3Fragments(sentences, semanticSplit)
			inserted, err := insertL3Sentences(ctx, s.pg, l2ID, sentences, versionID, chainAfter)
			if err != nil {
				return docUUIDStr, fmt.Errorf("postgres knowledge_l3 %s: %w", c.SectionID, err)
			}
			forHash := make([]l3Insert, 0, len(inserted)+1)
			if hasTitleL3 {
				forHash = append(forHash, l3Insert{l3ID: titleL3ID, text: headingLine, sourceMetaJSON: nil})
			}
			forHash = append(forHash, inserted...)
			if len(forHash) > 0 {
				l3h := computeL3ChildHash(forHash)
				_, _ = s.pg.Exec(ctx, `UPDATE knowledge_l2 SET l3_child_hash = $1, modified_at = now() WHERE id = $2`, l3h, l2ID)
			}
			for _, ins := range inserted {
				l3Items = append(l3Items, l3ToEmbed{
					l1ID:        headL1ID,
					l3ID:        ins.l3ID,
					l2ID:        l2ID,
					sectionID:   c.SectionID,
					sectionType: secType,
					text:        ins.text,
					headingText: headingLine,
					versionID:   versionID,
					sourceType:  sourceType,
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
		qdrantProjectID := projectIDForPayloadFromMetadata(msg.SourceMetadata)
		l1PayloadID := docUUIDStr
		if headL1ID != uuid.Nil {
			l1PayloadID = headL1ID.String()
		}
		l1Text := msg.Title
		if len(msg.Content) > 500 {
			l1Text += "\n" + truncateUTF8ByBytes(msg.Content, 500)
		} else {
			l1Text += "\n" + msg.Content
		}
		vec, err := s.embed.Embed(ctx, l1Text)
		if err != nil {
			slog.Warn("embed L1 failed, skipping Qdrant L1", "err", err)
		} else {
			l1PointID := l1PayloadID + "_l1"
			if err := s.upsertPoint(ctx, collection, l1PointID, vec, l1PayloadID, version, sourceType, qdrantProjectID); err != nil {
				return docUUIDStr, fmt.Errorf("qdrant L1: %w", err)
			}
		}

		for _, item := range l3Items {
			// IMP-09: prepend section heading for contextual embedding.
			// This closes the semantic gap between short bullet-point L3 chunks and Korean queries.
			embedText := item.text
			if item.headingText != "" {
				embedText = item.headingText + "\n" + item.text
			}
			vec, err := s.embed.Embed(ctx, embedText)
			if err != nil {
				slog.Warn("embed L3 failed", "l3_id", item.l3ID, "err", err)
				continue
			}
			pointID := fmt.Sprintf("%s_l3_%s", docUUIDStr, item.l3ID.String())
			storedID, err := s.upsertL3Point(ctx, collection, pointID, vec, item, qdrantProjectID)
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

func (s *DefaultSink) upsertL3Point(ctx context.Context, collection, pointID string, vector []float32, item l3ToEmbed, projectID int64) (string, error) {
	qid := qdrantUUID(pointID)
	keywordIDs := s.tuberKeywordIDs(ctx, item.text)
	st := item.sourceType
	if st == "" {
		st = sourceTypeForPayload()
	}
	secType := item.sectionType
	if secType == "" {
		secType = types.SectionTypeHeading
	}
	payload := map[string]interface{}{
		"l1_id":        item.l1ID.String(),
		"l2_id":        item.l2ID.String(),
		"l3_id":        item.l3ID.String(),
		"section_id":   item.sectionID,
		"section_type": secType,
		"version_id":   item.versionID,
		"keyword_ids":  keywordIDs,
		"source_type":  st,
		"project_id":   projectID,
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
	l1ID        uuid.UUID
	l3ID        uuid.UUID
	l2ID        uuid.UUID
	sectionID   string
	sectionType string
	text        string
	// headingText is the section heading line prepended to text at embed time (IMP-09 contextual embedding).
	// Empty for code domain chunks (which already carry function-signature context in text).
	headingText string
	versionID   int64
	sourceType  string
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
		st := c.SectionType
		if st == "" {
			st = types.SectionTypeHeading
		}
		parts = append(parts, fmt.Sprintf("%s|%s|%s|%s|%s", c.SectionID, c.ParentSectionID, st, c.Path, contentHashHex(c.Text)))
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

// projectIDFromMetadata parses source_metadata["project_id"] when set (RegisterProject / Root).
func projectIDFromMetadata(meta map[string]string) (int64, bool) {
	if meta == nil {
		return 0, false
	}
	s := strings.TrimSpace(meta["project_id"])
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

func projectIDForPayloadFromMetadata(meta map[string]string) int64 {
	if v, ok := projectIDFromMetadata(meta); ok {
		return v
	}
	return projectIDForPayloadFromEnv()
}

func ingestProjectIDPtrFromMetadata(meta map[string]string) interface{} {
	if v, ok := projectIDFromMetadata(meta); ok {
		return v
	}
	return ingestProjectIDPtrFromEnv()
}

func projectIDForPayloadFromEnv() int64 {
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

func ingestProjectIDPtrFromEnv() interface{} {
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
		"OPENAI_API_KEY_SET":     os.Getenv("OPENAI_API_KEY") != "",
		"QDRANT_COLLECTION":      os.Getenv("QDRANT_COLLECTION"),
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

// sentenceSplitMaskedRe splits on Latin/CJK sentence punctuation after maskForSentenceSplit.
var sentenceSplitMaskedRe = regexp.MustCompile(`[.!?。！？…]+`)
var keywordRe = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_-]{2,}`)

// mdHeadingLineRe matches a single markdown heading line (same rule as chunker/heading.go).
var mdHeadingLineRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

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

// extractFirstMarkdownHeadingLine returns the first ATX heading line in the chunk (trimmed), e.g. "## Title".
func extractFirstMarkdownHeadingLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if mdHeadingLineRe.FindStringSubmatch(trim) != nil {
			return trim
		}
	}
	return ""
}

// expandSemanticL3Fragments splits long or clause-heavy sentences on commas, pipes, and
// semicolons so insertL3Sentences can chain them under the same L2 (first fragment parent for the rest).
func expandSemanticL3Fragments(sents []string, split bool) []string {
	if !split || len(sents) == 0 {
		return sents
	}
	var out []string
	for _, s := range sents {
		out = append(out, splitClauseFragments(s)...)
	}
	return out
}

func splitClauseFragments(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Keep short, simple sentences intact.
	if len(s) < 120 && !strings.Contains(s, "|") && strings.Count(s, ",") <= 1 {
		return []string{s}
	}
	var parts []string
	var b strings.Builder
	for _, r := range s {
		if r == ',' || r == '|' || r == ';' {
			p := strings.TrimSpace(b.String())
			b.Reset()
			if p != "" {
				parts = append(parts, p)
			}
			continue
		}
		b.WriteRune(r)
	}
	rest := strings.TrimSpace(b.String())
	if rest != "" {
		parts = append(parts, rest)
	}
	if len(parts) <= 1 {
		return []string{s}
	}
	return parts
}

// splitMarkdownTableDataRows returns one string per data row (after header + separator) for L3 embedding.
func splitMarkdownTableDataRows(tableMarkdown string) []string {
	lines := strings.Split(strings.TrimSpace(tableMarkdown), "\n")
	if len(lines) < 3 {
		t := strings.TrimSpace(tableMarkdown)
		if t == "" {
			return nil
		}
		return []string{t}
	}
	var rows []string
	for _, ln := range lines[2:] {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if !strings.Contains(t, "|") {
			break
		}
		rows = append(rows, t)
	}
	if len(rows) == 0 {
		t := strings.TrimSpace(tableMarkdown)
		if t == "" {
			return nil
		}
		return []string{t}
	}
	return rows
}

func splitSentencesEnglish(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Normalize whitespace a bit.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	masked, repls := maskForSentenceSplit(s)
	rawParts := sentenceSplitMaskedRe.Split(masked, -1)
	out := make([]string, 0, len(rawParts))
	for _, p := range rawParts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = unmaskSplitText(p, repls)
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
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
	l3ID           uuid.UUID
	text           string
	sourceMetaJSON []byte // stored in knowledge_l3.source_metadata; nil -> '{}'
}

// insertCodeL3Lines inserts pre-computed CodeLine rows into knowledge_l3.
// Each line gets its own source_metadata JSONB with tree-sitter node info.
// parent_id is set from ParentIdx (chunk-relative index) to build the tree structure.
// Returns (all inserted rows, anchor-only rows for Qdrant embedding).
func insertCodeL3Lines(
	ctx context.Context,
	pg *pgxpool.Pool,
	l2ID uuid.UUID,
	lines []types.CodeLine,
	versionID int64,
) (all []l3Insert, anchors []l3Insert, err error) {
	inserted := make([]uuid.UUID, len(lines))

	for i, ln := range lines {
		sortOrder := ln.LineNum * 1000 // preserve original line order

		var parentArg interface{}
		if ln.ParentIdx >= 0 && ln.ParentIdx < i {
			pid := inserted[ln.ParentIdx]
			if pid != uuid.Nil {
				parentArg = pid
			}
		}

		metaMap := map[string]any{
			"node_type":      ln.NodeType,
			"is_anchor":      ln.IsAnchor,
			"is_block_start": ln.IsBlockStart,
			"line_num":       ln.LineNum,
		}
		metaJSON, _ := json.Marshal(metaMap)

		hHex := contentHashHex(fmt.Sprintf("%d|%s", ln.LineNum, ln.Content))

		var l3ID uuid.UUID
		insErr := pg.QueryRow(ctx,
			`INSERT INTO knowledge_l3
			   (l2_id, content, content_hash, version, version_id,
			    sort_order, parent_id, source_metadata, created_at, modified_at)
			 VALUES ($1, $2, $3, 1, $4, $5, $6, $7::jsonb, now(), now())
			 ON CONFLICT DO NOTHING
			 RETURNING id`,
			l2ID, ln.Content, hHex, versionID, sortOrder, parentArg, metaJSON,
		).Scan(&l3ID)

		if insErr != nil {
			// Row may already exist (idempotent re-ingest): look up existing id.
			lookupErr := pg.QueryRow(ctx,
				`SELECT id FROM knowledge_l3 WHERE l2_id = $1 AND content_hash = $2 LIMIT 1`,
				l2ID, hHex,
			).Scan(&l3ID)
			if lookupErr != nil {
				return nil, nil, fmt.Errorf("insertCodeL3Lines row %d: %w", ln.LineNum, lookupErr)
			}
		}

		inserted[i] = l3ID
		row := l3Insert{l3ID: l3ID, text: ln.Content, sourceMetaJSON: metaJSON}
		all = append(all, row)
		if ln.IsAnchor && strings.TrimSpace(ln.Content) != "" {
			anchors = append(anchors, row)
		}
	}
	return all, anchors, nil
}

// insertL3Sentences stores sentence-level L3 rows. If chainAfter is non-nil, the first sentence's parent_id
// is set to that UUID (typically the section title L3 row at sort_order=0).
func insertL3Sentences(ctx context.Context, pg *pgxpool.Pool, l2ID uuid.UUID, sentences []string, versionID int64, chainAfter *uuid.UUID) ([]l3Insert, error) {
	var out []l3Insert
	var prev *uuid.UUID
	if chainAfter != nil {
		prev = chainAfter
	}
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
			`INSERT INTO knowledge_l3 (l2_id, content, content_hash, version, version_id, sort_order, parent_id, source_metadata, created_at, modified_at)
			 VALUES ($1, $2, $3, 1, $4, $5, $6, '{}'::jsonb, now(), now())
			 RETURNING id`,
			l2ID, sent, hHex, versionID, sortOrder, parentArg,
		).Scan(&l3ID)
		if insErr != nil {
			return nil, insErr
		}
		out = append(out, l3Insert{l3ID: l3ID, text: sent, sourceMetaJSON: nil})
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
	sum = annotateKoreanTerms(sum)
	sum = truncateUTF8ByBytes(sum, 400)
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
	out = truncateUTF8ByBytes(out, 1200)
	return strings.TrimSpace(out)
}

// annotateKoreanTerms appends English equivalents for known Korean technical terms found in
// the input text. This improves cross-language recall when documents contain Korean-only
// terminology that users query in English (IMP-03).
// Format: original text + " [en: term1 term2 ...]"
func annotateKoreanTerms(text string) string {
	type kv struct{ ko, en string }
	terms := []kv{
		{"파티션", "partition"},
		{"워터마킹", "watermarking"},
		{"스마트 싱크", "smart sink"},
		{"싱크", "sink"},
		{"인제스트", "ingest"},
		{"임베딩", "embedding"},
		{"임베드", "embed"},
		{"검색", "search retrieval"},
		{"파이프라인", "pipeline"},
		{"라우팅", "routing"},
		{"중복", "duplicate deduplication"},
		{"청크", "chunk"},
		{"벡터", "vector"},
		{"색인", "index"},
		{"요약", "summary"},
		{"복원", "restore"},
		{"계층", "layer hierarchy"},
		{"도메인", "domain"},
		{"컬렉션", "collection"},
		{"스키마", "schema"},
	}
	var annotations []string
	for _, t := range terms {
		if strings.Contains(text, t.ko) {
			annotations = append(annotations, t.en)
		}
	}
	if len(annotations) == 0 {
		return text
	}
	return text + " [en: " + strings.Join(annotations, " ") + "]"
}

// truncateUTF8ByBytes returns s unchanged if len(s) <= maxBytes; otherwise the longest
// prefix with byte length <= maxBytes that is whole UTF-8 (avoids invalid sequences for PG TEXT).
func truncateUTF8ByBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
