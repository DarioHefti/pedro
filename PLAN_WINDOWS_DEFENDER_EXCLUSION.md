# Plan: Windows Defender exclusion helper on the Update button

## Context
On Windows, after installing/updating, Windows Defender's Attack Surface Reduction
can block `pedro.exe`. Users currently have to go to GitHub to find the
PowerShell exclusion command, which is a recurring pain point. We will surface the
exact command directly inside the app (with a one-click copy) so users don't have
to leave the app.

Target command:
```
Add-MpPreference -AttackSurfaceReductionOnlyExclusions "C:\Program Files\Pedro Corp\Pedro\pedro.exe"
```

## Approach (modular, matches existing patterns)

### 1. Backend — `updater.go`
Add a method to the bound `Updater` struct:
- `GetWindowsDefenderExclusion() string`
  - On non-Windows: return `""`.
  - On Windows: resolve the real install path via the existing
    `getWindowsInstallDir()` (defined in `updater_windows.go`) and format:
    `Add-MpPreference -AttackSurfaceReductionOnlyExclusions "<installDir>\pedro.exe"`.
  - This keeps the path accurate (registry `InstallLocation`, else `ProgramFiles`
    env, else default `C:\Program Files\Pedro Corp\Pedro`) instead of hardcoding.
- Put the windows body in `updater_windows.go` (build-tagged `windows`) and a
  non-windows stub in `updater_nonwindows.go` (build-tagged `!windows`),
  mirroring the existing `updater_windows_install.go` / cross-platform pattern.

### 2. Frontend service — `frontend/src/services/wailsService.ts`
- Add `GetWindowsDefenderExclusion` to the `Updater` import block.
- Add to `updaterService`:
  ```ts
  getWindowsDefenderExclusion: (): Promise<string> =>
    useDevStub ? Promise.resolve('') : GetWindowsDefenderExclusion(),
  ```

### 3. Frontend component — `frontend/src/components/UpdateNotification.tsx`
- On mount / when update available, fetch the exclusion command.
- When the command is non-empty (i.e. Windows), render a copyable code block
  below the "Update" / "Later" actions with:
  - explanatory line ("Run this in PowerShell as Admin to prevent Defender from
    blocking Pedro"),
  - a `<code>` block with the command,
  - a "Copy" button that uses `navigator.clipboard.writeText` and shows a
    transient "Copied!" state.
- Keep all existing states (available / downloading / installing / done / error)
  intact; only augment the `available` state.

### 4. Styling — `frontend/src/style.css`
Add design-system-driven classes (reuse existing tokens, no raw colors):
- `.update-exclusion` (wrapper, subtle surface + border)
- `.update-exclusion code` (mono, muted code surface)
- `.update-copy-btn` (ghost button variant matching `.update-btn-dismiss`)

### 5. Verify
- `go build ./...` and `go vet ./...` (windows file cross-compiles under tag;
  run `GOOS=windows go build ./...` to confirm the new windows code compiles).
- Frontend typecheck (`npm --prefix frontend run build` or `tsc --noEmit`).

## Files touched
- `updater.go` (add method signature / non-windows path)
- `updater_windows.go` (windows implementation)
- `frontend/src/services/wailsService.ts` (expose service call)
- `frontend/src/components/UpdateNotification.tsx` (UI + copy)
- `frontend/src/style.css` (styles)

## Out of scope
- Auto-running the command (would require elevation / user consent).
- Light/dark mode toggle work.
- Changing the actual installer behavior.
