# Banana Lab (Local)

本仓库包含 Banana Lab 的本地开发版本：

- Go 后端（Gin + SQLite）
- React 前端（Vite + TypeScript + Tailwind）
- Playwright E2E 测试（前后端联调）

---

## 目录结构

```text
.
├── cmd/                 # Go 程序入口
├── internal/            # Go 业务实现（api/service/model/...）
├── web/                 # React + Vite 前端
│   └── tests/e2e/       # Playwright E2E
├── config.yaml          # 本地配置文件（可选，支持被环境变量覆盖）
└── README.md
```

---

## 环境要求

- Go `1.25+`
- Node.js `20+`（推荐 LTS）
- npm `10+`

---

## 快速开始

### 1) 启动后端（仓库根目录）

后端启动前需要提供以下关键配置（可写在 `config.yaml` 或环境变量中）：

- `PROXY_BASE_URL`
- `PROXY_AUTH_HEADER_VALUE`
- `MODEL_ID`

本仓库默认提供了 `config.yaml`，可直接：

```bash
go run ./cmd/server
```

健康检查：

```bash
curl http://127.0.0.1:8000/api/health
```

> 若你的 `config.yaml` / 环境变量里 `SERVER_PORT` 不是 `8000`，请把上面端口改为实际值。

### 2) 启动前端（`web/` 目录）

```bash
npm install
npm run dev
```

默认情况下，前端会将 `/api` 和 `/assets` 代理到：

- `VITE_BACKEND_ORIGIN`（若设置）
- 否则默认 `http://127.0.0.1:8000`

如果后端端口不同，启动前端时指定：

```bash
VITE_BACKEND_ORIGIN=http://127.0.0.1:8080 npm run dev
```

---

## 常用命令

### 后端（根目录）

```bash
# 启动
go run ./cmd/server

# 全量测试
go test ./...

# 单包测试
go test ./internal/api

# 单用例测试
go test ./internal/api -run TestSessionsAPI
```

### 前端（`web/`）

```bash
# 开发
npm run dev

# 构建（tsc + vite）
npm run build

# Lint
npm run lint

# 单元测试（Vitest）
npm run test

# E2E（Playwright）
npm run test:e2e
```

也可在仓库根目录直接运行：

```bash
npx playwright test
```

---

## E2E 测试说明

`web/playwright.config.ts` 会在测试时自动拉起：

1. 后端（`SERVER_PORT=18080`，并注入 mock 代理相关环境变量）
2. 前端（`http://127.0.0.1:5173`，`VITE_BACKEND_ORIGIN=http://127.0.0.1:18080`）

因此通常只需执行：

```bash
cd web
npm run test:e2e
```

---

## 配置说明（后端）

配置加载优先级：**环境变量 > `config.yaml` > 代码默认值**。

关键字段如下：

| 配置项 | 是否必需 | 默认值 | 说明 |
|---|---|---|---|
| `PROXY_BASE_URL` | 是 | 无 | 上游推理服务地址 |
| `PROXY_AUTH_HEADER_VALUE` | 是 | 无 | 上游鉴权值 |
| `MODEL_ID` | 是 | 无 | 生成模型 ID |
| `SERVER_PORT` | 否 | `8080` | 服务监听端口 |
| `STORAGE_ROOT` | 否 | `./storage` | 存储目录 |
| `DB_PATH` | 否 | `<storage_root>/banana-lab.db` | SQLite 文件路径 |
| `PROXY_AUTH_HEADER_NAME` | 否 | `Authorization` | 上游鉴权 Header 名 |
| `MAX_CONCURRENT` | 否 | `1` | 生成并发上限 |
| `LIBRARY_PAGE_SIZE` | 否 | `20` | 资源库分页大小 |
| `GENERATE_TIMEOUT` | 否 | `120s` | 生成超时 |

> 注意：请勿在公开仓库提交真实凭据。生产环境建议仅通过安全的环境变量注入敏感配置。

---

## API 概览

常用接口（前缀均为 `/api`）：

- `GET /health`
- Sessions：`GET/POST/PATCH/DELETE /sessions...`
- `GET /sessions/:id/tree`
- `GET /nodes/:id`
- `GET /library`
- `POST /upload`
- `POST /generate`

静态资源：`/assets/images`、`/assets/uploads`

---

## 分辨率调节（`image_size`）

- 前端生成请求会在 `POST /api/generate` 中携带 `image_size`，可选值：`1K`、`2K`、`4K`。
- 后端会将该值持久化到节点（`prompt_nodes.image_size`），用于历史节点恢复。
- 后端会将该值透传给上游代理请求：`image_config.image_size`。
- `width` / `height` 字段仍保留并继续发送，兼容现有调用语义。

---

## 开发约定

- 保持最小改动，优先遵循现有代码风格。
- Go 代码使用 `gofmt`。
- 前端使用 ESLint（`web/eslint.config.js`）与 TS strict（`web/tsconfig.app.json`）。
- 详细协作规范见 `AGENTS.md`。
