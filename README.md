# XBoard

<div align="center">

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)
![SQLite](https://img.shields.io/badge/SQLite-Embedded-003B57.svg)
![License](https://img.shields.io/badge/License-MIT-yellow.svg)

</div>

XBoard is a Go-based panel + agent system for subscription and traffic management. It ships as a single binary with embedded admin/user frontends, SQLite storage, background jobs, and deployment tooling for panel/agent nodes.

## ✨ Highlights

- **Go + Chi**: Single-runtime backend focused on panel APIs, agent communication, and background jobs.
- **SQLite + Embedded Migration**: Out-of-the-box embedded database with automatic Goose-style migrations on startup.
- **Built-in Scheduler**: Traffic aggregation, telemetry sampling, and notification jobs run in-process.
- **Embedded Frontends**: Admin/User SPA assets are bundled in the backend binary by default.
- **MCP Server (LLM 运维)**: Built-in JSON-RPC 2.0 + SSE endpoint for LLM tool calling — query status, read logs, manage keys, all through standardized MCP protocol.
- **Agent Log Upload**: Agents tail and upload local logs via existing gRPC connection (in-memory cache, no DB pressure).
- **Scripted Deployment**: Panel/agent install, bootstrap, and uninstall flows are provided by `deploy/*.sh` scripts.

## 📁 Directory

```
cmd/
├── xboard/           # Panel main program (serve, tui, user, config, etc.)
└── agent/            # Agent program
internal/             # API, Service, Repository, Jobs, Async, Bootstrap...
pkg/, test/           # Shared libraries and contract/integration tests
web/user-vite/        # Unified frontend (Vite + React + shadcn/ui)
scripts/              # Build and test scripts
Dockerfile            # Go multi-stage build
config.example.yml    # YAML configuration example
```

For more details, stage goals, and architectural constraints, see `coding.md`.

## 🚀 Quick Start

### Local Run

```bash
# 1. Prepare Go toolchain
source ~/.gvm/scripts/gvm && gvm use go1.25.1   # Or any Go 1.25+

# 2. Initialize configuration
mkdir -p data
cp config.example.yml config.yml # Use YAML config

# 3. Start service
go run ./cmd/xboard serve
```

Default listens on `0.0.0.0:8080`. First start will automatically execute SQLite migrations in `data/xboard.db`.

### CLI Commands

The `xboard` binary provides several subcommands:

- `xboard serve`: Start the HTTP server (default).
- `xboard user`: User management (create, list, reset-password, etc.).
- `xboard config`: View or update system configuration.
- `xboard migrate`: Manage database migrations.
- `xboard backup`: Backup database.
- `xboard restore`: Restore database from backup.
- `xboard job`: Manage background jobs.
- `xboard version`: Show version information.

### Initialization Wizard

- If no admin account exists in the database, the HTTP service automatically redirects to `/install` to show the initialization interface.
- The wizard allows creating the first admin account with "Username (optional) / Email (optional) + Password".
- Alternatively, use CLI: `go run ./cmd/xboard user create --email admin@example.com --password secret --admin`.

### Admin Frontend

- Admin Frontend uses Vite/React, built assets are embedded in the binary.
- Access `/{secure_path}/login` (default `/admin/login`) in browser to open login page.
- Can be disabled via config `ui.admin.enabled: false` for custom CDN deployment.

### User Frontend

- User Frontend uses Vite/React with shadcn/ui components, supporting light/dark themes and Chinese/English localization.
- Access `/` in browser to open user dashboard (requires login).
- Features: Dashboard, Server list, Plan details, Traffic statistics, Knowledge base, Settings.
- Can be disabled via config `ui.user.enabled: false`.

### Docker

```bash
docker build -t xboard .
docker run --rm -it \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  --name xboard \
  xboard serve
```

### Linux Service Management (systemd/OpenRC)

Use the provided scripts for install/uninstall:

```bash
# Install panel (requires root)
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/panel.sh | sudo bash

# Install agent (requires root)
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# Install agent + sing-box core
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090' -c sing-box

# One-liner bootstrap entry (bootstrap logic is merged into agent.sh)
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo INSTALL_DIR=/opt/xboard/agent bash -s -- \
  --bootstrap --ref latest -- -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# Bootstrap with explicit tag (script/service/binary version bound to same tag)
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo INSTALL_DIR=/opt/xboard/agent bash -s -- \
  --bootstrap --ref v1.2.3 -- -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# Uninstall panel-managed artifacts
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/panel.sh | sudo bash -s -- --uninstall

# Uninstall agent-managed artifacts
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- --uninstall

# Alternative: download then run (works on all systems, no /dev/fd required)
wget -qO /tmp/panel.sh https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/panel.sh
sudo bash /tmp/panel.sh

# Agent install (curl pipe)
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# Agent install (wget pipe)
wget -qO- https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# Agent bootstrap with explicit INSTALL_DIR (wget download + run)
wget -qO /tmp/agent.sh https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh
sudo INSTALL_DIR=/opt/xboard/agent bash /tmp/agent.sh --bootstrap --ref latest -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# Agent uninstall (curl pipe)
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- --uninstall

# Agent uninstall (wget download + run)
wget -qO /tmp/agent.sh https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh
sudo bash /tmp/agent.sh --uninstall
```

Service manager behavior:
- Installer prefers systemd; if systemd is unavailable and OpenRC is available, it falls back to OpenRC (`/etc/init.d/*` + `rc-update add`).
- `XBOARD_INSTALL_SKIP_SYSTEMD=1` keeps legacy behavior: skip automatic service registration.
- With custom `INSTALL_DIR`, generated systemd units render `WorkingDirectory` and `ExecStart` to that path (not hardcoded `/opt/xboard`).
- `--uninstall` performs best-effort cleanup for both systemd and OpenRC (`systemctl` / `rc-service` + `rc-update`) and remains idempotent.

Service control examples:
```bash
# systemd host
sudo systemctl start xboard
sudo systemctl status xboard

# OpenRC host
sudo rc-service xboard start
sudo rc-service xboard status
```

Default installation directories are `/opt/xboard/panel` (panel) and `/opt/xboard/agent` (agent).

Download dependency preparation (`curl` + CA certificates) is handled directly in `deploy/panel.sh` and `deploy/agent.sh` before binary download.

Release binary integrity:
- `deploy/panel.sh` and `deploy/agent.sh` verify downloaded release binaries against `SHA256SUMS.txt` from the same release.
- Missing checksum entries, checksum mismatches, or checksum manifest download failures all cause hard failure.

Agent install environment variables:
- `XBOARD_BOOTSTRAP_REF`: bootstrap target ref (`latest`, release tag, or commit hash; commit hash requires `XBOARD_RELEASE_TAG` to be set explicitly for version consistency).
- `XBOARD_BOOTSTRAP_REPO`: bootstrap source repository (default `creamcroissant/xboard2p`).
- `XBOARD_AGENT_SCRIPT_URL` / `XBOARD_AGENT_SERVICE_URL`: optional override URLs for private mirror.
- `XBOARD_BOOTSTRAP_DOWNLOAD_STRICT`: deprecated compatibility flag; bootstrap is strict-only by default.

Agent install parameters (`deploy/agent.sh`):
- `-k, --communication-key` / `XBOARD_AGENT_COMMUNICATION_KEY`
- `-g, --grpc-address` / `XBOARD_AGENT_GRPC_ADDRESS`
  - Default split-port deployment: set Panel gRPC endpoint (for example `10.0.0.2:9090`).
  - If Panel enables `grpc.reuse_http_port=true`, set this to the Panel HTTP endpoint (for example `10.0.0.2:8080`).
- `-t, --grpc-tls-enabled` / `XBOARD_AGENT_GRPC_TLS_ENABLED` (default `false`)
- `--traffic-type` / `XBOARD_AGENT_TRAFFIC_TYPE` (default `netio`)
- `-f, --force-config-overwrite` / `XBOARD_AGENT_CONFIG_OVERWRITE=1`
- `-c, --with-core` / `XBOARD_AGENT_WITH_CORE` (default does not install any core)
- `--uninstall` (remove script-managed artifacts only)

Core-only maintenance parameters:
- `--core-action` / `XBOARD_AGENT_CORE_ACTION`
- `--core-type` / `XBOARD_AGENT_CORE_TYPE`
- `--core-version` / `XBOARD_AGENT_CORE_VERSION`
- `--core-channel` / `XBOARD_AGENT_CORE_CHANNEL`
- `--core-flavor` / `XBOARD_AGENT_CORE_FLAVOR`

Config generation behavior:
- If `config.yml` does not exist: installer writes it from parameters.
- If `config.yml` exists: installer keeps it unless overwrite is explicitly enabled.
- Missing `communication_key` or `grpc_address` causes hard failure with usage example.
- Fresh install config always starts with empty `panel.host_token` and non-empty `panel.communication_key`.
- `host_token` is not a public install input anymore; it is written back by the Agent after first-boot registration.
- Installer logs do not print secret values.

Uninstall behavior:
- `--uninstall` removes only artifacts managed by the scripts.
- It does not remove unknown files under `INSTALL_DIR`.
- It does not uninstall system dependencies (e.g., `curl`, `ca-certificates`).
- `agent.sh` treats `--bootstrap` and `--uninstall` as mutually exclusive; mixing `--uninstall` with install/bootstrap parameters fails.

Example (non-interactive):
```bash
sudo INSTALL_DIR=/opt/xboard/agent \
  XBOARD_AGENT_COMMUNICATION_KEY='your-agent-communication-key' \
  XBOARD_AGENT_GRPC_ADDRESS='10.0.0.2:9090' \
  sh ./deploy/agent.sh
```

Bootstrap is strict-only:
- Checksum manifest source is fixed to release `SHA256SUMS.txt` for the selected release tag.
- `agent.sh` and `agent.service` must both pass checksum verification before installer execution.
- `agent.service` download failure => bootstrap exits immediately.
- `agent.service` checksum mismatch => bootstrap exits immediately.

## 🔌 API Overview

Base URL: `/api`

### Health & observability
- `GET /healthz`
- `GET /health`
- `GET /_internal/ready`
- `GET /metrics` (optional token protection)

### MCP (Model Context Protocol) — LLM 运维接入
- `POST /mcp/message` — JSON-RPC 2.0 tool calls (Bearer auth)
- `GET /mcp/events` — SSE event stream
- 17+ tools: `system_status`, `agent_list`, `agent_logs_fetch`, `server_log_tail`, `operation_logs_list`, `user_list`, `plan_list`, `cdn_site_list`, `mesh_network`, `config_artifacts`, etc.
- API Key management via admin UI (`/{securePath}/mcp-keys`)

### Install endpoints
- `GET /api/install/status`
- `POST /api/install/`

### v2 endpoints (`/api/v2`)
- Admin: `/api/v2/{securePath}` with modules such as `config`, `plan`, `user`, `stat`, `system`, `notice`, `knowledge`, `agent-hosts`, `forwarding`, `access-logs`
- User: `/api/v2/user`
- Passport: `/api/v2/passport/auth`, `/api/v2/passport/comm`
- Server: `/api/v2/server/*` (for server/agent communication)
- Guest: `/api/v2/guest/i18n/{lang}`

### v1 endpoints (`/api/v1`)
- Client: `/api/v1/client`
- Guest: `/api/v1/guest` (plan/telegram/comm)
- Passport: `/api/v1/passport/auth`, `/api/v1/passport/comm`
- User: `/api/v1/user` and submodules (`invite`, `notice`, `server`, `telegram`, `comm`, `knowledge`, `plan`, `stat`, `shortlink`)
- Agent: `/api/v1/agent` (`register`, `status`, `heartbeat`)

### Short link
- `GET /s/{code}`

Route registration reference: `internal/api/router.go`.

## ⚙️ Configuration

Configuration is loaded from `config.yml` (preferred) or Environment Variables (for containerization).

See `config.example.yml` for structure and `coding.md` for details.

## 🧪 Development Workflow

| Action | Command |
| --- | --- |
| Install Deps | `go mod tidy` |
| Format Code | `gofmt -w ./cmd ./internal ./pkg ./test` |
| Unit Test | `go test ./...` |
| Run Service | `go run ./cmd/xboard serve` |
| Build All | `make build` |
| Build Frontend Only | `make build-frontend` |
| Build Backend Only | `make build-backend` |
| Smoke Test | `make smoke` |
| E2E (full) | `./scripts/e2e-test.sh` |
| E2E (deploy-cmd) | `E2E_MODE=deploy-cmd PLAYWRIGHT_ARGS='tests/02-admin-agents.spec.ts' ./scripts/e2e-test.sh` |

## 📊 Feature Status (2026-07)

- ✅ Admin: Config / Plan / User / Server / Stat / Notice / Knowledge / Forwarding / System Settings / **MCP API Keys**.
- ✅ Admin Frontend: Vite/React (shadcn/ui), embedded in binary.
- ✅ User: Subscription, Traffic Log, Node List, Announcement, Knowledge Base, Profile Settings.
- ✅ User Frontend: Dashboard, Servers, Plans, Traffic, Knowledge, Settings (Vite/React/shadcn/ui).
- ✅ Server: Heartbeat, telemetry, traffic reporting, core switching (Sing-box/Xray).
- ✅ Background Jobs: Traffic Aggregation, Node Sampling, Notification Queue, Traffic Reset.
- ✅ **MCP Server**: LLM operational access via standard MCP protocol (JSON-RPC 2.0 + SSE), 17+ read-only tools, DB-backed API key management.
- ✅ **Agent Log Upload**: Automatic local log tailing and upload via gRPC, in-memory FIFO cache (50 lines/agent default, configurable), accessible via MCP `agent_logs_fetch` tool.
- ✅ Security: Rate Limiting, Captcha, IP-based Restrictions, Input Validation.
- ⚠️ Some endpoints are still under iterative migration; behavior is defined by current implementation.

## 📄 License

[MIT](LICENSE)