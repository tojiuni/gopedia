---
title: Gopedia Ver1 테스트용 샘플 문서
author: Gopedia
date: "2025-03-11"
tags:
  - test
  - markdown
  - e2e
---

# 서론

이 문서는 인게스트·TOC 추출·Transpiration 검증 테스트용 샘플입니다.

## 목표

키워드 검색과 섹션 단위 맥락 조회가 동작하는지 확인합니다.

## 구현 흐름

Root가 마크다운을 Phloem으로 보내고, Phloem은 PostgreSQL·Qdrant에 저장합니다.  
TypeDB 동기화는 Root에서 `TYPEDB_HOST` 설정 시 자동으로 수행됩니다.

### 검증 항목

- Identity: 한 문서의 모든 레코드가 동일한 `machine_id`를 가짐
- Ontology: TypeDB에서 document–section–composition 조회 가능
- Efficiency: Qdrant hit의 `section_id`로 TypeDB에서 해당 섹션 본문만 조회

## 참고

- 테스트 실행: `./scripts/run_transpiration_e2e.sh doc/sample.md "서론"`
- 상세 절차: [test-initialize-and-run.md](test-initialize-and-run.md)
