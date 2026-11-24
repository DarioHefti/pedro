import {
  getDesignPaletteFromSettings,
  getDesignSettingsKeys,
  getMessageFontSizePxFromSettings,
  getUiFontSizePxFromSettings,
} from '../designTheme'
import type { MemoryRecord } from '../services/wailsService'

export const DEFAULT_WELCOME_MESSAGE = 'Welcome to Pedro'
/** Defaults for never-saved media deployments (aligned with providers/azure_apikey). */
export const DEFAULT_IMAGE_DEPLOYMENT = 'gpt-image-2.5-sunburst'
export const DEFAULT_TRANSCRIPTION_DEPLOYMENT = 'gpt-transcribe'
const DEFAULT_OPENAI_MODEL = 'gpt-4o'
const DEFAULT_COMPAT_BASE_URL = 'http://localhost:1234/v1'
const STORED_CONNECTION_KEY = 'connection_test'

export type ProviderType = 'azure' | 'azure_apikey' | 'openai' | 'compat'

/** Everything the backend settings calls need; App wires it to wailsService. */
export interface SettingsServices {
  getSettings: () => Promise<Record<string, string>>
  getDefaultSystemPrompt: () => Promise<string>
  saveSettings: (settings: Record<string, string>) => Promise<void>
  setSetting: (key: string, value: string) => Promise<void>
  testConnection: () => Promise<string>
  signIn: () => Promise<string>
  signOut: () => Promise<void>
  isAuthenticated: () => Promise<boolean>
  getMemories: () => Promise<MemoryRecord[]>
  saveMemory: (key: string, value: string, category: string) => Promise<void>
  deleteMemory: (id: number) => Promise<void>
  getLifetimeStats: () => Promise<{ totalRequests: number; totalTokens: number }>
  getCurrentVersion: () => Promise<string>
}

/** The settings edited in the dialog and persisted together by Save. */
export interface SettingsDraft {
  providerType: string
  endpoint: string
  deployment: string
  azureApiKey: string
  azureTenantId: string
  imageDeployment: string
  transcriptionDeployment: string
  apiKey: string
  model: string
  compatBaseURL: string
  compatApiKey: string
  compatModel: string
  systemPrompt: string
  customSystemPrompt: string
  welcomeMessage: string
  designLightBaseColor: string
  designDarkBaseColor: string
  uiFontSizePx: number
  messageFontSizePx: number
}

export type ConnectionCheck =
  | { kind: 'idle' }
  | { kind: 'ok'; message: string; checkedAt: number }
  | { kind: 'fail'; message: string; checkedAt: number }

export function draftFromSettings(s: Record<string, string>): SettingsDraft {
  const palette = getDesignPaletteFromSettings(s)
  return {
    providerType: s.provider_type ?? 'azure',
    endpoint: s.azure_endpoint ?? '',
    deployment: s.azure_deployment ?? '',
    azureApiKey: s.azure_api_key ?? '',
    azureTenantId: s.azure_tenant_id ?? '',
    imageDeployment: s.azure_image_deployment ?? DEFAULT_IMAGE_DEPLOYMENT,
    transcriptionDeployment: s.azure_transcription_deployment ?? DEFAULT_TRANSCRIPTION_DEPLOYMENT,
    apiKey: s.openai_api_key ?? '',
    model: s.openai_model ?? DEFAULT_OPENAI_MODEL,
    compatBaseURL: s.compat_base_url ?? DEFAULT_COMPAT_BASE_URL,
    compatApiKey: s.compat_api_key ?? '',
    compatModel: s.compat_model ?? '',
    systemPrompt: s.system_prompt ?? '',
    customSystemPrompt: s.custom_system_prompt ?? '',
    welcomeMessage: s.welcome_message ?? DEFAULT_WELCOME_MESSAGE,
    designLightBaseColor: palette.lightBase,
    designDarkBaseColor: palette.darkBase,
    uiFontSizePx: getUiFontSizePxFromSettings(s),
    messageFontSizePx: getMessageFontSizePxFromSettings(s),
  }
}

/** Settings persisted via SaveSettings for the selected provider. */
export function providerRecord(d: SettingsDraft): Record<string, string> {
  const settings: Record<string, string> = { provider_type: d.providerType }
  if (d.providerType === 'azure') {
    settings.azure_endpoint = d.endpoint
    settings.azure_deployment = d.deployment
    settings.azure_tenant_id = d.azureTenantId.trim()
  } else if (d.providerType === 'azure_apikey') {
    settings.azure_endpoint = d.endpoint
    settings.azure_deployment = d.deployment
    settings.azure_api_key = d.azureApiKey
    settings.azure_image_deployment = d.imageDeployment.trim()
    settings.azure_transcription_deployment = d.transcriptionDeployment.trim()
  } else if (d.providerType === 'openai') {
    settings.openai_api_key = d.apiKey
    settings.openai_model = d.model
  } else if (d.providerType === 'compat') {
    settings.compat_base_url = d.compatBaseURL.trim()
    settings.compat_api_key = d.compatApiKey
    settings.compat_model = d.compatModel.trim()
  }
  return settings
}

/** The non-provider settings Save writes one key at a time. */
export function extraSettingEntries(d: SettingsDraft): [string, string][] {
  const keys = getDesignSettingsKeys()
  return [
    ['system_prompt', d.systemPrompt],
    ['custom_system_prompt', d.customSystemPrompt],
    ['welcome_message', d.welcomeMessage],
    [keys.light, d.designLightBaseColor],
    [keys.dark, d.designDarkBaseColor],
    [keys.uiFontSizePx, String(d.uiFontSizePx)],
    [keys.messageFontSizePx, String(d.messageFontSizePx)],
  ]
}

/** Stable identity of a draft; the backend stores it with the last connection test. */
export function fingerprint(d: SettingsDraft): string {
  const rec: Record<string, string> = { ...providerRecord(d), ...Object.fromEntries(extraSettingEntries(d)) }
  return JSON.stringify(Object.keys(rec).sort().map(k => [k, rec[k]]))
}

export function connectionCheckFromSettings(s: Record<string, string>, fp: string): ConnectionCheck {
  const raw = s[STORED_CONNECTION_KEY]?.trim()
  if (!raw) return { kind: 'idle' }
  try {
    const o = JSON.parse(raw) as { ok?: boolean; message?: string; at?: number; fingerprint?: string }
    if (typeof o.fingerprint !== 'string' || o.fingerprint !== fp) return { kind: 'idle' }
    const at = typeof o.at === 'number' && !Number.isNaN(o.at) ? o.at : Date.now()
    if (o.ok === true) {
      return { kind: 'ok', message: typeof o.message === 'string' ? o.message : 'Connection successful!', checkedAt: at }
    }
    if (o.ok === false) {
      return { kind: 'fail', message: typeof o.message === 'string' ? o.message : 'Connection failed', checkedAt: at }
    }
  } catch {
    /* a malformed record reads as untested */
  }
  return { kind: 'idle' }
}

export function normalizeErrorMessage(raw: string): string {
  const trimmed = raw.trim()
  return trimmed.startsWith('Error:') ? trimmed.slice(6).trim() || trimmed : trimmed
}

export function formatCheckedAt(ts: number): string {
  try {
    return new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(ts))
  } catch {
    return ''
  }
}
