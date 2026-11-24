import { useShortcutHint } from '@ui-sdk/react/shortcuts'

/** Renders a shortcut like "mod+k" as ⌘K on Apple platforms and Ctrl+K elsewhere. */
export function Kbd({ shortcut }: { shortcut: string }) {
  const hint = useShortcutHint(shortcut)
  if (!hint.text) return null
  return (
    <kbd className="kbd" aria-hidden="true">
      {hint.text}
    </kbd>
  )
}
