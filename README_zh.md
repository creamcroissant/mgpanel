# XBoard

<div align="center">

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)
![SQLite](https://img.shields.io/badge/SQLite-Embedded-003B57.svg)
![License](https://img.shields.io/badge/License-MIT-yellow.svg)

</div>

XBoard 是一个基于 Go 的面板与 Agent 系统，用于订阅与流量管理。项目以单一二进制交付，内置管理端/用户端前端、SQLite 存储、后台任务与面板/探针部署脚本。

## ✨ 亮点

- **Go + Chi**：单一运行时覆盖面板 API、Agent 通信与后台任务。
- **内置 SQLite + 迁移**：开箱即用嵌入式数据库，启动自动执行 Goose 风格迁移。
- **后台作业内建**：流量聚合、遥测采样、通知队列在进程内调度执行。
- **前端内置交付**：管理端/用户端 SPA 构建产物默认随后端二进制发布。
- **脚本化部署**：`deploy/*.sh` 提供 panel/agent 的安装、bootstrap 与卸载流程。

## 📁 目录概览

```
cmd/              # xboard (统一 CLI 入口)
internal/         # API、Service、Repository、Job、Async、Bootstrap 等核心模块
pkg/, test/       # 预留扩展库与契约/集成测试
Dockerfile        # Go 多阶段构建
config.example.yml # YAML 配置示例
coding.md         # 官方架构文档
README.md         # 英文概览
README_zh.md      # 中文概览
todo.list         # 开发任务板
```

详细架构、约束与规划请参阅 `coding.md`。

## 🚀 快速开始

### 本地运行

```bash
# 1. 启用 Go 工具链（示例使用 gvm）
source ~/.gvm/scripts/gvm && gvm use go1.25.1

# 2. 准备配置
mkdir -p data
cp config.example.yml config.yml # 使用 YAML 配置

# 3. 启动服务
go run ./cmd/xboard serve
```

服务默认监听 `0.0.0.0:8080`，首次启动会在 `data/xboard.db` 自动执行 SQLite 迁移。

### CLI 命令

`xboard` 二进制文件提供以下子命令：

- `xboard serve`: 启动 HTTP 服务（默认）。
- `xboard user`: 用户管理（创建、列表、重置密码等）。
- `xboard config`: 查看或更新系统配置。
- `xboard migrate`: 数据库迁移管理。
- `xboard backup`: 备份数据库。
- `xboard restore`: 从备份恢复数据库。
- `xboard job`: 管理后台任务。
- `xboard version`: 查看版本信息。

### 初始化向导

- 当数据库中尚未存在管理员账号时，服务会自动跳转到 `/install`，展示与面板同风格的安装引导。
- 引导界面允许填写“用户名（可选）/ 邮箱（可选）+ 密码”，至少提供其一即可完成初始化。
- 也可使用 CLI (`go run ./cmd/xboard user create --email admin@example.com --password secret --admin`) 手动创建。

### 管理前端

- Admin 前端已迁移至 Vite/React，构建产物已嵌入二进制文件中。
- 浏览器访问 `/{secure_path}/login`（默认 `/admin/login`）即可进入登录页，支持"邮箱 / 用户名"登录。
- 可通过 `config.yml` 中的 `ui.admin.enabled: false` 关闭内置前端。

### 用户前端

- 用户前端使用 Vite/React + HeroUI 组件库，支持亮色/暗色主题和中英双语。
- 浏览器访问 `/` 进入用户面板（需登录）。
- 功能：仪表盘、节点列表、套餐详情、流量统计、知识库、个人设置。
- 可通过 `config.yml` 中的 `ui.user.enabled: false` 关闭内置前端。

### Docker

```bash
docker build -t xboard .
docker run --rm -it \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  --name xboard \
  xboard serve
```

镜像中只包含编译后的二进制；`/data` 用于持久化 SQLite 文件。

### Linux 服务管理（systemd/OpenRC）

使用脚本进行安装/卸载：

```bash
# 安装 panel（需要 root）
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/panel.sh | sudo bash

# 安装 agent（需要 root）
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# 安装 agent + sing-box core
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090' -c sing-box

# 单命令 bootstrap 入口（bootstrap 逻辑已并入 agent.sh）
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo INSTALL_DIR=/opt/xboard/agent bash -s -- \
  --bootstrap --ref latest -- -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# 指定 tag 的 bootstrap（脚本/service/二进制版本强绑定）
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo INSTALL_DIR=/opt/xboard/agent bash -s -- \
  --bootstrap --ref v1.2.3 -- -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# 卸载 panel 脚本管理产物
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/panel.sh | sudo bash -s -- --uninstall

# 卸载 agent 脚本管理产物
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- --uninstall

# 备用方案：先下载再执行（所有系统通用，不依赖 /dev/fd）
wget -qO /tmp/panel.sh https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/panel.sh
sudo bash /tmp/panel.sh

# agent 安装（curl 管道）
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# agent 安装（wget 管道）
wget -qO- https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# agent bootstrap（wget 下载后执行）
wget -qO /tmp/agent.sh https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh
sudo INSTALL_DIR=/opt/xboard/agent bash /tmp/agent.sh --bootstrap --ref latest -- \
  -k 'your-agent-communication-key' -g '10.0.0.2:9090'

# agent 卸载（curl 管道）
curl -fsSL https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh | sudo bash -s -- --uninstall

# agent 卸载（wget 下载后执行）
wget -qO /tmp/agent.sh https://raw.githubusercontent.com/creamcroissant/xboard2p/main/deploy/agent.sh
sudo bash /tmp/agent.sh --uninstall
```

服务管理行为：
- 安装时优先使用 systemd；若 systemd 不可用且 OpenRC 可用，则自动回退到 OpenRC（`/etc/init.d/*` + `rc-update add`）。
- `XBOARD_INSTALL_SKIP_SYSTEMD=1` 保持旧语义：跳过自动服务注册。
- 当使用自定义 `INSTALL_DIR` 时，落地的 systemd unit 会将 `WorkingDirectory` 与 `ExecStart` 渲染为该目录（不再固定 `/opt/xboard`）。
- `--uninstall` 会对 systemd 与 OpenRC 都执行 best-effort 清理（`systemctl` / `rc-service` + `rc-update`），且保持幂等。

服务控制示例：
```bash
# systemd 主机
sudo systemctl start xboard
sudo systemctl status xboard

# OpenRC 主机
sudo rc-service xboard start
sudo rc-service xboard status
```

默认安装目录为：panel 使用 `/opt/xboard/panel`，agent 使用 `/opt/xboard/agent`。

下载依赖准备（`curl` + CA 证书）由 `deploy/panel.sh` 与 `deploy/agent.sh` 在二进制下载前直接处理。

release 二进制完整性校验：
- `deploy/panel.sh` 与 `deploy/agent.sh` 会使用同一 release 的 `SHA256SUMS.txt` 校验下载二进制。
- checksum 条目缺失、checksum 不一致、或清单下载失败都会直接 hard-fail。

agent 安装相关环境变量：
- `XBOARD_BOOTSTRAP_REF`：bootstrap 目标版本（`latest`、release tag 或 commit hash；commit hash 场景需显式设置 `XBOARD_RELEASE_TAG` 以保持版本一致）。
- `XBOARD_BOOTSTRAP_REPO`：bootstrap 源仓库（默认 `creamcroissant/xboard2p`）。
- `XBOARD_AGENT_SCRIPT_URL` / `XBOARD_AGENT_SERVICE_URL`：私有镜像下载地址覆盖。
- `XBOARD_BOOTSTRAP_DOWNLOAD_STRICT`：兼容保留变量；bootstrap 默认 strict-only，不再影响行为。

`deploy/agent.sh` 安装参数（CLI/ENV 对应）：
- `-k, --communication-key` / `XBOARD_AGENT_COMMUNICATION_KEY`
- `-g, --grpc-address` / `XBOARD_AGENT_GRPC_ADDRESS`
  - 默认双端口部署：填写 Panel 的 gRPC 端点（例如 `10.0.0.2:9090`）。
  - 若 Panel 开启 `grpc.reuse_http_port=true`：填写 Panel 的 HTTP 端点（例如 `10.0.0.2:8080`）。
- `-t, --grpc-tls-enabled` / `XBOARD_AGENT_GRPC_TLS_ENABLED`（默认 `false`）
- `--traffic-type` / `XBOARD_AGENT_TRAFFIC_TYPE`（默认 `netio`）
- `-f, --force-config-overwrite` / `XBOARD_AGENT_CONFIG_OVERWRITE=1`
- `-c, --with-core` / `XBOARD_AGENT_WITH_CORE`（默认不安装任何 core）
- `--uninstall`（仅清理脚本管理产物）

仅 Core 运维参数：
- `--core-action` / `XBOARD_AGENT_CORE_ACTION`
- `--core-type` / `XBOARD_AGENT_CORE_TYPE`
- `--core-version` / `XBOARD_AGENT_CORE_VERSION`
- `--core-channel` / `XBOARD_AGENT_CORE_CHANNEL`
- `--core-flavor` / `XBOARD_AGENT_CORE_FLAVOR`

配置文件生成规则：
- `config.yml` 不存在：按参数写入。
- `config.yml` 已存在：默认不覆盖；显式开启 overwrite 才覆盖。
- `communication_key` 或 `grpc_address` 缺失：直接失败并输出示例。
- 新安装生成的配置固定为 `panel.host_token` 为空、`panel.communication_key` 非空。
- `host_token` 不再作为公开安装输入；它只会在 Agent 首启注册后自动回写。
- 安装日志不打印敏感值。

卸载行为说明：
- `--uninstall` 仅清理脚本管理项。
- 不会删除 `INSTALL_DIR` 下未知文件。
- 不会卸载系统依赖（如 `curl`、`ca-certificates`）。
- `agent.sh` 中 `--bootstrap` 与 `--uninstall` 互斥；`--uninstall` 与安装/bootstrap 参数混用会直接失败。

非交互示例：
```bash
sudo INSTALL_DIR=/opt/xboard/agent \
  XBOARD_AGENT_COMMUNICATION_KEY='your-agent-communication-key' \
  XBOARD_AGENT_GRPC_ADDRESS='10.0.0.2:9090' \
  sh ./deploy/agent.sh
```

bootstrap 现为 strict-only：
- checksum 清单来源固定为目标 release tag 对应的 `SHA256SUMS.txt`。
- `agent.sh` 与 `agent.service` 均需通过 checksum 校验后才会执行安装。
- `agent.service` 下载失败：立即退出。
- `agent.service` checksum 校验失败：立即退出。

## 🔌 API 接口概览

基础路径：`/api`

### 健康检查与观测
- `GET /healthz`
- `GET /health`
- `GET /_internal/ready`
- `GET /metrics`（可配置 token 保护）

### 安装初始化接口
- `GET /api/install/status`
- `POST /api/install/`

### v2 接口（`/api/v2`）
- 管理端：`/api/v2/{securePath}`，包含 `config`、`plan`、`user`、`stat`、`system`、`notice`、`knowledge`、`agent-hosts`、`forwarding`、`access-logs` 等模块
- 用户端：`/api/v2/user`
- 认证与通信：`/api/v2/passport/auth`、`/api/v2/passport/comm`
- 服务端通信：`/api/v2/server/*`
- 访客端：`/api/v2/guest/i18n/{lang}`

### v1 接口（`/api/v1`）
- 客户端：`/api/v1/client`
- 访客端：`/api/v1/guest`（plan/telegram/comm）
- 认证与通信：`/api/v1/passport/auth`、`/api/v1/passport/comm`
- 用户端：`/api/v1/user` 及其子模块（`invite`、`notice`、`server`、`telegram`、`comm`、`knowledge`、`plan`、`stat`、`shortlink`）
- Agent：`/api/v1/agent`（`register`、`status`、`heartbeat`）

### 短链跳转
- `GET /s/{code}`

路由注册参考：`internal/api/router.go`。

## ⚙️ 配置参数

配置优先读取 `config.yml`，同时支持环境变量覆盖（适合容器化部署）。

详见 `config.example.yml` 及 `coding.md`。

## 🧪 开发流程

| 动作 | 命令 |
| --- | --- |
| 安装依赖 | `go mod tidy` |
| 代码格式化 | `gofmt -w ./cmd ./internal ./pkg ./test` |
| 单元测试 | `go test ./...` |
| 启动服务 | `go run ./cmd/xboard serve` |
| 完整构建 | `make build` |
| 仅构建前端 | `make build-frontend` |
| 仅构建后端 | `make build-backend` |
| 冒烟测试 | `make smoke` |
| E2E（完整） | `./scripts/e2e-test.sh` |
| E2E（deploy-cmd） | `E2E_MODE=deploy-cmd PLAYWRIGHT_ARGS='tests/02-admin-agents.spec.ts' ./scripts/e2e-test.sh` |

## 📊 功能状态（2025-12）

- ✅ Admin：Config / Plan / User / Server / Stat / Notice / Knowledge。
- ✅ Admin 前端：Vite/React，已嵌入二进制。
- ✅ User：订阅、流量日志、节点列表、公告、知识库。
- ✅ 用户前端：仪表盘、节点、套餐、流量、知识库、设置（Vite/React/HeroUI）。
- ✅ Server：心跳、遥测、流量上报。
- ✅ Background Jobs：流量汇总、节点采样、通知队列。
- ⚠️ 部分接口仍在持续迭代，行为以当前实现为准。

## 📄 许可证

[MIT](LICENSE)