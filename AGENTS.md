# AGENTS.md — Banana Lab Local

## 构建 / 运行 / 测试命令

### Go 后端（仓库根目录执行）
```bash
go run ./cmd/server                          # 需要 PROXY_BASE_URL、PROXY_AUTH_HEADER_VALUE 与 config.yaml 的 models: 数组
go test ./...                                # 全量
go test ./internal/api                       # 单包
go test ./internal/api -run TestArchiveListInvalidRange  # 单用例
```
- 格式化用 `gofmt`；无 `golangci-lint` 配置。
- `webui_embed.go` 以 `//go:embed web/dist/*` 内嵌前端，`web/dist` 不存在时 Go 编译失败，需先执行 `npm --prefix web run build`。

### 前端 Web（在 `web/` 目录执行）
```bash
npm run dev                                  # Vite，/api 与 /assets 代理到 VITE_BACKEND_ORIGIN（默认 http://127.0.0.1:8000）
npm run build                                # tsc -b && vite build
npm run lint                                 # eslint .（flat config，无 Prettier）
npm run test                                 # Vitest（jsdom），src/**/*.test.{ts,tsx}
npx vitest run src/path/to/foo.test.tsx      # 单文件
npx vitest run -t "test name"                # 单用例
npm run test:e2e                             # Playwright
npx playwright test tests/e2e/smoke.spec.ts  # 单个 E2E 文件
```
- 后端代码默认端口为 `8080`，而 Vite 代理默认指向 `8000`；`config.yaml` 未入库，本地联调需让两者一致（设 `server_port`/`SERVER_PORT` 或 `VITE_BACKEND_ORIGIN`）。
- 根目录 `playwright.config.ts` 复用 `web/playwright.config.ts`，也可在根目录执行 `npx playwright test`。

### E2E 启动细节（`web/playwright.config.ts`）
- 先构建前端，再启动后端（`SERVER_PORT=18080`、`BANANA_FORCE_RESET_DB=1`、`STORAGE_ROOT=./storage-e2e`、mock 上游）、mock-llm（`18081`）与 Vite（`5173`，`VITE_BACKEND_ORIGIN=http://127.0.0.1:18080`）；启动前会 `kill -9` 占用这些端口的进程，本地复用已运行的服务。
- 单线程执行（`workers: 1`、`fullyParallel: false`），避免并发冲突。

### 测试专用启动脚本（LLM 使用）
```bash
bash scripts/start-test-stack.sh   # 后端 :38080、前端 :35173、mock-llm :38081，先释放占用端口
BACKEND_PORT=39080 FRONTEND_PORT=39173 MOCK_PORT=39081 bash scripts/start-test-stack.sh
```
- 日志写入 `.sisyphus/evidence/`。
- 需要拉起测试用前后端时优先使用该脚本，避免占用默认端口。

### 生图测试 Mock 强制策略
所有涉及上游生图调用（`ProxyClient.Generate`）的自动化测试（CI、E2E、Go 测试）**必须**将 `PROXY_BASE_URL` 指向本地 `cmd/mock-llm`，**严禁**调用真实上游 API。Playwright 已自动配置为 mock（`18081`），不得覆盖为真实地址；纯单元测试不受此限制。手动运行：
```bash
go run ./cmd/mock-llm -port 18081
PROXY_BASE_URL=http://127.0.0.1:18081 PROXY_AUTH_HEADER_VALUE=mock go test ./...
```

## 架构要点
- 单个 Go 二进制（`cmd/server`）同时提供 `/api/*` JSON API、`/assets/images` 与 `/assets/uploads` 静态目录，以及内嵌的 Vite 构建产物（SPA fallback）。
- 配置优先级：环境变量 > `config.yaml` > 代码默认值。模型注册表只来自 `config.yaml` 的 `models:` 数组，首个条目为默认模型；`MODEL_ID` 环境变量已弃用、不再读取。
- 已禁用的模型仍可 `Lookup` 到，以便历史节点的 `model_id` 正常解析；生成请求的（模型, 尺寸, 比例）由 `Registry.IsValid` 校验，区分 `invalid` 与 `model_disabled`。
- API 路由全部在 `cmd/server/main.go` 注册，`internal/api/router.go` 只注册 `/api/health`。
- SQLite：`model.Open` 强制 WAL 与 `max_open_conns=1`；`EnsureLegacySchemaGuard` 遇到旧 `sessions`/`nodes` 表结构会拒绝启动，除非设置 `BANANA_FORCE_RESET_DB=1`。`PromptNode.model_id` 为 NOT NULL 列，由原生 SQL ALTER 加列，`Migrate` 将空值幂等回填为 `gemini-3-pro-image`。
- 生成并发由 `ConcurrencyLimiter` 控制（`MAX_CONCURRENT`，默认 1）。
- 数据模型：`PromptNode` 为核心实体，承载 prompt 与参数、树的父指针及参考图 ID；`image_size` 既持久化在节点上，又作为 `image_config.image_size` 透传给上游，可选值由各模型的 `allowed_sizes` 决定。`Result` 关联 `PromptNode` 与 `Asset`，`status` 为 `active`/`archived`/`deleted`，`source_asset_id`/`source_domain` 记录 img2img / reroll 来源。`Asset.domain` 为 `txt2img`/`img2img`，`Asset.origin` 为 `generated`/`uploaded`。

## 代码风格与约定

### Go 后端
- 包名小写简短；导入顺序：标准库 → 第三方 → `banana-lab/...`。
- 错误处理用 `if err != nil` 早返回，不吞错误。
- JSON 字段通过 `json:"snake_case"` tag 命名；可空字段用指针类型。
- API 错误体为简洁错误码，如 `{ "error": "invalid" | "internal" | "not_found" }`。

### TypeScript / React（web）
- `tsconfig.app.json` 开启 `strict` 与 `noUnusedLocals`/`noUnusedParameters`。
- 组件文件名用 `PascalCase.tsx`；单引号、带分号。
- API 类型字段与后端一致使用 `snake_case`；`src/lib/api.ts` 的 `apiFetch` 统一解析后端 `error`/`message` 字段并抛出异常。

## 问题排查原则
遇到 Bug 必须先自行排查闭环，不得未经复现就询问用户：
1. 读代码定位故障点及影响范围。
2. 通过浏览器操作或测试用例复现，保留日志、截图等证据。
3. 修复后重新执行复现步骤或测试，确认已解决且无回归；配置/启动流程类改动须本地启动验证，不能只凭推理宣称修复。
4. 自检完成后仍有不确定性，再向用户汇报结论。
