#!/usr/bin/env bash
# Installs Pedro for the current user: binary, launcher entry and icon.
#
#   build/linux/install.sh [path/to/pedro-binary]      (default: build/bin/pedro)
#   build/linux/install.sh --uninstall
#
# Honors PREFIX (default ~/.local). The in-app updater replaces the installed binary in place.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
prefix="${PREFIX:-$HOME/.local}"
bin="$prefix/bin/pedro"
desktop="$prefix/share/applications/pedro.desktop"
icon="$prefix/share/icons/hicolor/512x512/apps/pedro.png"

refresh() {
  command -v update-desktop-database >/dev/null && update-desktop-database -q "$prefix/share/applications" || true
  command -v gtk-update-icon-cache >/dev/null && gtk-update-icon-cache -qtf "$prefix/share/icons/hicolor" || true
}

if [[ "${1:-}" == "--uninstall" ]]; then
  rm -f "$bin" "$desktop" "$icon"
  refresh
  echo "Pedro uninstalled (your chats in ~/.config/Pedro are kept)."
  exit 0
fi

src="${1:-$root/build/bin/pedro}"
[[ -f "$src" ]] || { echo "Binary not found: $src (run 'wails build' first)" >&2; exit 1; }

install -Dm755 "$src" "$bin"
install -Dm644 "$root/build/appicon.png" "$icon"
mkdir -p "$(dirname "$desktop")"
# Absolute Exec so launchers work even when ~/.local/bin is not on their PATH.
sed "s|^Exec=pedro|Exec=$bin|" "$here/pedro.desktop" > "$desktop"
chmod 644 "$desktop"
refresh

echo "Installed Pedro to $bin"
case ":$PATH:" in *":$prefix/bin:"*) ;; *) echo "Note: $prefix/bin is not on your PATH." ;; esac
