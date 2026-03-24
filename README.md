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

```bash
wails dev
```

## Build

```bash
wails build
```

# Create a release
git tag v1.0.0
git push origin v1.0.0

## Why "Pedro"?

We didn't name it. The logs just started calling it that and we were too afraid to ask.


# Problems on windows (windows defender issues)
the installer is not signed, so you need to do the following to allow it.
right click on the installer, then click "unblock", apply, save.

then open powershell as admin and run this (you will need to set the actual path)

```ps
Add-MpPreference -AttackSurfaceReductionOnlyExclusions "PATH TO INSTALLER \Pedro-windows-amd64-installer.exe"
```

then run this (if you use the standard installation path, otherwise set your path).

```ps
Add-MpPreference -AttackSurfaceReductionOnlyExclusions "C:\Program Files\Pedro Corp\Pedro\pedro.exe"
```

then double click the installer and it should finally work.