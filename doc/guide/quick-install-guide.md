# Gopedia Quick Install Guide

`gopedia`를 5분 안에 띄우고, 10분 안에 첫 검색 시나리오를 확인하는 요약 가이드입니다.

## 사전 요구 사항 (최소)

- `docker` + `docker compose`
- `git`
- `OPENAI_API_KEY`
- 권장 리소스: `4 vCPU / 8GB RAM` (3개 조합은 `8 vCPU / 16GB RAM`)

## 설치 (복사-붙여넣기)

```bash
cd /neunexus/gopedia
cp .env.local.example .env
sed -i 's/^OPENAI_API_KEY=.*/OPENAI_API_KEY=YOUR_OPENAI_KEY/' .env
sed -i 's/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=gopedia123!/' .env
docker compose -f docker-compose.dev.yml --env-file .env --profile app up -d --build
```

## 설치 확인

```bash
curl -s http://127.0.0.1:18787/api/health
```

- health JSON이 오면 성공

## 삭제

```bash
cd /neunexus/gopedia
docker compose -f docker-compose.dev.yml --env-file .env down -v
```

## 10분 첫 시나리오 (Obsidian 권장)

1. Obsidian 노트 2~3개 생성
2. 노트 경로 ingest
3. `/api/search?q=키워드` 호출

## 3개 조합 확장

- 품질 테스트 필요: `gardener_gopedia` 설치
- Agent 연동 필요: `gopedia_mcp` 설치
- 전체 연동: Gopedia -> Gardener -> MCP 순서 권장

상세 가이드는 [install-guide.md](./install-guide.md)를 참고하세요.
