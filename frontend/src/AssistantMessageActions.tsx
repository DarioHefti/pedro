export function IconCopy({ size = 15 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <rect
        x="9"
        y="9"
        width="13"
        height="13"
        rx="2"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinejoin="round"
      />
      <path
        d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinejoin="round"
      />
    </svg>
  )
}

export function IconRegenerate({ size = 15 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <path
        d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M3 3v5h5"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M21 21v-5h-5"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

interface AssistantMessageActionsProps {
  onCopy: () => void
  onRegenerate: () => void
  regenerateDisabled?: boolean
  copyDisabled?: boolean
  visible: boolean
}

export default function AssistantMessageActions({
  onCopy,
  onRegenerate,
  regenerateDisabled,
  copyDisabled,
  visible,
}: AssistantMessageActionsProps) {
  if (!visible) return null

  return (
    <div
      className="message-actions message-actions--assistant-peek-br"
      role="group"
      aria-label="Assistant message actions"
    >
      <button
        type="button"
        className="message-action-btn"
        onClick={onCopy}
        disabled={copyDisabled}
        title="Copy message"
        aria-label="Copy message"
      >
        <IconCopy />
      </button>
      <button
        type="button"
        className="message-action-btn"
        onClick={onRegenerate}
        disabled={regenerateDisabled}
        title="Regenerate response"
        aria-label="Regenerate response"
      >
        <IconRegenerate />
      </button>
    </div>
  )
}
