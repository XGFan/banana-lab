# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Go backend (run from repo root)

```bash
go run ./cmd/server           # start backend (needs PROXY_BASE_URL, PROXY_AUTH_HEADER_VALUE, models: array in config.yaml)
go test ./...                 # all tests
go test ./internal/api        # one package
go test ./internal/api -run TestSessionsAPI   # one case
```

Go code uses `gofmt`. No `golangci-lint` config.

### Frontend (run from `web/`)

```bash
npm run dev            # Vite dev server, proxies /api and /assets to VITE_BACKEND_ORIGIN or http://127.0.0.1:8000
npm run build          # tsc -b && vite build
npm run lint           # eslint .
npm run test           # Vitest (jsdom) — src/**/*.test.{ts,tsx}
npx vitest run src/path/foo.test.tsx       # single file
npx vitest run -t "test name"              # single case
npm run test:e2e       # Playwright (auto-starts backend:18080, mock-llm:18081, web:5173)
```

### Full test stack (for LLM-driven E2E work)

```bash
bash scripts/start-test-stack.sh   # backend :38080, frontend :35173, mock-llm :38081 (kills occupants first)
```

Override ports with `BACKEND_PORT=...`, `FRONTEND_PORT=...`, `MOCK_PORT=...`. Logs land in `.sisyphus/evidence/`.

### Image-generation mock policy (strict)

Any test that exercises `ProxyClient.Generate` **must** point `PROXY_BASE_URL` at the local `cmd/mock-llm` — never the real upstream. Playwright sets this automatically; manual runs do it via `scripts/start-test-stack.sh` or:

```bash
go run ./cmd/mock-llm -port 18081
PROXY_BASE_URL=http://127.0.0.1:18081 PROXY_AUTH_HEADER_VALUE=mock go test ./...
```

Note: The `MODEL_ID` environment variable is deprecated; model selection is now driven by the `models:` array in `config.yaml`. Test environments rely on this config (see `web/playwright.config.ts` for E2E setup).

## Architecture

Single Go binary (`cmd/server`) that serves the JSON API on `/api/*`, static asset dirs under `/assets/*`, and the embedded Vite build (`web/dist` via `webui_embed.go`) as an SPA fallback.

### Layered layout (`internal/`)

- `config/` — env-var + `config.yaml` loader. Precedence: env > `config.yaml` > code defaults. `PROXY_BASE_URL`, `PROXY_AUTH_HEADER_VALUE` are required; model selection is driven by the `models:` array in `config.yaml` (see README "支持的模型" for details).
- `model/` — GORM models and SQLite setup. `Open` forces WAL and `max_open_conns=1`. `EnsureLegacySchemaGuard` refuses to boot against old `sessions`/`nodes` schema unless `BANANA_FORCE_RESET_DB=1`. `Migrate` auto-migrates `PromptNode`, `PromptSnippet`, `Asset`, `Result`, `Image`.
- `storage/` — disk layout rooted at `STORAGE_ROOT` (default `./storage`, DB at `<root>/banana-lab.db`, images at `<root>/images`, uploads at `<root>/uploads`).
- `service/` — generation pipeline: `ProxyClient` (HTTP to upstream), `ConcurrencyLimiter` (default 1, `MAX_CONCURRENT`), `GenerateService` (orchestrates DB + storage + proxy), plus `fork_decision`, `image_compress`, `data_url`.
- `api/` — Gin handlers wired in `cmd/server/main.go` (not in `router.go`, which only registers `/api/health`). One handler struct per resource (`PromptsHandler`, `NodesHandler`, `ResultsHandler`, `ArchiveHandler`, `LibraryHandler`, `UploadHandler`, `GenerateHandler`, `PromptTreeHandler`, `PromptSnippetsHandler`).

### Data model notes

- `PromptNode` is the central entity: it carries the prompt + params (`fragments`, `image_size`, `negative_prompt`, `seed`, …), parent pointer for the tree, and reference image IDs. Image-size options: `1K` / `2K` / `4K`; the value is both persisted on the node and forwarded as `image_config.image_size` in the upstream call.
- `Result` links a `PromptNode` to an `Asset`, with `status` in `{active, archived, deleted}` and optional `source_asset_id`/`source_domain` for lineage (img2img / reroll).
- `Asset.domain` = `txt2img` | `img2img`; `Asset.origin` = `generated` | `uploaded`.
- JSON on the wire is `snake_case` (`json:"..."` tags); optional fields use pointer types.

### Frontend (`web/`)

React 19 + Vite 7 + TypeScript strict + Tailwind v4. Routes live in `src/App.tsx`: `/txt2img`, `/img2img`, `/prompts`, `/archive`. Shared API client is `src/lib/api.ts` (unwraps backend `error`/`message` into thrown exceptions; fields are `snake_case`). Dev server proxies `/api` and `/assets` to `VITE_BACKEND_ORIGIN` (default `http://127.0.0.1:8000`).

Playwright config builds the frontend, then starts backend (`SERVER_PORT=18080`, mock upstream, `BANANA_FORCE_RESET_DB=1`, `STORAGE_ROOT=./storage-e2e`), mock-llm, and Vite (`5173`) with `reuseExistingServer` enabled locally. E2E is single-worker (`fullyParallel: false`).

## Conventions

- Go packages: short lowercase (`api`, `service`, `model`). Imports: stdlib → third-party → `banana-lab/...`. Guard-clause error handling, no error swallowing. API error bodies: `{ "error": "invalid" | "internal" | ... }`.
- TS/React: `PascalCase.tsx` component files, single quotes, semicolons, strict TS (no unused vars/params), ESLint flat config at `web/eslint.config.js`. No Prettier config.
- Minimum-diff edits; follow existing style before inventing new patterns.

## Bug-fix workflow (project rule)

Before reporting a bug fixed: (1) read the code to locate it, (2) reproduce (browser or test), capturing logs/screenshots, (3) fix and re-run the repro or tests to prove no regression, (4) only then ask the user to confirm. Don't claim a fix based on reasoning alone — especially for config/startup-flow changes, boot the app locally to verify.
