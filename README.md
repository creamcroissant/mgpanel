# XBoard

<div align="center">

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)
![SQLite](https://img.shields.io/badge/SQLite-Embedded-003B57.svg)
![License](https://img.shields.io/badge/License-MIT-yellow.svg)

</div>

XBoard 是基于 Go 的面板 + Agent 管理系统，集成了订阅管理、流量统计、节点通信和自动化运维能力。单个二进制内嵌了管理端/用户端前端、SQLite 数据库、后台任务调度器以及面板/Agent 部署脚本。

## ✨ 特性

- **Go + Chi**: 统一运行时后端，聚合面板 API、Agent 通信和后台任务。
- **SQLite + 内嵌迁移**: 开箱即用的嵌入式数据库，启动时自动执行迁移。
- **内置调度器**: 流量聚合、节点采样、通知推送等任务在进程内运行。
- **内嵌前端**: 管理端/用户端 SPA 资源默认打包进二进制文件。
- **MCP Server (LLM 运维)**: 内建 JSON-RPC 2.0 + SSE 端点，LLM 可通过标准 MCP 协议查询状态、读取日志、管理密钥。
- **Agent 日志上传**: Agent 自动尾随本地日志，通过现有 gRPC 连接上传（内存 FIFO 缓存，不增加数据库压力）。
- **脚本化部署**: `deploy/*.sh` 覆盖面板/Agent 的安装与卸载流程。

## 📁 目录结构

```
cmd/
├── xboard/           # 面板主程序（serve, tui, user, config 等）
└── agent/            # Agent 程序
internal/             # API、Service、Repository、后台任务、异步、Bootstrap……
pkg/, test/           # 共享库与契约/集成测试
web/user-vite/        # 统一前端（Vite + React + shadcn/ui）
scripts/              # 构建与测试脚本
Dockerfile            # Go 多阶段构建
config.example.yml    # YAML 配置示例
```

更多细节、阶段目标和架构约束参见 `coding.md`。

## 🚀 快速开始

### 本地运行

```bash
# 1. 准备 Go 工具链
source ~/.gvm/scripts/gvm && gvm use go1.25.1   # 或任意 Go 1.25+

# 2. 初始化配置
mkdir -p data
cp config.example.yml config.yml

# 3. 启动服务
go run ./cmd/xboard serve
```

默认监听 `0.0.0.0:8080`，首次启动会自动在 `data/xboard.db` 执行 SQLite 迁移。

### CLI 命令

`xboard` 二进制提供多个子命令：

- `xboard serve`：启动 HTTP 服务（默认）
- `xboard user`：用户管理（创建、列出、重置密码等）
- `xboard config`：查看或更新系统配置
- `xboard migrate`：数据库迁移管理
- `xboard backup`：备份数据库
- `xboard restore`：从备份恢复数据库
- `xboard job`：管理后台任务
- `xboard version`：显示版本信息

### 初始化向导

- 如果数据库中不存在管理员账号，HTTP 服务会自动跳转到 `/install` 显示初始化界面。
- 向导支持通过"用户名（可选）/ 邮箱（可选）+ 密码"创建首个管理员账号。
- 也可使用 CLI：`go run ./cmd/xboard user create --email admin@example.com --password secret --admin`。

### 管理端前端

- 管理端前端使用 Vite/React，构建产物内嵌在二进制中。
- 浏览器访问 `/{secure_path}/login`（默认 `/admin/login`）打开登录页。
- 可通过配置 `ui.admin.enabled: false` 禁用，用于自定义 CDN 部署。

### 用户端前端

- 用户端前端使用 Vite/React + shadcn/ui，支持亮/暗主题和中/英文国际化。
- 浏览器访问 `/` 打开用户面板（需登录）。
- 功能：仪表盘、服务器列表、套餐详情、流量统计、知识库、设置。
- 可通过配置 `ui.user.enabled: false` 禁用。

### Docker

```bash
docker build -t xboard .
docker run --rm -it \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  --name xboard \
  xboard serve
```

### Linux 服务管理（systemd/OpenRC）

使用部署脚本进行安装/卸载：

```bash
# 安装面板（需要 root）
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/panel.sh | sudo bash

# 安装 Agent（需要 root）
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# 卸载面板
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/panel.sh | sudo bash -s -- --uninstall

# 卸载 Agent
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- --uninstall

# 或用 wget 下载后执行
wget -qO /tmp/agent.sh https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh
sudo bash /tmp/agent.sh -k 'your-agent-communication-key' -g '10.0.0.2:9090'
```

服务管理器说明：
- 优先使用 systemd；如果 systemd 不可用且 OpenRC 可用，则回退 OpenRC（`/etc/init.d/*` + `rc-update add`）。
- 设置 `XBOARD_INSTALL_SKIP_SYSTEMD=1` 跳过自动服务注册。
- 自定义 `INSTALL_DIR` 后，生成的 systemd unit 会将 `WorkingDirectory` 和 `ExecStart` 渲染到对应路径。
- `--uninstall` 对 systemd 和 OpenRC 都做尽力清理（`systemctl` / `rc-service` + `rc-update`），幂等执行。

服务控制：

```bash
# systemd 主机
sudo systemctl start xboard
sudo systemctl status xboard

# OpenRC 主机
sudo rc-service xboard start
sudo rc-service xboard status
```

默认安装目录为 `/opt/xboard/panel`（面板）和 `/opt/xboard/agent`（Agent）。

下载依赖（`curl` + CA 证书）由 `deploy/panel.sh` 和 `deploy/agent.sh` 在下载二进制前自动处理。

发布二进制完整性：
- `deploy/panel.sh` 和 `deploy/agent.sh` 会从同一次发布中下载 `SHA256SUMS.txt` 并校验二进制。
- 缺少校验和条目、校验和不匹配或校验和文件下载失败都会导致安装中止。

Agent 安装参数：

| 参数 | 环境变量 | 说明 |
|------|---------|------|
| `-k` | `XBOARD_AGENT_COMMUNICATION_KEY` | Agent 注册通信密钥（必填） |
| `-g` | `XBOARD_AGENT_GRPC_ADDRESS` | Panel gRPC 地址，例如 `10.0.0.2:9090`（必填） |
| `-t` | `XBOARD_AGENT_GRPC_TLS_ENABLED` | gRPC TLS 开关，默认 `false` |
| `--traffic-type` | `XBOARD_AGENT_TRAFFIC_TYPE` | 流量统计方式，默认 `netio` |
| `-f` | `XBOARD_AGENT_CONFIG_OVERWRITE=1` | 强制覆盖已有 config.yml |
| `--uninstall` | — | 卸载 Agent |

配置生成行为：
- 如果 `config.yml` 不存在：安装器根据参数写入。
- 如果 `config.yml` 已存在：安装器保留现有文件（除非显式启用覆盖）。
- 缺少 `communication_key` 或 `grpc_address` 会导致安装中止。
- 初始配置中 `panel.host_token` 为空，`panel.communication_key` 由安装参数写入。
- `host_token` 不再是公开安装参数，由 Agent 首次注册后回写。
- 安装日志不会打印密钥值。

卸载行为：
- `--uninstall` 仅删除脚本管理的文件，不会移除配置目录下的未知文件。
- 不会卸载系统依赖（如 `curl`、`ca-certificates`）。

非交互式安装示例：

```bash
sudo INSTALL_DIR=/opt/xboard/agent \
  XBOARD_AGENT_COMMUNICATION_KEY='your-agent-communication-key' \
  XBOARD_AGENT_GRPC_ADDRESS='10.0.0.2:9090' \
  sh ./deploy/agent.sh
```

## 🔌 API 概览

基础路径：`/api`

### 健康与可观测性
- `GET /healthz`
- `GET /health`
- `GET /_internal/ready`
- `GET /metrics`（可选 token 鉴权）

### MCP（Model Context Protocol）— LLM 运维接入
- `POST /mcp/message` — JSON-RPC 2.0 工具调用（Bearer 鉴权）
- `GET /mcp/events` — SSE 事件流
- 17+ 工具：`system_status`、`agent_list`、`agent_logs_fetch`、`server_log_tail`、`operation_logs_list`、`user_list`、`plan_list`、`cdn_site_list`、`mesh_network`、`config_artifacts` 等
- API Key 管理：管理后台 `/{securePath}/mcp-keys`

### Install 端点
- `GET /api/install/status`
- `POST /api/install/`

### v2 端点（`/api/v2`）
- 管理端：`/api/v2/{securePath}`，模块包括 `config`、`plan`、`user`、`stat`、`system`、`notice`、`knowledge`、`agent-hosts`、`forwarding`、`access-logs`
- 用户端：`/api/v2/user`
- 认证：`/api/v2/passport/auth`、`/api/v2/passport/comm`
- 服务端：`/api/v2/server/*`（用于 server/agent 通信）
- 访客：`/api/v2/guest/i18n/{lang}`

### v1 端点（`/api/v1`）
- 客户端：`/api/v1/client`
- 访客：`/api/v1/guest`（plan/telegram/comm）
- 认证：`/api/v1/passport/auth`、`/api/v1/passport/comm`
- 用户端：`/api/v1/user` 及子模块（`invite`、`notice`、`server`、`telegram`、`comm`、`knowledge`、`plan`、`stat`、`shortlink`）
- Agent：`/api/v1/agent`（`register`、`status`、`heartbeat`）

### 短链接
- `GET /s/{code}`

路由注册参考 `internal/api/router.go`。

## ⚙️ 配置

配置加载顺序：`config.yml`（推荐）> 环境变量（容器化场景）。

结构参见 `config.example.yml`，详细说明参见 `coding.md`。

## 🧪 开发工作流

| 操作 | 命令 |
| --- | --- |
| 安装依赖 | `go mod tidy` |
| 格式化代码 | `gofmt -w ./cmd ./internal ./pkg ./test` |
| 单元测试 | `go test ./...` |
| 启动服务 | `go run ./cmd/xboard serve` |
| 构建全部 | `make build` |
| 仅构建前端 | `make build-frontend` |
| 仅构建后端 | `make build-backend` |
| 冒烟测试 | `make smoke` |
| E2E 测试 | `./scripts/e2e-test.sh` |

## 📊 功能状态（2026-07）

- ✅ 管理端：配置 / 套餐 / 用户 / 服务器 / 统计 / 公告 / 知识库 / 转发 / 系统设置 / **MCP API Key 管理**。
- ✅ 管理端前端：Vite/React（shadcn/ui），内嵌于二进制。
- ✅ 用户端：订阅、流量记录、节点列表、公告、知识库、个人设置。
- ✅ 用户端前端：仪表盘、服务器、套餐、流量、知识库、设置（Vite/React/shadcn/ui）。
- ✅ 服务端：心跳、遥测、流量上报、核心切换（Sing-box/Xray）。
- ✅ 后台任务：流量聚合、节点采样、通知队列、流量重置。
- ✅ **MCP Server**：LLM 通过标准 MCP 协议进行运维操作（JSON-RPC 2.0 + SSE），17+ 只读工具，DB 持久化 API Key 管理。
- ✅ **Agent 日志上传**：Agent 自动尾随日志并通过 gRPC 上传，内存 FIFO 缓存（默认每 Agent 50 行，可配置），通过 MCP `agent_logs_fetch` 工具访问。
- ✅ 安全：频率限制、验证码、IP 限制、输入校验。
- ⚠️ 部分端点仍在迭代迁移中，行为以当前实现为准。

## 📄 许可证

[MIT](LICENSE)
