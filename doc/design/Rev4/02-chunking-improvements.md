# 02. Chunking 개선 필요 사항 — Rev4

> 평가 기준: Gardener eval 파이프라인 IR 메트릭 (Recall@5, MRR@10, nDCG@10)  
> 원칙: 한 번에 하나만 변경 → 재평가 → 반복
> Rev4 확장안: [`03-atomic-l3-metadata-strategy.md`](./03-atomic-l3-metadata-strategy.md)

---

## I. 구조적 문제

### I-1. 헤딩 기반 청크의 크기 불균형

**문제**  
`ByHeadingChunker`는 헤딩 단위로만 자르므로 섹션 길이에 의존한다.
짧은 섹션(`## 설치` 3줄)과 긴 섹션(`## 상세 구현` 200줄)이 동일하게 1 L2 청크로 처리된다.
임베딩 모델에는 토큰 상한이 있고, 너무 짧은 청크는 컨텍스트 부족, 너무 긴 청크는 truncation 위험이 있다.

**개선 방향**  
- 청크 생성 후 토큰 수 측정 → 상한 초과 시 문단/문장 단위로 재분할
- 하한 미만 청크는 인접 청크와 병합 (작은 섹션 연속 시)
- 참고 파일: `internal/phloem/chunker/heading.go`, `flows/xylem_flow/retriever.py::_token_count()`

---

### I-2. 청크 경계에서 컨텍스트 절단

**문제**  
"다음 헤딩 직전까지"로 경계를 결정하므로, 논리적으로 이어지는 내용이
헤딩 경계에서 잘릴 수 있다.  
예: `## 배경`에서 시작된 설명이 `## 구현`에서 이어지는 경우,
`## 배경` 청크만 hit되면 `## 구현`의 연속 내용이 검색에서 누락된다.

**개선 방향**  
- 인접 섹션 간 sliding overlap 도입 (앞 섹션 마지막 N문장을 다음 청크에 포함)
- 또는 neighbor_window 확장으로 검색 시 보완 (현재 `retrieve_and_enrich`의 `neighbor_window` 파라미터)
- 단, overlap은 Qdrant 포인트 중복 저장 비용 증가 주의

---

### I-3. 코드 도메인의 nested 함수 청크 누락

**문제**  
`CodeChunker`는 `parent_idx == -1` top-level anchor만 L2 경계로 사용한다.
클래스 메서드, 중첩 함수는 독립 L2 청크가 되지 않고 부모 청크의 L3 라인으로만 저장된다.
따라서 "Bar.method가 무슨 일을 하는가" 같은 쿼리는 `Bar` 청크 전체를 검색해야 hit 가능하다.

**개선 방향**  
- 클래스 내 메서드를 별도 L2로 분리하고 `parent_section_id`로 클래스와 연결
- `toc.CodeTOCParser` 파서가 중첩 depth를 제공하므로 depth=1 anchor도 L2 경계로 확장
- 참고 파일: `internal/phloem/chunker/code.go::buildCodeChunks()`

---

### I-4. `BySymbolChunker` 미구현

**문제**  
`symbol.go`는 TOC 노드를 그대로 청크 텍스트로 사용하는 placeholder다.
실제 소스 코드의 symbol 범위(byte offset)가 아닌 헤딩 텍스트만 들어간다.

**개선 방향**  
- `CodeTOCParser`에서 symbol의 start/end byte offset 또는 line range 제공
- `BySymbolChunker`에서 해당 범위의 원본 소스 텍스트를 추출
- 또는 `CodeChunker`에 통합하고 `BySymbolChunker` 제거

---

## II. 구조화 블록 파생 (`ExpandStructuredChunks`) 문제

### II-1. 중첩 목록 미처리

**문제**  
`structured.go`는 ordered list (`1. item`) 감지만 구현되어 있고,
unordered list (`- item`, `* item`) 및 **중첩 목록**은 산문(prose)으로 처리된다.

**개선 방향**  
- unordered list 감지 추가 (`^\s*[-*+]\s+`)
- 중첩 목록은 depth 기준으로 부모 항목과 `ParentSectionID` 연결

---

### II-2. 테이블 셀 단위 검색 불가

**문제**  
GFM 테이블 전체가 1개의 L2 청크로 저장된다.
"테이블에서 특정 행/열 값" 쿼리는 테이블 청크가 hit되더라도
어느 셀에 해당하는지 알 수 없다.

**개선 방향**  
- 테이블 청크 내 각 행을 L3로 저장 (헤더 컬럼 + 행 값 형식)
- `source_metadata`에 `row_index`, `column_headers` 포함
- 참고: `structured.go::tryExtractGFMTable()`이 이미 `headers`, `column_count` 메타를 추출함

---

## III. 임베딩 관련

### III-1. 짧은 청크의 임베딩 품질 저하

**문제**  
3~5줄 이하의 매우 짧은 청크(이미지 alt text, 짧은 ordered item 등)는
임베딩 벡터의 표현력이 낮아 semantic search hit률이 저하된다.

**개선 방향**  
- 저장 단계에서는 L3를 원자적으로 유지 (자동 병합 최소화)
- 벡터화 시 `breadcrumb/fact_tags/domain_tags`를 prefix로 결합해 의미 보강
- 검색 단계에서 `block_group_id`, `prev/next` 기반으로 context를 동적으로 확장
- 즉, "저장 시 병합"보다 "조회 시 메타 기반 결합"을 우선 적용

---

### III-2. 임베딩 모델 단일화

**문제**  
현재 `text-embedding-3-small` (OpenAI) 또는 local multilingual-e5-large 중 하나를
프로젝트 단위로 선택한다. 동일 컬렉션에 다른 모델로 임베딩된 벡터가 혼재하면
검색 품질이 불안정해진다.

**개선 방향**  
- 프로젝트 생성 시 임베딩 모델을 고정하고 `projects.source_metadata`에 기록
- re-ingest 시 동일 모델 사용 강제
- 참고 파일: `flows/xylem_flow/project_config.py::resolve_retrieval_settings()`

---

## IV. 우선순위 요약

| 우선순위 | 항목 | 예상 효과 | 난이도 |
|:--------:|------|-----------|:------:|
| ★★★ | I-1. 청크 크기 불균형 | Recall, 임베딩 품질 직접 개선 | 중 |
| ★★★ | I-3. 코드 nested 함수 누락 | 코드 도메인 검색 품질 개선 | 중 |
| ★★☆ | I-2. 청크 경계 절단 | 긴 문서 검색 품질 개선 | 중 |
| ★★☆ | II-2. 테이블 행 단위 L3 | 테이블 내용 검색 가능 | 하 |
| ★☆☆ | II-1. 중첩 목록 미처리 | 목록 heavy 문서 개선 | 하 |
| ★☆☆ | III-1. 짧은 청크 메타 보강 | 임베딩 품질 소폭 개선 | 하 |
| ★☆☆ | I-4. BySymbolChunker 미구현 | 코드 symbol 검색 정밀도 | 고 |
| ★☆☆ | III-2. 임베딩 모델 단일화 | 운영 안정성 | 하 |

> **진단 방법**: Gardener eval 파이프라인으로 Recall@5 < 0.5 확인 후
> per-query 실패 패턴에 따라 위 항목 중 해당하는 것부터 적용.
> 참고: `doc/design/plan/TODO.md` — 검색 품질 개선 후보 섹션
