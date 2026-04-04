# Installation guide

Install prerequisites so you can build and run Gopedia using [`docker-compose.dev.yml`](../../docker-compose.dev.yml) and optionally the `gopedia` CLI.

## Common requirements (all platforms)

| Tool | Notes |
|------|--------|
| **Git** | Clone the repository. |
| **Docker Engine** | Recent version with container networking and volume support. |
| **Docker Compose** | v2 plugin (`docker compose`) *or* standalone `docker-compose` (e.g. Homebrew on macOS). |
| **OpenAI API key** | Required for embeddings and search; set `OPENAI_API_KEY` in `.env`. |

The API image (see [`Dockerfile`](../../Dockerfile)) installs **Python 3.12**, PyTorch (CPU), and Go-built `api` inside the container. You do **not** need a full Python ML stack on the host unless you run scripts or tests there.

### Go toolchain (for local `gopedia` CLI)

[`go.mod`](../../go.mod) requires **Go 1.24.11+**. If your installed Go is older:

```bash
cd /path/to/gopedia
GOTOOLCHAIN=auto go version
GOTOOLCHAIN=auto go build -o gopedia ./cmd/gopedia
```

---

## macOS (Docker via Colima)

Colima runs Linux containers on macOS without Docker Desktop.

### 1. Install Colima and Docker CLI

Using Homebrew:

```bash
brew install colima docker docker-compose
```

Docker Compose: Apple Silicon and Homebrew often install the Compose **plugin** under `/opt/homebrew/lib/docker/cli-plugins`. If `docker compose` is missing, either:

- Add to `~/.docker/config.json`:

  ```json
  "cliPluginsExtraDirs": [ "/opt/homebrew/lib/docker/cli-plugins" ]
  ```

- Or use the standalone binary: `docker-compose` (hyphen) from the same brew formula.

### 2. Start Colima

```bash
colima start
```

Default socket: `unix://$HOME/.colima/default/docker.sock`.

### 3. Docker context

Point the CLI at Colima (often automatic):

```bash
docker context use colima
docker info
```

### 4. Credential helper pitfall (`docker pull` failures)

If `~/.docker/config.json` contains `"credsStore": "desktop"` but **Docker Desktop is not installed**, pulls may fail with `docker-credential-desktop` not found. Fixes:

- **A.** Remove `credsStore` from `config.json`, or set it to a helper you actually have.
- **B.** For a one-off session, use a minimal config and explicit socket:

  ```bash
  mkdir -p /tmp/docker-nocreds
  echo '{"auths":{}}' > /tmp/docker-nocreds/config.json
  export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"
  export DOCKER_CONFIG=/tmp/docker-nocreds
  docker pull hello-world
  ```

Then run Compose with the same `DOCKER_HOST` / `DOCKER_CONFIG` if needed.

### 5. Repository env file

```bash
cd /path/to/gopedia
cp .env.local.example .env
# Edit .env: set OPENAI_API_KEY and POSTGRES_PASSWORD at minimum
```

---

## macOS (Docker Desktop alternative)

1. Install [Docker Desktop for Mac](https://docs.docker.com/desktop/install/mac-install/).
2. Ensure **Docker Compose v2** is enabled (default in recent versions).
3. Use the same `.env` steps as above.

---

## Windows

### Recommended: Docker Desktop + WSL 2

1. Install [Docker Desktop for Windows](https://docs.docker.com/desktop/install/windows-install/) with the **WSL 2** backend.
2. Clone the repo **inside the Linux filesystem** (e.g. `\\wsl$\Ubuntu\home\you\gopedia`) for reasonable file-sharing performance with bind mounts.
3. Open **WSL** (Ubuntu or your distro), install **Go** (1.24+ or `GOTOOLCHAIN=auto`) if you want the `gopedia` CLI:

   ```bash
   # Example: follow https://go.dev/dl/ or your distro’s packages
   GOTOOLCHAIN=auto go build -o gopedia ./cmd/gopedia
   ```

4. From WSL, run Compose:

   ```bash
   docker compose -f docker-compose.dev.yml --env-file .env --profile app up -d --build
   ```

### PowerShell / CMD without WSL

Possible but not ideal: bind-mount performance and path semantics differ. Prefer WSL 2 for development.

### Compose command on Windows

Use the same syntax as Linux:

```powershell
docker compose -f docker-compose.dev.yml --env-file .env --profile app up -d --build
```

If only `docker-compose.exe` (v1 style) is available, replace `docker compose` with `docker-compose`.

---

## Linux

### Docker Engine + Compose plugin

Example (Debian/Ubuntu-style):

```bash
# Install Docker per https://docs.docker.com/engine/install/
sudo apt-get update
sudo apt-get install docker.io docker-compose-plugin
sudo usermod -aG docker "$USER"
# Log out and back in for group membership
docker compose version
```

### Colima on Linux

Colima exists on Linux for rootless-ish workflows; setup mirrors macOS (`colima start`, then `docker context use colima`). Use the same credential-helper caveats if you copy a config from another machine.

### Firewall

Ensure local ports **5432**, **6333**, **6334**, **1729**, **8000**, **18787**, **50051** are not blocked if you access services from the host (see [`docker-compose.dev.yml`](../../docker-compose.dev.yml) for defaults and `*_PUBLISH_*` overrides).

### `.env`

```bash
cp .env.local.example .env
# Edit secrets and keys
```

---

## Verify installation

```bash
docker version
docker compose version   # or: docker-compose version
cd /path/to/gopedia && GOTOOLCHAIN=auto go build -o /tmp/gopedia ./cmd/gopedia && /tmp/gopedia --help
```

---

## Korean guide addendum (merged)

이 섹션은 기존 설치 가이드에 한국어 기준 요약/시나리오를 통합한 내용입니다.

### 사전 요구 사항

- Kubernetes: `v1.28+` (로컬은 Docker Compose 기반으로도 가능)
- CPU/Memory(개발 최소): `4 vCPU / 8GB RAM`
- CPU/Memory(3개 조합 권장): `8 vCPU / 16GB RAM`
- Disk: 여유 `20GB+` (컨테이너 이미지/벡터 인덱스 포함)
- 필수 도구: `git`, `docker` + `docker compose v2`
- 선택 도구: `go 1.24+`, `python 3.11+`, `node 18+`
- 필수 환경값: `OPENAI_API_KEY`, `POSTGRES_PASSWORD`

### 설치 (5분 이내)

```bash
cd /neunexus/gopedia
cp .env.local.example .env
sed -i 's/^OPENAI_API_KEY=.*/OPENAI_API_KEY=YOUR_OPENAI_KEY/' .env
sed -i 's/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=gopedia123!/' .env
docker compose -f docker-compose.dev.yml --env-file .env --profile app up -d --build
```

### 설치 확인 방법

```bash
curl -s http://127.0.0.1:18787/api/health
```

성공 기준:

- JSON 응답에 상태(`ok` 또는 유사 health payload)가 표시되면 성공
- `http://127.0.0.1:18787/api/search?q=test` 호출 시 응답이 오면 정상

### 삭제 방법

```bash
cd /neunexus/gopedia
docker compose -f docker-compose.dev.yml --env-file .env down -v
```

### 3개 조합 설치 시나리오

#### A. Gopedia 단독 (플랫폼만)

- 목적: 데이터 적재 + 검색 API만 빠르게 검증
- 설치: 본 문서의 설치 단계까지 수행

#### B. Gopedia + Gardener (품질 테스트 운영)

1. Gopedia 기동 완료 확인
2. `/neunexus/gardener_gopedia/doc/guide/install-guide.md` 수행
3. Gardener에서 `GARDENER_GOPEDIA_BASE_URL=http://127.0.0.1:18787`로 연결

#### C. Gopedia + MCP (Agent 연동)

1. Gopedia 기동 완료 확인
2. `/neunexus/gopedia_mcp/doc/guide/install-guide.md` 수행
3. MCP 클라이언트(Cursor/Claude/Gemini)에서 `gopedia_search` 호출 확인

#### D. Full Stack (Gopedia + Gardener + MCP)

1. Gopedia 설치
2. Gardener 설치 후 스모크 평가 실행
3. MCP 설치 후 동일 질의를 Agent에서 재현
4. Gardener 결과와 Agent 답변 품질을 함께 비교

### 첫 번째 시나리오 (10분 이내, Obsidian 권장)

1. Obsidian Vault에 샘플 노트 2~3개 작성 (프로젝트 개요, 회의 요약, TODO)
2. 노트 폴더를 Gopedia ingest 경로로 지정해 적재
3. `search?q=회의 요약`으로 핵심 문장 검색
4. Gardener에서 동일 질의로 정답셋(qrels) 간단 검증
5. MCP로 같은 질의를 Agent에서 실행해 응답 비교

Obsidian 권장 이유:

- 문서 연결(링크/백링크)이 지식 그래프 구조 검증에 유리
- 문서 변경 후 재적재 테스트가 빠름

### 관련 문서

- 요약 설치: [quick-install-guide.md](./quick-install-guide.md)
- 실행 가이드: [run.md](./run.md)

Next: [run.md](run.md).
