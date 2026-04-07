# TODO

## tree.py — 프로젝트 지식 트리 조회 모듈

**배경**

`flows/xylem_flow/tree.py`는 knowledge_l1 노드를 트리 구조로 조회하는 모듈로,
프로젝트 단위 문서 탐색 UI(뷰어/탐색기) 구현을 위한 scaffolding이었으나
실제 호출처가 없어 `refactor/cleanup-unused-code` 브랜치에서 제거됨.

**구현 예정 기능**

- `fetch_project_l1_nodes(conn, project_id)` — 특정 프로젝트의 L1 노드 목록을 flat list로 조회 (id, parent_id, title, source_type, document_id)
- `build_project_l1_tree(conn, project_id)` — parent_id 관계를 재귀적으로 중첩 트리로 변환, 루트 노드 배열 반환
- `get_project_tree_for_viewer(conn, project_id)` — API 응답용 JSON 래퍼 (`{ project_id, tree: [...] }`)

**활용 시나리오**

- 지식 그래프 탐색 UI (문서 목차 트리 뷰어)
- `/api/tree` 엔드포인트 추가 시 즉시 연결 가능
- 멀티 프로젝트 문서 구조 시각화

**참고**

- DB 스키마: `knowledge_l1` 테이블 (`id`, `parent_id`, `title`, `source_type`, `document_id`, `project_id`)
- 복원 시 `flows/xylem_flow/__init__.py` `__all__`에 재추가 필요

---

## 인덱스 초기화 API — `DELETE /api/index` (또는 리셋 스크립트)

**배경**

gardener_gopedia 평가 파이프라인(gopedia-eval-pipeline 플랜)에서 클린 re-ingest 시
PostgreSQL + Qdrant 인덱스를 초기화할 메커니즘이 필요하나, 현재 Gopedia에
삭제/초기화 API가 없음. 평가 당시 직접 SQL + Qdrant API로 우회 처리함.

**구현 예정 내용**

- `DELETE /api/index` 또는 `POST /api/index/reset` 엔드포인트 추가
- 대상 테이블 (외래키 순서 고려): `keyword_so` → `knowledge_l3` → `knowledge_l2` → `knowledge_l1` → `documents` → `projects`
- Qdrant collection points 삭제 (collection 자체 재생성 또는 `DELETE /collections/{name}/points`)
- dry-run 모드 지원 ("삭제 예정 N건" 확인 후 실행)
- project_id 단위 부분 삭제 지원 (선택적)

**참고**

- 스키마: `core/ontology_so/postgres_ddl.sql`
- 기존 리셋 스크립트: `scripts/reset_rhizome_docker.py`

---

## 검색 품질 개선 후보 (평가 메트릭 기반)

**배경**

gardener_gopedia 평가 파이프라인으로 universitas/ 문서 인제스트 후 Recall@5, MRR@10,
nDCG@10, P@3 메트릭 측정. 현재 메트릭은 기준치 충족 (T14 SKIPPED)이나,
향후 문서 규모 확대 또는 도메인 변경 시 재진단 필요. 아래 항목은 병목 발생 시 개선 후보.

**개선 후보 목록**

### 1. 청킹 전략
- 현재: heading-based chunking (`internal/phloem/chunker/`)
- 후보: semantic chunking 또는 heading + fixed-size 하이브리드
- 증상: 특정 쿼리에서 관련 내용이 청크 경계에서 잘려 hit 실패

### 2. 임베딩 모델
- 현재: `text-embedding-3-small` (OpenAI)
- 후보: `text-embedding-3-large` 또는 multilingual-e5-large (로컬)
- 관련 파일: `internal/phloem/embedder/openai.go`, `flows/xylem_flow/retriever.py`

### 3. 검색 파라미터 및 랭킹
- 현재: Qdrant 벡터 검색 + neighbor_window 기반 컨텍스트 확장
- 후보: 하이브리드 검색 (벡터 + 키워드), 리랭킹 레이어 추가
- 관련 파일: `flows/xylem_flow/retriever.py::retrieve_and_enrich()`
- 파라미터: `candidate_limit`, `final_limit`, `neighbor_window`, `context_level`

### 4. Cross-Encoder 로딩/실행 구조 최적화
- 현재: 검색 요청마다 Python subprocess가 새로 실행되어 `flows/xylem_flow/retriever.py`의 `_get_cross_encoder()` 호출 시 무거운 Cross-Encoder 모델을 반복 로드
- 문제: 모델 디스크 로딩/초기화 비용이 누적되어 검색 레이턴시 오버헤드 발생
- 후보: 모델을 메모리에 상주시켜 재사용하는 별도 서비스(gRPC 등)로 분리
- 기대 효과: warm process 기반 재사용으로 리랭킹 지연 감소 및 처리량 안정화

**진단 방법**

- Gardener eval 파이프라인으로 Recall@5 < 0.5 이면 개선 필요
- per-query 분석으로 실패 쿼리 패턴 파악 후 위 후보 중 택일
- 한 번에 하나만 수정하고 재평가
