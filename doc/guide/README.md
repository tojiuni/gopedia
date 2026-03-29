# Gopedia setup and run guides

for installing and running Gopedia locally across platforms.

| Document | Purpose |
|----------|---------|
| [overview.md](overview.md) | What the dev stack is, architecture, and how pieces fit together |
| [install.md](install.md) | Prerequisites and installation on macOS (Colima), Windows, and Linux |
| [run.md](run.md) | Starting Docker services, DB initialization, CLI, and E2E scripts |
| [agent-interop.md](agent-interop.md) | Agent-oriented API: JSON search (`detail` / `fields`), jobs, health/deps, structured errors |

For the production-style stack (external Docker networks), see the root [docker-compose.yml](../../docker-compose.yml) and [.env.example](../../.env.example).
