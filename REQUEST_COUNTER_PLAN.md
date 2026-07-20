# Pedro — LLM Request Counter (Cross-Platform Plan)

## Goal
Show how many HTTP requests have been made to the LLM **per chat**, a **global total** in
Settings, and a **live counter in the app window's top corner** that increments per request.
Clicking it reveals a popover with **estimated context (prompt + completion tokens, derived
bytes)** per recent request.

## Platform handling (cross-platform requirement)
Wails uses the **native title bar on macOS, Windows, and Linux** (no `Frameless` set in
`main.go:21-42`). Native window controls differ per OS:
- **macOS** → traffic lights top-**left**; dock counter top-**right** with ~70px right clearance.
- **Windows** → min/max/close top-**right**; dock counter top-**left** (right of the title/icon area).
- **Linux** → decorations vary; default to the Windows-style top-**left** dock to avoid the usual
  right-side buttons.

The frontend will call the existing `wailsjs/runtime` `Environment()` (returns
`platform: "darwin" | "windows" | "linux"`) at startup and choose the dock side + inset
accordingly. No new backend binding required for platform detection.

## Decisions (confirmed with user)
- **Unit:** every HTTP request to the provider (tool-call sub-requests counted separately).
- **Popover metric:** prompt + completion tokens + derived bytes (~4 B/token).
- **Persistence:** per-chat count in a `conversations` column; global total in a `settings` key —
  both survive restarts.

## Backend (Go)
1. **Schema migration** (`database.go`, `migrate()`): add `request_count INTEGER NOT NULL DEFAULT 0`
   to `conversations`.
2. **DB helpers** (`database.go`): `IncrementRequestCount(convID)` (per-chat +1, transactional),
   `GetRequestCount(convID) int`, and global tally via existing `settings` table key
   `global_request_count` (`GetSetting`/`SetSetting` already exist).
3. **Capture usage** (`providers/openaiutil/chat.go:130-208`): the stream loop already accumulates
   chunks. Read `chunk.Usage` (or the `ChatCompletionAccumulator`'s usage) per HTTP request; return
   a `RequestStat{PromptTokens, CompletionTokens}` from `Chat` (add to return signature or a
   callback param). Note: OpenAI streaming usage is only present on the *final* chunk, so capture it
   at loop end per request.
4. **Count + emit** (`app.go` `runChat`, `:109-158`): inside the inner HTTP request completion
   path, increment per-chat + global counts (persist), then
   `runtime.EventsEmit(ctx, "request_count_updated", convID, perChat, global, promptTokens,
   completionTokens)`.
5. **Expose bindings** on `App`: `GetRequestCounts(convID) (perChat, global int)`,
   `GetGlobalRequestCount() int`, `GetLifetimeStats() (totalRequests, totalPromptTokens,
   totalCompletionTokens)`. Then run `wails generate` to refresh
   `frontend/wailsjs/go/main/App.{js,d.ts}`.

## Frontend (React/TS)
6. **Service** (`wailsService.ts`): add `statsService` wrapping the new bindings; expose
   `eventService.on("request_count_updated", cb)` (already supported).
7. **Live counter component** `RequestCounter.tsx` (new): platform-aware fixed overlay (uses
   `Environment()` to pick top-left on Windows/Linux, top-right on macOS). Shows global count, pulses
   on increment. Click → popover listing last ~10 requests (prompt/completion tokens, est. bytes,
   timestamp) kept in component state from the event stream. Model styling on
   `UpdateNotification.tsx`. Must work at `MinWidth: 808` — ensure it never overlaps chat controls.
8. **Per-chat count** (`Chat.tsx`): show per-chat request count in the chat header, sourced from
   `useMessaging`/`request_count_updated`.
9. **Settings** (`SettingsModal.tsx`): new **"Stats"** tab → global total requests + lifetime
   estimated tokens/bytes.

## Design
- Reuse existing tokens in `style.css` / `designTheme.ts`. Counter = subtle pill variant;
  popover/card reuse existing modal styling. No ad-hoc `text-white`/`bg-white` — all via
  design-system tokens. Responsive + works in both light/dark.

## Verification
- `go build ./...` and `wails generate` succeed (no compile errors — required by repo guidelines).
- `npm run build` (frontend) compiles; `tsc` clean.
- Manual: send messages (incl. a tool-call turn) → top counter increments per HTTP request; click
  popover shows tokens/bytes; per-chat header updates; Settings Stats tab shows global lifetime
  totals; restart app → counts persist; confirm counter docks correctly on macOS (top-right) and
  Windows/Linux (top-left).

## Out of scope (first version)
- Per-request context history persistence (popover is session-only, last ~10).
- Actual serialized payload byte measurement (using token-derived estimate per decision).
