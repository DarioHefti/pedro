/** App-wide keyboard shortcuts. `mod` is ⌘ on Apple platforms and Ctrl elsewhere. */
export const SHORTCUTS = {
  commandPalette: 'mod+k',
  newChat: 'mod+n',
  /** Also starts a new chat; not shown in hints, which show `newChat`. */
  newChatAlt: 'mod+t',
  toggleSidebar: 'mod+b',
  settings: 'mod+,',
  requestDetails: 'mod+shift+d',
} as const
