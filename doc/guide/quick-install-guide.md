# Gopedia Quick Install Guide

`gopedia`를 5분 안에 띄우고, 10분 안에 첫 검색 시나리오를 확인하는 요약 가이드입니다.

## 사전 요구 사항 (최소)

- `git`
- `OPENAI_API_KEY`
- 권장 리소스: `4 vCPU / 8GB RAM` (3개 조합은 `8 vCPU / 16GB RAM`)
- **방법 A (빠른 실행)**: `go` 1.24+ 권장. 설치된 버전이 낮으면 `GOTOOLCHAIN=auto`로 `go.mod`에 맞는 툴체인을 자동으로 씁니다.
- **방법 B (Docker Compose)**: `docker` + `docker compose`

## 설치 (복사-붙여넣기)

### 방법 A — 빠른 실행 (`GOTOOLCHAIN` 빌드 + 로컬 서버)

PostgreSQL·API 키 등은 `.env`에 맞춰 두어야 합니다. 값 설명은 [install.md](./install.md)를 참고하세요.

```bash
cd /path/to/gopedia
cp .env.local.example .env
# Edit .env: OPENAI_API_KEY, POSTGRES_* 등
GOTOOLCHAIN=auto go build -o gopedia ./cmd/gopedia
./gopedia server --addr 0.0.0.0:18787
```

### 방법 B — Docker Compose로 한 번에

```bash
cd /path/to/gopedia
cp .env.local.example .env
# Edit .env and set:
# OPENAI_API_KEY=YOUR_OPENAI_KEY
# POSTGRES_PASSWORD=gopedia123!
docker compose -f docker-compose.dev.yml --env-file .env --profile app up -d --build
```

## 설치 확인

```bash
curl -s http://127.0.0.1:18787/api/health
```

- health JSON이 오면 성공

## 삭제

방법 B(Docker Compose)로 올린 스택만 해당합니다.

```bash
cd /path/to/gopedia
docker compose -f docker-compose.dev.yml --env-file .env down -v
```

## 10분 첫 시나리오 (Obsidian 권장)

1. Obsidian 노트 2~3개 생성
2. 노트 경로 ingest
3. `/api/search?q=키워드` 호출

## 3개 조합 확장

- 품질 테스트 필요: [gardener_gopedia](https://github.com/tojiuni/gardener_gopedia/blob/main/README.md) 설치
- Agent 연동 필요: [gopedia_mcp](https://github.com/tojiuni/gopedia_mcp/blob/main/README.md) 설치
- 전체 연동: Gopedia -> Gardener -> MCP 순서 권장

상세 가이드는 [install.md](./install.md)를 참고하세요.
