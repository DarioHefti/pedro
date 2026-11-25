# Pedro

Your friendly desktop AI chat companion. Built with Wails, Go.

## What is this?

A desktop app for chatting with Openai any compatible AI models (azure, lm studio, openai). It stores your conversations locally in sqlite, renders pretty markdown, and occasionally hums while it thinks. 

## Features

- Chat with Openai Compatible LLM's
- Multimodal support - drop images in and ask questions
- Conversations that persist (SQLite is watching)
- Markdown rendering + Mermaid diagrams (because why not)
- Native file picker - because clicking through folders shouldn't be a workout
- Search across all your chats
- Websearch
- Read local files

## Quick Start

1. Configure your LLM provider with credentials in Settings
2. Start chatting
3. Hope the AI is feeling helpful

## Development

Prerequisites: Go 1.25+, Node 22.12+ (for the frontend test tools; building alone works on Node 20.19+), and the Wails CLI.

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails dev
```

Frontend is also served here by default: http://localhost:5173/

### macOS prerequisites

Install Xcode Command Line Tools (required for CGO / Wails):

```bash
xcode-select --install
```

### Windows prerequisites

No extra system dependencies needed beyond Go, Node, and the Wails CLI.

### Linux prerequisites

Pedro builds against WebKitGTK 4.1 (the `webkit2_41` tag is set in `wails.json`, so plain `wails dev` / `wails build` just work).

```bash
# Debian / Ubuntu (22.04+)
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config
# Arch / Omarchy
sudo pacman -S --needed gtk3 webkit2gtk-4.1 base-devel pkgconf
```

### UI without the Go backend

```bash
cd frontend
npm run dev:mock
```

Runs the UI in a browser against an in-memory fake backend (`frontend/e2e/fake-backend`) with demo chats and canned, streamed replies. No database, provider or API key is involved.

## Tests

```bash
go test -race ./...          # backend: store, chat turns, App flows against a scripted fake LLM

cd frontend
npm run typecheck            # app + test code
npm test                     # Vitest unit tests for the pure modules
npx playwright install chromium   # once
npm run test:e2e             # Playwright E2E against a production build and the fake backend
```

- The E2E suite never calls an LLM. Replies are scripted per test (`pedro.queueReply(...)`) or picked from `e2e/fake-backend/replies.ts`.
- `E2E_DEV=1 npm run test:e2e` runs the specs against the Vite dev server, which is faster to iterate on. `npm run test:e2e:ui` opens Playwright's UI mode.
- The fake backend is typed against the generated Wails bindings: after changing an `App` method, run `wails generate module` and `npm run typecheck` shows what the fake is missing.
- `testdata/connection_fingerprints.json` is a contract shared by the Go and TypeScript tests. Both sides must fingerprint provider settings identically.

## Build

```bash
wails build
```

## Create a release

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Why "Pedro"?

We didn't name it. The logs just started calling it that and we were too afraid to ask.

---

## Platform-specific notes

### macOS — Gatekeeper (unsigned app)

The app is not code-signed or notarized, so macOS Gatekeeper will block it on first launch.

**Option A — Right-click → Open:**
Right-click (or Control-click) on `Pedro.app`, select **Open**, then click **Open** in the dialog. You only need to do this once.

**Option B — Remove the quarantine attribute:**

```bash
xattr -cr /path/to/Pedro.app
```

**Data location:** `~/Library/Application Support/Pedro/pedro.db`

### Windows — Windows Defender issues

The installer is not signed, so you need to do the following to allow it.
Right-click on the installer, then click "unblock", apply, save.

Then open PowerShell as admin and run this (set the actual path):

```ps
Add-MpPreference -AttackSurfaceReductionOnlyExclusions "PATH TO INSTALLER\Pedro-windows-amd64-installer.exe"
```

Then run this (if you use the standard installation path, otherwise set your path):

```ps
Add-MpPreference -AttackSurfaceReductionOnlyExclusions "C:\Program Files\Pedro Corp\Pedro\pedro.exe"
```

Then double-click the installer and it should finally work.

**Data location:** `%AppData%\Pedro\pedro.db`

### Linux (X11 and Wayland, incl. Hyprland / Omarchy)

Pedro runs as a native Wayland client. Install it for your user so it shows up in the app launcher with its icon:

```bash
wails build
build/linux/install.sh                 # or: build/linux/install.sh ~/Downloads/Pedro-linux-amd64
build/linux/install.sh --uninstall
```

This puts the binary in `~/.local/bin/pedro` (the in-app updater replaces it in place), plus `pedro.desktop` and the icon under `~/.local/share`. The Wayland app-id / window class is always `pedro`, so you can target it in compositor window rules.

- On tiling compositors the window takes the tile's size (the min size only applies on X11).
- On Wayland Pedro sets `WEBKIT_DISABLE_COMPOSITING_MODE=1` (WebKit otherwise flashes stale frames, e.g. while a reply streams), and on the NVIDIA driver `WEBKIT_DISABLE_DMABUF_RENDERER=1` (avoids a blank window). Set either to `0` yourself to opt out.
- To force XWayland: `GDK_BACKEND=x11 pedro`.

**Data location:** `~/.config/Pedro/pedro.db`

---

## Login with Azure Entra ID

For this to work when you create the Azure OpenAI service you will need to add RBAC roles to the services in IAM.
All the users need this role "Cognitive Services OpenAI User".
