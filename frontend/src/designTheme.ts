const DESIGN_LIGHT_BASE_KEY = 'design_light_base_color'
const DESIGN_DARK_BASE_KEY = 'design_dark_base_color'
const DESIGN_MESSAGE_FONT_SIZE_KEY = 'design_message_font_size_px'
const DESIGN_UI_FONT_SIZE_KEY = 'design_ui_font_size_px'

const DEFAULT_LIGHT_BASE = '#242f62'
const DEFAULT_DARK_BASE = '#6478d4'

export const DEFAULT_MESSAGE_FONT_SIZE_PX = 12
export const MESSAGE_FONT_SIZE_SLIDER_MIN_PX = 10
export const MESSAGE_FONT_SIZE_SLIDER_MAX_PX = 22

export const DEFAULT_UI_FONT_SIZE_PX = 12
export const UI_FONT_SIZE_SLIDER_MIN_PX = 10
export const UI_FONT_SIZE_SLIDER_MAX_PX = 18

interface RgbColor {
  r: number
  g: number
  b: number
}

interface ThemeColorSet {
  accentPrimary: string
  accentSecondary: string
  accentSubtleBg: string
  btnPrimaryBg: string
  btnPrimaryText: string
  btnPrimaryHoverBg: string
  btnGhostText: string
  btnGhostBorder: string
  btnGhostHoverBg: string
  sidebarSelectedBg: string
  sidebarSelectedIndicator: string
  inputFocusBorder: string
  inputFocusShadow: string
  bgGradient: string
}

interface DesignPalette {
  lightBase: string
  darkBase: string
}

export function getDesignSettingsKeys() {
  return {
    light: DESIGN_LIGHT_BASE_KEY,
    dark: DESIGN_DARK_BASE_KEY,
    messageFontSizePx: DESIGN_MESSAGE_FONT_SIZE_KEY,
    uiFontSizePx: DESIGN_UI_FONT_SIZE_KEY,
  }
}

export function getMessageFontSizePxFromSettings(settings: Record<string, string>): number {
  const raw = settings[DESIGN_MESSAGE_FONT_SIZE_KEY]?.trim()
  const n = raw ? Number.parseInt(raw, 10) : NaN
  if (!Number.isFinite(n)) {
    return DEFAULT_MESSAGE_FONT_SIZE_PX
  }
  return clamp(n, MESSAGE_FONT_SIZE_SLIDER_MIN_PX, MESSAGE_FONT_SIZE_SLIDER_MAX_PX)
}

export function applyMessageFontSizeToDocument(px: number) {
  if (typeof document === 'undefined') {
    return
  }
  const clamped = clamp(px, MESSAGE_FONT_SIZE_SLIDER_MIN_PX, MESSAGE_FONT_SIZE_SLIDER_MAX_PX)
  document.documentElement.style.setProperty('--custom-message-font-size', `${clamped}px`)
}

export function getUiFontSizePxFromSettings(settings: Record<string, string>): number {
  const raw = settings[DESIGN_UI_FONT_SIZE_KEY]?.trim()
  const n = raw ? Number.parseInt(raw, 10) : NaN
  if (!Number.isFinite(n)) {
    return DEFAULT_UI_FONT_SIZE_PX
  }
  return clamp(n, UI_FONT_SIZE_SLIDER_MIN_PX, UI_FONT_SIZE_SLIDER_MAX_PX)
}

export function applyUiFontSizeToDocument(px: number) {
  if (typeof document === 'undefined') {
    return
  }
  const clamped = clamp(px, UI_FONT_SIZE_SLIDER_MIN_PX, UI_FONT_SIZE_SLIDER_MAX_PX)
  document.documentElement.style.setProperty('--custom-ui-font-size', `${clamped}px`)
}

/** Applies saved design palette + typography to `document.documentElement`. */
export function applyDesignAndTypographyFromSettings(settings: Record<string, string>) {
  applyDesignPaletteToDocument(getDesignPaletteFromSettings(settings))
  applyMessageFontSizeToDocument(getMessageFontSizePxFromSettings(settings))
  applyUiFontSizeToDocument(getUiFontSizePxFromSettings(settings))
}

export function getDesignPaletteFromSettings(settings: Record<string, string>): DesignPalette {
  return {
    lightBase: normalizeHex(settings[DESIGN_LIGHT_BASE_KEY], DEFAULT_LIGHT_BASE),
    darkBase: normalizeHex(settings[DESIGN_DARK_BASE_KEY], DEFAULT_DARK_BASE),
  }
}

export function applyDesignPaletteToDocument(palette: DesignPalette) {
  if (typeof document === 'undefined') {
    return
  }
  const root = document.documentElement
  const light = buildLightThemeColors(palette.lightBase)
  const dark = buildDarkThemeColors(palette.darkBase)

  setThemeCssVars(root, 'light', light)
  setThemeCssVars(root, 'dark', dark)
}

export function normalizeHex(value: string | null | undefined, fallback: string): string {
  if (!value) return fallback
  const trimmed = value.trim()
  if (/^#[0-9a-fA-F]{6}$/.test(trimmed)) return trimmed.toLowerCase()
  if (/^#[0-9a-fA-F]{3}$/.test(trimmed)) {
    const [, a, b, c] = trimmed
    return `#${a}${a}${b}${b}${c}${c}`.toLowerCase()
  }
  return fallback
}

function setThemeCssVars(root: HTMLElement, theme: 'light' | 'dark', colors: ThemeColorSet) {
  const prefix = `--custom-${theme}`
  root.style.setProperty(`${prefix}-accent-primary`, colors.accentPrimary)
  root.style.setProperty(`${prefix}-accent-secondary`, colors.accentSecondary)
  root.style.setProperty(`${prefix}-accent-subtle-bg`, colors.accentSubtleBg)
  root.style.setProperty(`${prefix}-btn-primary-bg`, colors.btnPrimaryBg)
  root.style.setProperty(`${prefix}-btn-primary-text`, colors.btnPrimaryText)
  root.style.setProperty(`${prefix}-btn-primary-hover-bg`, colors.btnPrimaryHoverBg)
  root.style.setProperty(`${prefix}-btn-ghost-text`, colors.btnGhostText)
  root.style.setProperty(`${prefix}-btn-ghost-border`, colors.btnGhostBorder)
  root.style.setProperty(`${prefix}-btn-ghost-hover-bg`, colors.btnGhostHoverBg)
  root.style.setProperty(`${prefix}-sidebar-selected-bg`, colors.sidebarSelectedBg)
  root.style.setProperty(`${prefix}-sidebar-selected-indicator`, colors.sidebarSelectedIndicator)
  root.style.setProperty(`${prefix}-input-focus-border`, colors.inputFocusBorder)
  root.style.setProperty(`${prefix}-input-focus-shadow`, colors.inputFocusShadow)
  root.style.setProperty(`${prefix}-bg-gradient`, colors.bgGradient)
}

function buildDarkThemeColors(baseHex: string): ThemeColorSet {
  const accentPrimary = mixHex(baseHex, '#ffffff', 0.1)
  const btnPrimaryBg = accentPrimary
  return {
    accentPrimary,
    accentSecondary: mixHex(baseHex, '#ffffff', 0.35),
    accentSubtleBg: withAlpha(baseHex, 0.16),
    btnPrimaryBg,
    btnPrimaryText: pickReadableText(btnPrimaryBg),
    btnPrimaryHoverBg: mixHex(baseHex, '#000000', 0.14),
    btnGhostText: mixHex(baseHex, '#ffffff', 0.24),
    btnGhostBorder: mixHex(baseHex, '#ffffff', 0.14),
    btnGhostHoverBg: withAlpha(baseHex, 0.12),
    sidebarSelectedBg: withAlpha(baseHex, 0.15),
    sidebarSelectedIndicator: mixHex(baseHex, '#ffffff', 0.24),
    inputFocusBorder: mixHex(baseHex, '#ffffff', 0.22),
    inputFocusShadow: `0 0 0 1px ${withAlpha(baseHex, 0.45)}`,
    bgGradient: `linear-gradient(180deg, ${withAlpha(baseHex, 0.14)} 0%, var(--bg) 56%)`,
  }
}

function buildLightThemeColors(baseHex: string): ThemeColorSet {
  const accentPrimary = mixHex(baseHex, '#000000', 0.08)
  const btnPrimaryBg = mixHex(baseHex, '#000000', 0.06)
  return {
    accentPrimary,
    accentSecondary: mixHex(baseHex, '#ffffff', 0.28),
    accentSubtleBg: withAlpha(baseHex, 0.1),
    btnPrimaryBg,
    btnPrimaryText: pickReadableText(btnPrimaryBg),
    btnPrimaryHoverBg: mixHex(baseHex, '#000000', 0.18),
    btnGhostText: accentPrimary,
    btnGhostBorder: mixHex(baseHex, '#000000', 0.12),
    btnGhostHoverBg: withAlpha(baseHex, 0.07),
    sidebarSelectedBg: withAlpha(baseHex, 0.08),
    sidebarSelectedIndicator: accentPrimary,
    inputFocusBorder: mixHex(baseHex, '#000000', 0.15),
    inputFocusShadow: `0 0 0 1px ${withAlpha(baseHex, 0.22)}`,
    bgGradient: `linear-gradient(180deg, ${withAlpha(baseHex, 0.1)} 0%, var(--bg) 60%)`,
  }
}

function withAlpha(hex: string, alpha: number): string {
  const rgb = hexToRgb(hex)
  return `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${clamp(alpha, 0, 1).toFixed(3)})`
}

function mixHex(colorA: string, colorB: string, ratioToB: number): string {
  const a = hexToRgb(colorA)
  const b = hexToRgb(colorB)
  const ratio = clamp(ratioToB, 0, 1)
  const mixed: RgbColor = {
    r: Math.round(a.r + (b.r - a.r) * ratio),
    g: Math.round(a.g + (b.g - a.g) * ratio),
    b: Math.round(a.b + (b.b - a.b) * ratio),
  }
  return rgbToHex(mixed)
}

function pickReadableText(bgHex: string): string {
  const rgb = hexToRgb(bgHex)
  const luminance = relativeLuminance(rgb)
  return luminance > 0.62 ? '#111a34' : '#f8f9ff'
}

function relativeLuminance(rgb: RgbColor): number {
  const channels = [rgb.r, rgb.g, rgb.b].map(value => {
    const normalized = value / 255
    return normalized <= 0.03928
      ? normalized / 12.92
      : ((normalized + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2]
}

function hexToRgb(hex: string): RgbColor {
  const normalized = normalizeHex(hex, '#000000')
  const hexValue = normalized.slice(1)
  return {
    r: Number.parseInt(hexValue.slice(0, 2), 16),
    g: Number.parseInt(hexValue.slice(2, 4), 16),
    b: Number.parseInt(hexValue.slice(4, 6), 16),
  }
}

function rgbToHex(rgb: RgbColor): string {
  const toHex = (value: number) => clamp(Math.round(value), 0, 255).toString(16).padStart(2, '0')
  return `#${toHex(rgb.r)}${toHex(rgb.g)}${toHex(rgb.b)}`
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}
