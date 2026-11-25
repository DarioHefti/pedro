import { describe, expect, it } from 'vitest'
// Shared with settings_connection_test.go: both sides must fingerprint settings identically, or a
// remembered connection test is lost (or wrongly kept) when the dialog reopens.
import fingerprintCasesJSON from '../../../testdata/connection_fingerprints.json'
import {
  connectionCheckFromSettings,
  draftFromSettings,
  fingerprint,
  normalizeErrorMessage,
  providerRecord,
} from './settingsModel'

interface FingerprintCase {
  name: string
  settings: Record<string, string>
  fingerprint: string
}
// JSON imports type each case separately; they all share this shape.
const fingerprintCases = fingerprintCasesJSON as unknown as FingerprintCase[]

describe('fingerprint', () => {
  it.each(fingerprintCases.map(c => [c.name, c] as const))('matches the Go fingerprint for %s', (_name, c) => {
    expect(fingerprint(draftFromSettings(c.settings))).toBe(c.fingerprint)
  })

  it('ignores settings that do not affect the connection', () => {
    const base = fingerprintCases[0].settings
    expect(fingerprint(draftFromSettings({ ...base, authenticated: 'false', connection_test: '{}' }))).toBe(
      fingerprintCases[0].fingerprint,
    )
  })

  it('changes when a connection setting changes', () => {
    const base = fingerprintCases[0].settings
    expect(fingerprint(draftFromSettings({ ...base, compat_model: 'another-model' }))).not.toBe(
      fingerprintCases[0].fingerprint,
    )
  })
})

describe('providerRecord', () => {
  it('saves only the selected provider’s keys, trimmed where the backend expects it', () => {
    const draft = draftFromSettings({
      provider_type: 'compat',
      compat_base_url: ' http://localhost:1234/v1 ',
      compat_model: ' local ',
      openai_api_key: 'sk-not-sent',
    })
    expect(providerRecord(draft)).toEqual({
      provider_type: 'compat',
      compat_base_url: 'http://localhost:1234/v1',
      compat_api_key: '',
      compat_model: 'local',
    })
  })

  it('fills media deployment defaults for Azure API key settings never saved before', () => {
    const record = providerRecord(draftFromSettings({ provider_type: 'azure_apikey' }))
    expect(record.azure_image_deployment).toBeTruthy()
    expect(record.azure_transcription_deployment).toBeTruthy()
  })
})

describe('connectionCheckFromSettings', () => {
  const fp = 'fp-1'
  const stored = (record: unknown) => ({ connection_test: JSON.stringify(record) })

  it('is idle without a stored test, for another fingerprint, or for a malformed record', () => {
    expect(connectionCheckFromSettings({}, fp)).toEqual({ kind: 'idle' })
    expect(connectionCheckFromSettings(stored({ ok: true, fingerprint: 'other' }), fp)).toEqual({ kind: 'idle' })
    expect(connectionCheckFromSettings({ connection_test: '{oops' }, fp)).toEqual({ kind: 'idle' })
  })

  it('restores a passed or failed test for the same settings', () => {
    expect(connectionCheckFromSettings(stored({ ok: true, message: 'Connected', at: 42, fingerprint: fp }), fp)).toEqual({
      kind: 'ok',
      message: 'Connected',
      checkedAt: 42,
    })
    expect(connectionCheckFromSettings(stored({ ok: false, at: 7, fingerprint: fp }), fp)).toEqual({
      kind: 'fail',
      message: 'Connection failed',
      checkedAt: 7,
    })
  })
})

it('normalizeErrorMessage drops the backend’s Error: prefix', () => {
  expect(normalizeErrorMessage('Error: 401 unauthorized ')).toBe('401 unauthorized')
  expect(normalizeErrorMessage('Error:')).toBe('Error:')
  expect(normalizeErrorMessage(' timeout ')).toBe('timeout')
})
