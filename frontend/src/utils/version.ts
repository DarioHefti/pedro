/**
 * The version line in the settings. Release builds report their version ("1.8.0" → "Pedro
 * v1.8.0"). A local build (`wails dev`) is not stamped and reports "dev"; it is named after the
 * git tag of the source it was built from, e.g. "Pedro v1.8.0 (dev)", when that is known.
 */
export function formatVersion(version: string, sourceVersion = ''): string {
  if (version === 'dev' && sourceVersion) return `Pedro ${sourceVersion} (dev)`
  if (!version) return 'Pedro'
  return /^\d/.test(version) ? `Pedro v${version}` : `Pedro ${version}`
}
