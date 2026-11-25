import { describe, expect, it } from 'vitest'
import { formatVersion } from './version'

describe('formatVersion', () => {
  it('shows a release version', () => {
    expect(formatVersion('1.8.0')).toBe('Pedro v1.8.0')
    expect(formatVersion('1.8.0', 'v1.9.0-2-gabc1234')).toBe('Pedro v1.8.0')
  })

  it('names a local build after the source it was built from', () => {
    expect(formatVersion('dev', 'v1.8.0')).toBe('Pedro v1.8.0 (dev)')
    expect(formatVersion('dev', 'v1.8.0-3-gf60f48d')).toBe('Pedro v1.8.0-3-gf60f48d (dev)')
  })

  it('falls back when the version is unknown', () => {
    expect(formatVersion('dev')).toBe('Pedro dev')
    expect(formatVersion('')).toBe('Pedro')
  })
})
