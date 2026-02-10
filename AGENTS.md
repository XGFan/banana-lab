# AGENTS.md — Banana Lab Local

本文件用于指导代理在本仓库内工作，确保命令、代码风格与工程约定一致。

## 仓库结构
- Go 后端：`cmd/`、`internal/`
- 前端 Web：`web/`（React + Vite + TS + Tailwind）
- E2E：`web/tests/e2e`（Playwright）

## 构建 / 运行 / 测试命令

### Go 后端（仓库根目录执行）
- 启动开发服务：
  - `go run ./cmd/server`
- 全量测试：
  - `go test ./...`
- 单包测试：
  - `go test ./internal/api`
- 单用例测试：
  - `go test ./internal/api -run TestSessionsAPI`
- 单文件测试（可用）：
  - `go test ./internal/api -run TestSessionsAPI`（建议使用 -run）

> 说明：Go 默认使用 gofmt 格式化；本仓库未发现 golangci-lint 配置。

### 前端 Web（在 `web/` 目录执行）
- 启动开发服务：
  - `npm run dev`
- 构建：
  - `npm run build`（包含 `tsc -b` + `vite build`）
- Lint：
  - `npm run lint`
- 单元测试（Vitest）：
  - `npm run test`
- 单测：单文件 / 单用例（Vitest CLI）：
  - `npx vitest run src/path/to/foo.test.tsx`
  - `npx vitest run -t "test name"`

### E2E（Playwright）
- 在 `web/`：
  - `npm run test:e2e`（= `playwright test`）
- 指定单个 E2E 文件：
  - `npx playwright test tests/e2e/smoke.spec.ts`

### 测试专用启动脚本（LLM 使用）
- 脚本：`scripts/start-test-stack.sh`
- 用途：启动测试专用的**独立高位端口**服务（会先强制释放占用端口）
  - 后端：`38080`
  - 前端：`35173`
  - mock-llm：`38081`
- 启动命令（仓库根目录）：
  - `bash scripts/start-test-stack.sh`
- 可覆盖端口：
  - `BACKEND_PORT=39080 FRONTEND_PORT=39173 MOCK_PORT=39081 bash scripts/start-test-stack.sh`
- 规则：后续涉及测试环境拉起后端/前端时，优先使用该脚本，避免占用默认端口。

#### E2E 启动细节（来自 `web/playwright.config.ts`）
- E2E 会自动启动：
  - 后端：`SERVER_PORT=18080`，并使用 mock 代理配置
  - 前端：`VITE_BACKEND_ORIGIN=http://127.0.0.1:18080`，Vite 端口 `5173`

### 根目录 Playwright
- 根目录 `playwright.config.ts` 复用 `web/playwright.config.ts`。
- 可在根目录执行：`npx playwright test`。

## 代码风格与约定

### 共通原则
- 保持变更最小化；不要引入未使用的依赖。
- 错误处理使用早返回（guard clauses），避免深层嵌套。
- 先读现有文件，遵循已存在的风格与结构。

### 问题排查原则
- 遇到 Bug 或可疑问题时，**必须**先自行排查并闭环，严禁在未尝试复现的情况下直接询问用户。
- 排查流程**必须**包含以下步骤：
  1. **读代码定位**：深入源码分析逻辑，确定故障点及受影响范围。
  2. **复现并取证**：通过浏览器手动操作或编写测试用例复现问题，并收集日志、截图等证据。
  3. **修复与自证**：修复后**必须**重新执行复现步骤或运行测试，确保问题解决且无回归。
  4. **向用户确认**：仅在上述自检完成且问题依然存在不确定性时，再向用户汇报排查结论。
- 向用户确认前**必须**完成以上步骤并保留取证材料（截图/日志）。

### Go 后端
- 包命名：小写、简短（如 `api`, `service`, `model`）。
- 导出标识符：`PascalCase`；未导出使用 `camelCase`。
- 错误处理：`if err != nil { ... }` 早返回；不要忽略错误。
- 导入顺序：标准库 → 第三方库 → 本地模块（`banana-lab/...`）。
- JSON 命名：通过 `json:"snake_case"` tag（见 `internal/model/models.go`）。
- 可选字段：使用指针类型（`*string`, `*int` 等）表示可空。
- API 返回：在错误情况下使用 `{ "error": "invalid" | "internal" }` 等简洁错误串。

### TypeScript / React（web）
- ESLint：`web/eslint.config.js` 使用 flat config + recommended 规则。
- TypeScript：`web/tsconfig.app.json` 开启 `strict`，禁止未使用变量/参数。
- 组件命名：React 组件使用 `PascalCase` 文件名（如 `Txt2ImgPage.tsx`）。
- API 类型命名：与后端对齐，接口字段使用 `snake_case`（见 `web/src/lib/api.ts`）。
- 字符串风格：项目中通常使用单引号 `'`。
- 语句结尾：项目中通常使用分号 `;`。
- 错误处理：`apiFetch` 统一解析后端 `error`/`message` 字段并抛出异常。

## Lint / 格式化

### Go
- 格式化：`gofmt`（手动或编辑器保存时触发）。
- 未发现 `golangci-lint` 配置。

### Web
- Lint：`eslint .`（脚本为 `npm run lint`）。
- 未发现 Prettier 配置文件（`.prettierrc*`）。

## 测试策略建议
- Go：优先使用 `go test ./...`；调试时使用 `-run` 过滤测试。
- Web：优先使用 `npm run test`；E2E 使用 `npm run test:e2e`。
- E2E：默认单线程（`workers: 1`），避免并发冲突。

### 生图测试 Mock 强制策略 (Image Generation Test Mock Policy)
所有涉及上游生图调用（`ProxyClient.Generate`）的自动化测试（包括 CI、E2E 和 Go 测试）**必须**使用本地 mock，**严禁**调用真实上游 API。

- **强制要求**：测试环境下必须设置 `PROXY_BASE_URL=http://127.0.0.1:18081`。
- **Mock 实现**：使用 `cmd/mock-llm` 提供的模拟服务。
- **E2E 自动处理**：Playwright E2E 测试已在 `web/playwright.config.ts` 中配置为自动启动 `mock-llm`（端口 18081），**不得**将其覆盖为真实地址（**MUST NOT** call real upstream APIs in tests）。
- **脚本约定**：LLM 本地拉起测试环境时，使用 `scripts/start-test-stack.sh`，由脚本统一设置测试专用端口与 `PROXY_BASE_URL`。
- **适用范围**：本策略仅适用于涉及生图上游调用的测试，纯单元测试不受影响。

**本地运行方式**：
如果需要在本地手动运行涉及生图的测试，请先启动 mock 服务：
```bash
# 启动 Mock 服务
go run ./cmd/mock-llm -port 18081

# 运行测试时指定环境变量
PROXY_BASE_URL=http://127.0.0.1:18081 go test ./...
```

## AI 规则文件
- Cursor 规则：未发现 `.cursorrules` 或 `.cursor/rules/*`。
- Copilot 规则：未发现 `.github/copilot-instructions.md`。

如需新增 AI 指令，请在根目录添加对应规则文件，并在此文档更新。

## 文件清单（关键配置）
- `README.md`：后端启动与测试命令。
- `web/package.json`：前端脚本与依赖。
- `web/eslint.config.js`：前端 lint 规则。
- `web/vite.config.ts`：Vite 与 Vitest 配置（含 dev proxy）。
- `web/playwright.config.ts`：E2E 配置与 webServer 启动流程。
- `playwright.config.ts`：根目录转发配置。
- `web/tsconfig.app.json`：TS 严格模式与 lint 相关选项。

## 变更提示
- 新增命令或工具时，请同步更新此文件。
- 当发现 Cursor / Copilot 规则文件时，请将其摘要写入本节。
