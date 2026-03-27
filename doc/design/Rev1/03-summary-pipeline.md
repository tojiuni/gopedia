# Bottom-up Summary Pipeline (Map-Reduce)

L2 요약 후 L1 요약을 만드는 **Bottom-up Map-Reduce** 스타일을 따른다.

## 1. L2 Summary

- **입력**: 각 L2의 원문 텍스트.
- **Context**: 요약 품질 향상을 위해 **부모 헤더 정보** 및 **L1(문서 제목/요약)** 정보를 context로 함께 전달한다.
  - 예: `[Document: {title}] [Section: {parent_path}] Content: {L2 text}`
- **출력**: L2별 핵심 요약문. PostgreSQL `knowledge_l2.summary` 등에 저장한다.
- **실행**: L2 개수만큼 병렬(또는 배치)로 LLM 호출 가능(Map 단계).

## 2. L1 Summary (Aggregation)

- **입력**: 위에서 생성된 **모든 L2 요약문**의 집합.
- **방식**: L2 요약들을 모아 **전체 문서의 L1 요약**을 한 번에 생성(Reduce 단계).
- **출력**: 문서 단위 요약. PostgreSQL **`knowledge_l1.summary`** (또는 정책에 따른 L1 루트 필드)에 저장한다. 인제스트 앵커는 **`documents`** 이지만, 루트 요약 본문은 L1 리비전 행에 둔다.
- **옵션**: L2 요약이 이미 정리되어 있으므로, L1에서는 짧은 지시(예: "다음 섹션 요약들을 종합해 문서 요약을 한 문단으로 작성")만 주면 된다.

## 3. 구현 위치

- **LLM 호출**: `internal/llm` — `Summarizer` 인터페이스 및 GPT 등 구현체. L2 요약 시 인자로 `parentPath`, `docTitle` 등 context를 받도록 설계.
- **오케스트레이션**: Phloem 파이프라인 또는 별도 Summary 서비스에서 Map(각 L2 → 요약) → Reduce(L2 요약들 → L1 요약) 순서로 호출.

## 4. Idempotency와의 연동

- L2 단위 해시가 **기존과 동일한 L2**는 Summary Pipeline에서도 스킵한다(이미 요약이 있음).
- **변경된 L2**만 L2 Summary 재생성; 변경이 많으면 L1 Summary도 재생성한다.
