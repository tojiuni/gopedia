# Gopedia Install Guide (Detailed)

이 문서는 `gopedia`(지식 그래프 플랫폼)를 중심으로, `gardener_gopedia`(품질 테스트/운영), `gopedia_mcp`(AI Agent 연동)까지 한 번에 연결하는 설치 가이드입니다.

## 1) 사전 요구 사항

### 최소 환경

- Kubernetes: `v1.28+` (로컬은 Docker Compose 기반으로도 가능)
- CPU/Memory(개발 최소): `4 vCPU / 8GB RAM`
- CPU/Memory(3개 조합 권장): `8 vCPU / 16GB RAM`
- Disk: 여유 `20GB+` (컨테이너 이미지/벡터 인덱스 포함)

### 필수 도구

- `git`
- `docker` + `docker compose v2`
- `go 1.24+` (선택: CLI 사용 시)
- `python 3.11+` (선택: 스크립트/테스트)
- `node 18+` + `npm` (MCP 서버 실행 시)

### 필수 키/환경값

- `OPENAI_API_KEY` (임베딩/검색 품질 기능 사용 시)
- `POSTGRES_PASSWORD` (로컬 Postgres 보안)

## 2) 설치 (5분 이내)

아래는 로컬 개발 기준 복사-붙여넣기 명령어입니다.

```bash
cd /neunexus/gopedia
cp .env.local.example .env
sed -i 's/^OPENAI_API_KEY=.*/OPENAI_API_KEY=YOUR_OPENAI_KEY/' .env
sed -i 's/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=gopedia123!/' .env
docker compose -f docker-compose.dev.yml --env-file .env --profile app up -d --build
```

## 3) 설치 확인 방법

```bash
curl -s http://127.0.0.1:18787/api/health
```

성공 기준:

- JSON 응답에 상태(`ok` 또는 유사 health payload)가 표시되면 성공
- 브라우저/CLI에서 `http://127.0.0.1:18787/api/search?q=test` 호출 시 응답이 오면 정상

## 4) 삭제 방법

```bash
cd /neunexus/gopedia
docker compose -f docker-compose.dev.yml --env-file .env down -v
```

## 5) 3개 조합 설치 시나리오

### A. Gopedia 단독 (플랫폼만)

- 목적: 데이터 적재 + 검색 API만 빠르게 검증
- 설치: 본 문서 2장까지 수행

### B. Gopedia + Gardener (품질 테스트 운영)

1. Gopedia 기동 완료 확인
2. `/neunexus/gardener_gopedia/doc/guide/install-guide.md` 수행
3. Gardener에서 `GARDENER_GOPEDIA_BASE_URL=http://127.0.0.1:18787`로 연결

### C. Gopedia + MCP (Agent 연동)

1. Gopedia 기동 완료 확인
2. `/neunexus/gopedia_mcp/doc/guide/install-guide.md` 수행
3. MCP 클라이언트(Cursor/Claude/Gemini)에서 `gopedia_search` 호출 확인

### D. Full Stack (Gopedia + Gardener + MCP)

1. Gopedia 설치
2. Gardener 설치 후 스모크 평가 실행
3. MCP 설치 후 동일 질의를 Agent에서 재현
4. Gardener 결과와 Agent 답변 품질을 함께 비교

## 6) 첫 번째 시나리오 (10분 이내, Obsidian 권장)

1. Obsidian Vault에 샘플 노트 2~3개 작성 (프로젝트 개요, 회의 요약, TODO)
2. 노트 폴더를 Gopedia ingest 경로로 지정해 적재
3. `search?q=회의 요약`으로 핵심 문장 검색
4. Gardener에서 동일 질의로 정답셋(qrels) 간단 검증
5. MCP로 같은 질의를 Agent에서 실행해 응답 비교

Obsidian 권장 이유:

- 문서 연결(링크/백링크)이 지식 그래프 구조 검증에 유리
- 문서 변경 후 재적재 테스트가 빠름

## 7) 관련 문서

- 요약 설치: [quick-install-guide.md](./quick-install-guide.md)
- 기존 플랫폼 설치 참고: [install.md](./install.md)
- 실행 가이드: [run.md](./run.md)
