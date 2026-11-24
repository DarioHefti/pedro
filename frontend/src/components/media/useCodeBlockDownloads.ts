import { useEffect } from 'react'
import { fileService, isWailsDevStub } from '../../services/wailsService'
import { saveWithFeedback } from './mediaActions'

/** Language tag → common file extension used for downloads. */
const LANGUAGE_EXTENSIONS: Record<string, string> = {
  javascript: 'js',
  typescript: 'ts',
  python: 'py',
  rust: 'rs',
  go: 'go',
  java: 'java',
  kotlin: 'kt',
  swift: 'swift',
  ruby: 'rb',
  php: 'php',
  csharp: 'cs',
  c: 'c',
  cpp: 'cpp',
  'c++': 'cpp',
  shell: 'sh',
  bash: 'sh',
  zsh: 'zsh',
  powershell: 'ps1',
  html: 'html',
  css: 'css',
  scss: 'scss',
  json: 'json',
  yaml: 'yaml',
  yml: 'yml',
  xml: 'xml',
  sql: 'sql',
  markdown: 'md',
  mermaid: 'mmd',
  dockerfile: 'Dockerfile',
  vim: 'vim',
  lua: 'lua',
  perl: 'pl',
  r: 'r',
  scala: 'scala',
  groovy: 'groovy',
  dart: 'dart',
  elixir: 'ex',
  haskell: 'hs',
  clojure: 'clj',
  erlang: 'erl',
  ocaml: 'ml',
  fsharp: 'fs',
  solidity: 'sol',
}

function suggestedFilename(language: string): string {
  return `download.${LANGUAGE_EXTENSIONS[language.toLowerCase()] || 'txt'}`
}

/**
 * Routes Streamdown's code-block download buttons through the native save dialog
 * (webviews ignore blob downloads). Mount once: one listener serves every message.
 */
export function useCodeBlockDownloads(): void {
  useEffect(() => {
    if (isWailsDevStub) return
    const controller = new AbortController()

    const onClick = (e: MouseEvent) => {
      const button = (e.target as HTMLElement).closest('[data-streamdown="code-block-download-button"]')
      if (!button) return
      e.stopPropagation()
      e.preventDefault()

      const block = button.closest('[data-streamdown="code-block"]')
      const pre = block?.querySelector('pre')
      if (!block || !pre) return
      const language = (block.querySelector('span[class*="font-mono"]')?.textContent || 'txt').trim()
      void saveWithFeedback(fileService.saveFile(suggestedFilename(language), pre.textContent || ''), 'File')
    }

    document.addEventListener('click', onClick, { capture: true, signal: controller.signal })
    return () => controller.abort()
  }, [])
}
