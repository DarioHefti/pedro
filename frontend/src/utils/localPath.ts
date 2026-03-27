/** True if `s` looks like an absolute local filesystem path (Windows drive, UNC, or Unix). */
export function looksLikeLocalFilesystemPath(s: string): boolean {
  const t = s.trim()
  if (t.length < 4) {
    return false
  }
  // Windows: C:\ or C:/
  if (/^[a-zA-Z]:[\\/]/.test(t)) {
    return true
  }
  // UNC: \\server\share
  if (t.startsWith('\\\\')) {
    return true
  }
  // Unix absolute: /home/... (not a single slash)
  if (t.startsWith('/') && t.length > 1 && t !== '//') {
    return /^\/[^/]+/.test(t)
  }
  return false
}
