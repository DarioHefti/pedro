# Pedro

Your friendly desktop AI chat companion. Built with Wails, Go.

## What is this?

A desktop app for chatting with Azure AI models. It stores your conversations locally in sqlite, renders pretty markdown, and occasionally hums while it thinks. 

## Features

- Chat with Azure AI
- Multimodal support - drop images in and ask questions
- Conversations that persist (SQLite is watching)
- Markdown rendering + Mermaid diagrams (because why not)
- Native file picker - because clicking through folders shouldn't be a workout
- Search across all your chats
- Websearch
- Read local files

## Quick Start

1. Configure your Azure AI credentials in Settings
2. Start chatting
3. Hope the AI is feeling helpful

## Development

Prerequisites: Go 1.25+, Node 20+, and the Wails CLI.

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

```bash
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.0-dev build-essential pkg-config
```

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

### Linux

**Data location:** `~/.config/Pedro/pedro.db`

---

## Login with Azure Entra ID

For this to work when you create the Azure OpenAI service you will need to add RBAC roles to the services in IAM.
All the users need this role "Cognitive Services OpenAI User".