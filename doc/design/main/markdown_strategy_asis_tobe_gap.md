# Markdown Strategy: As-is / To-be / Gap

이 문서는 `markdown_strategy.md`를 실행 가능한 점검 문서로 보조하기 위한 체크리스트입니다.

- 기준 정책: `markdown_strategy.md`
- 프로세스 상세: `markdown_process.md`
- 사이드카 상세: `markdown_sidecar_data_strategy.md`
- 계층/그래프 상세: `hierarchy_embedding_strategy.md`

---

## 1) As-is (현재 구현 기준)

### Ingest / Chunking

- [x] Markdown는 L2/L3 계층으로 저장되며 L3가 검색 단위로 사용됨
- [x] 섹션 기반 분할(헤더/본문)과 문장/행 기반 분할 로직이 존재함
- [x] 테이블은 행 단위 분해 경로가 존재함
- [x] 코드 도메인은 라인/앵커 기반 L3 경로가 존재함
- [x] `project_machine_id` 기반 deterministic ID 생성 로직이 존재함

### Retrieval / Ranking

- [x] 검색은 Qdrant dense retrieval 중심으로 동작
- [x] PostgreSQL rich context 결합 경로가 존재함
- [x] Cross-encoder reranker(옵션) 경로가 존재함
- [x] API에서 `top_k`, `reranker`, `reranker_model` 파라미터 사용 가능
- [ ] TypeDB prefilter 상시 적용은 검색 경로에서 완전 구현 상태가 아님

### Sidecar

- [ ] JSON-LD entity/relation chunk 파이프라인이 전면 적용된 상태는 아님
- [ ] Parquet/CSV row/profile chunk 파이프라인이 전면 적용된 상태는 아님
- [ ] `[[data-ref:<dataset_id>]]` 기반 일관 참조의 전체 파이프라인 검증은 추가 필요

---

## 2) To-be Check-list (목표 상태)

### 정책/스키마

- [ ] `semantic(JSON-LD)` / `factual(Parquet/CSV/RDB)` 계층 분리가 운영 정책으로 고정되었는가
- [ ] Frontmatter(운영 메타)와 Sidecar(도메인 지식) 경계가 모든 문서/파이프라인에 반영되었는가
- [ ] ID 표준(`project_id`, `dataset_id`, `entity_id`, `file_id`, `chunk_id`)이 ingest/retrieval/store 전 구간에 일관 적용되는가
- [ ] 파일 네이밍 규칙이 고정되었는가 (`{title}.md`, `{dataset_id}.parquet`, `{dataset_id}.schema.jsonld`, `{dataset_id}.profile.json`)

### Ingest / Chunking

- [ ] L2/L3 chunk 정책 변경 시 `pipeline_version` 연동 재인덱싱 프로세스가 문서/운영에 반영되었는가
- [ ] Path-to-Context injection 템플릿이 canonical 형식으로 표준화되었는가
- [ ] 대형 테이블 raw 전량 임베딩 금지 규칙이 파이프라인에서 강제되는가
- [ ] sidecar chunk 규칙(JSON-LD entity/relation, row/profile)이 ingestion 단계에서 생성되는가

### Embedding

- [ ] chunk-aware embedding( markdown + sidecar 공통 )이 적용되었는가
- [ ] 입력 canonical 템플릿(`[Path][Type][IDs]<content>`)이 표준화되었는가
- [ ] dual index(small + parent) 전략이 실제 검색 흐름에 반영되었는가
- [ ] 모델/전처리 변경 시 A/B 품질 검증 루프가 운영되는가

### Retrieval / Ranking / Delivery

- [ ] 고정 오케스트레이션(`TypeDB -> factual prefilter -> vector -> rerank -> compose`)이 구현되었는가
- [ ] metadata-aware rerank가 기본 정책으로 동작하는가
- [ ] 1차 응답 ID 중심(`l3_id/l2_id/l1_id`, `dataset_id/entity_id`) + 2차 확장 정책이 적용되는가
- [ ] API 파라미터로 후보 수/컨텍스트 레벨/이웃 창 등 운영 튜닝이 가능한가

### 품질 운영

- [ ] `doc/rag-test-reports` + `gardener_gopedia` 지표로 변경 전후를 비교하는가
- [ ] 핵심 지표(`Recall@5`, `MRR@10`, `nDCG@10`, `P@3`) 회귀 알림 기준이 정의되었는가
- [ ] 서로 다른 dataset 간 절대 점수 직접 비교 금지 원칙이 준수되는가

---

## 3) Gap (우선순위)

### High

1. TypeDB prefilter 상시 적용의 검색 경로 통합
2. metadata-aware rerank(경로/헤더/태그/버전/권한 가중치) 실구현
3. sidecar chunk(JSON-LD/Parquet) 생성 및 벡터 적재 파이프라인 연결

### Medium

1. API 튜닝 파라미터 확장 (`candidate_limit`, `neighbor_window`, `context_level` 등)
2. dual index 범위 축소 로직의 운영 기본값 정교화
3. 1차 최소 토큰 전달 + 2차 확장 조회 UX 정착

### Low

1. 문서/운영 스크립트 자동 검증(규칙 위반 탐지) 보강
2. 온톨로지 버전 변경 시 영향 범위 자동 리포트

---

## 4) 실행 순서 제안 (권장)

1. **Rerank 고도화**: metadata-aware 점수 도입
2. **Prefilter 통합**: TypeDB + factual prefilter 고정 경로 완성
3. **Sidecar 파이프라인**: JSON-LD/Parquet chunk 생성-적재 자동화
4. **API/운영화**: 튜닝 파라미터 노출 + 품질 회귀 자동 점검

이 순서는 리스크를 낮추면서 검색 품질 체감을 빠르게 확보하는 데 초점을 둡니다.
