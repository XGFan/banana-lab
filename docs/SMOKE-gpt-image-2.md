# Smoke Test Checklist: gpt-image-2 Support

Execute manually before claiming the feature shipped.

## Prerequisites
- `config.yaml` has `models:` array with `gpt-image-2` as first enabled entry
- `PROXY_BASE_URL`, `PROXY_AUTH_HEADER_VALUE` set for the real upstream proxy
- Fresh shell; no `MODEL_ID` env set (variable is deprecated)

## Steps

1. **Backend startup**
   ```bash
   go run ./cmd/server
   ```
   Expected: server boots on :8000, no errors mentioning MODEL_ID, no legacy-schema-guard panic.

2. **GET /api/models**
   ```bash
   curl http://127.0.0.1:8000/api/models | jq '.models[].id'
   ```
   Expected: `"gpt-image-2"` (and NOT `"gemini-3-pro-image"` since it's disabled).

3. **Frontend render**
   Open `http://127.0.0.1:5173/txt2img` (or the embedded UI). Expected: Model dropdown shows "GPT Image 2" as the default.

4. **txt2img generate with gpt-image-2** — the canary for the whole feature
   In UI: enter prompt "a cat"; click Generate. Expected: an image appears in ~seconds. No console errors.
   If fails: abort rollout; escalate as Spec Constraint #1 or #9 breach.

5. **img2img generate with gpt-image-2 (data URL inline)** — THE LOAD-BEARING TEST per Spec Constraint #9
   In UI: upload a reference image; enter prompt "make it night"; click Generate.
   Expected: an image appears.
   If fails with 4xx on inline data URL: upgrade Constraint #9 and re-plan (do NOT ship).

6. **Reroll with same model**
   On a result, click reroll. Expected: new image generated with same `model_id`.

7. **Fork preserves model_id**
   Fork a result. Expected: new prompt node carries same `model_id` (verify via GET `/api/nodes/:id` or UI state).

## Pass/Fail
- All 7 steps pass → feature ships.
- Step 5 fails → do not ship; escalate as Non-Goal #1 or Constraint #9 revision.
- Any other step fails → debug; likely code-level issue not upstream.
