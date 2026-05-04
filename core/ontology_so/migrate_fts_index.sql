-- P3-D: PostgreSQL FTS GIN 인덱스 — 하이브리드 검색 성능 최적화
--
-- 목적: knowledge_l3.content에 대한 FTS(to_tsvector 'simple') GIN 인덱스를
--       생성해 pg_fts_search_l3() 전체 테이블 스캔을 인덱스 스캔으로 전환.
--
-- 적용 방법:
--   psql -U gopedia -d gopedia -f migrate_fts_index.sql
--
-- 참고:
--   - GOPEDIA_HYBRID_SEARCH_ENABLED=false(기본)이면 이 인덱스는 사용되지 않음.
--   - CONCURRENTLY 옵션으로 서비스 중단 없이 빌드 가능.
--   - 인덱스 생성 완료 후 gopedia-svc에 GOPEDIA_HYBRID_SEARCH_ENABLED=true 설정.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_l3_content_fts
    ON knowledge_l3
    USING gin (to_tsvector('simple', coalesce(content, '')));
