# Transpiration 사용 및 성공 기준 (Ver1)

Transpiration: 키워드 질의 → Qdrant 검색 → TypeDB에서 해당 섹션 맥락 조회.  
성공 기준(Efficiency, Speed) 요약 및 사용 흐름을 정리합니다.

---

## 1. 흐름: Qdrant hit → section_id → TypeDB section

1. **키워드로 Qdrant 검색**  
   쿼리 텍스트를 임베딩한 뒤 `QDRANT_COLLECTION`에서 `search(query_vector, limit=5)` 호출.

2. **Hit payload에서 식별자 추출**  
   각 hit의 `payload`에 **`l1_id`**, `l2_id`, `l3_id`, `section_id` 등이 들어 있음(Phloem은 `documents.machine_id`·`doc_id`·`toc_path`·`level` 을 Qdrant에 넣지 않음).  
   동일 리비전의 L1/L3 포인트는 같은 **`l1_id`**(`knowledge_l1.id`)를 공유한다.

3. **TypeDB에서 해당 섹션 조회**  
   `section_id` 또는 PG에서 `l1_id`→`documents.id`로 풀어 `doc_id`를 맞춘 뒤 TypeDB에 쿼리할 수 있음.  
   예: `match $s isa section, has section_id $sid; get $sid;`  
   또는 document–section 계층:  
   `match $d isa document, has doc_id $doc_id; $c (parent: $d, child: $s) isa composition; $s isa section, has section_id $sid; get $doc_id, $sid;`

4. **Efficiency**  
   검색 결과로 본문 전체가 아닌 **TOC 기반 특정 섹션만** 추출 가능.  
   Qdrant hit의 `section_id` → TypeDB에서 해당 `section`의 식별자 및 관계를 조회함.

**실행 예**: `python scripts/verify_transpiration.py "keyword"`  
전체 E2E: `./scripts/run_transpiration_e2e.sh [sample.md] [keyword]` (테스트 환경: docker)

---

## 2. 성공 기준 요약

| 기준 | 검증 |
|------|------|
| **Identity** | 인제스트 응답의 `machine_id`·`doc_id` 일관성 → `go test ./internal/phloem -run TestIdentityMachineIDConsistency` |
| **Ontology** | TypeDB `match $d isa document;` 시 하위 TOC(섹션) 조회 가능 → §5·§7 (doc/test-initialize-and-run.md) 및 `verify_transpiration.py` |
| **Efficiency** | Qdrant hit의 `section_id` → TypeDB `section` 식별자 및 관계 조회 (위 흐름) |
| **Speed** | 단일 Markdown 파일 인게스트 **약 1초 이내** 목표. 벤치: `go test -bench` 또는 Root run 경과 시간 로그로 점검 가능. |

---

## 3. Speed 목표 (~1초/파일)

- **목표**: 단일 .md 파일 한 건 인게스트가 약 1초 이내 완료.
- **측정**:  
  - Root: `python -m property.root_props.run /path/to/one.md` 실행 시 경과 시간.  
  - 또는 Phloem gRPC `IngestMarkdown` 1회 호출 시간 (네트워크·PG·Qdrant·임베딩 포함).
- **참고**: 임베딩 호출 횟수(문서 1 + 섹션 수)와 네트워크 지연에 따라 변동. 목표치 미달 시 임베딩 배치·캐시 등으로 조정 가능.
