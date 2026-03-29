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

Next: [run.md](run.md).
