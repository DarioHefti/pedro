const DESIGN_LIGHT_BASE_KEY = 'design_light_base_color'
const DESIGN_DARK_BASE_KEY = 'design_dark_base_color'
const DESIGN_MESSAGE_FONT_SIZE_KEY = 'design_message_font_size_px'
const DESIGN_UI_FONT_SIZE_KEY = 'design_ui_font_size_px'

const DEFAULT_LIGHT_BASE = '#242f62'
const DEFAULT_DARK_BASE = '#6478d4'

export const DEFAULT_MESSAGE_FONT_SIZE_PX = 14
export const MESSAGE_FONT_SIZE_SLIDER_MIN_PX = 10
export const MESSAGE_FONT_SIZE_SLIDER_MAX_PX = 22

export const DEFAULT_UI_FONT_SIZE_PX = 13
export const UI_FONT_SIZE_SLIDER_MIN_PX = 10
export const UI_FONT_SIZE_SLIDER_MAX_PX = 18

/** Curated base colors offered as one-click presets in Settings → Appearance. */
export const DESIGN_COLOR_PRESETS: { light: string; dark: string; name: string }[] = [
  { name: 'Pedro', light: '#242f62', dark: '#6478d4' },
  { name: 'Ocean', light: '#0b5d7a', dark: '#38b2d6' },
  { name: 'Forest', light: '#1f5a3d', dark: '#4fbf86' },
  { name: 'Plum', light: '#5b2a6e', dark: '#b07ad8' },
  { name: 'Ember', light: '#8a3412', dark: '#f08a4b' },
  { name: 'Rose', light: '#8c1d4a', dark: '#ec6f9e' },
  { name: 'Graphite', light: '#2f3438', dark: '#9aa5b1' },
  { name: 'Gold', light: '#6b5310', dark: '#e0b84a' },
]

interface Hsl {
  h: number
  s: number
  l: number
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
  return readPx(settings[DESIGN_MESSAGE_FONT_SIZE_KEY], DEFAULT_MESSAGE_FONT_SIZE_PX, MESSAGE_FONT_SIZE_SLIDER_MIN_PX, MESSAGE_FONT_SIZE_SLIDER_MAX_PX)
}

export function applyMessageFontSizeToDocument(px: number) {
  setRootVar('--custom-message-font-size', `${clamp(px, MESSAGE_FONT_SIZE_SLIDER_MIN_PX, MESSAGE_FONT_SIZE_SLIDER_MAX_PX)}px`)
}

export function getUiFontSizePxFromSettings(settings: Record<string, string>): number {
  return readPx(settings[DESIGN_UI_FONT_SIZE_KEY], DEFAULT_UI_FONT_SIZE_PX, UI_FONT_SIZE_SLIDER_MIN_PX, UI_FONT_SIZE_SLIDER_MAX_PX)
}

export function applyUiFontSizeToDocument(px: number) {
  setRootVar('--custom-ui-font-size', `${clamp(px, UI_FONT_SIZE_SLIDER_MIN_PX, UI_FONT_SIZE_SLIDER_MAX_PX)}px`)
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

/**
 * Writes the --custom-{light,dark}-* inputs that styles/tokens.css derives the
 * whole palette from. Lightness is clamped per theme so any base color the
 * person picks still reads on that theme's surfaces.
 */
export function applyDesignPaletteToDocument(palette: DesignPalette) {
  writeTheme('light', lightTokens(hexToHsl(palette.lightBase)))
  writeTheme('dark', darkTokens(hexToHsl(palette.darkBase)))
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

interface ThemeTokens {
  h: number
  tintS: number
  brand: Hsl
  primary: Hsl
  primaryHover: Hsl
  primaryFg: string
}

function lightTokens(base: Hsl): ThemeTokens {
  const s = clamp(base.s, 0, 90)
  const brand = { h: base.h, s, l: clamp(base.l, 16, 44) }
  return {
    h: base.h,
    tintS: Math.min(s, 48),
    brand,
    primary: brand,
    primaryHover: { ...brand, l: Math.max(8, brand.l - 6) },
    primaryFg: readableOn(brand),
  }
}

function darkTokens(base: Hsl): ThemeTokens {
  const s = clamp(base.s, 0, 90)
  const brandL = clamp(base.l + 4, 58, 80)
  const primary = { h: base.h, s, l: clamp(base.l - 5, 42, 62) }
  return {
    h: base.h,
    tintS: Math.min(s, 42),
    brand: { h: base.h, s, l: brandL },
    primary,
    primaryHover: { ...primary, l: Math.min(70, primary.l + 6) },
    primaryFg: readableOn(primary),
  }
}

function writeTheme(theme: 'light' | 'dark', t: ThemeTokens) {
  const p = `--custom-${theme}`
  setRootVar(`${p}-h`, String(round(t.h)))
  setRootVar(`${p}-ts`, `${round(t.tintS)}%`)
  setRootVar(`${p}-brand`, triplet(t.brand))
  setRootVar(`${p}-primary`, triplet(t.primary))
  setRootVar(`${p}-primary-hover`, triplet(t.primaryHover))
  setRootVar(`${p}-primary-fg`, t.primaryFg)
}

/** White or near-black text, whichever contrasts more with the fill. */
function readableOn(fill: Hsl): string {
  const bg = relativeLuminance(hslToRgb(fill))
  const white = 1.05 / (bg + 0.05)
  const ink = (bg + 0.05) / (relativeLuminance(hslToRgb({ h: fill.h, s: 45, l: 9 })) + 0.05)
  return white >= ink * 0.9 ? '0 0% 100%' : `${round(fill.h)} 45% 9%`
}

function triplet(c: Hsl): string {
  return `${round(c.h)} ${round(c.s)}% ${round(c.l)}%`
}

function setRootVar(name: string, value: string) {
  if (typeof document === 'undefined') return
  document.documentElement.style.setProperty(name, value)
}

function readPx(raw: string | undefined, fallback: number, min: number, max: number): number {
  const n = raw?.trim() ? Number.parseInt(raw, 10) : NaN
  return Number.isFinite(n) ? clamp(n, min, max) : fallback
}

function hexToHsl(hex: string): Hsl {
  const v = normalizeHex(hex, '#000000').slice(1)
  const r = Number.parseInt(v.slice(0, 2), 16) / 255
  const g = Number.parseInt(v.slice(2, 4), 16) / 255
  const b = Number.parseInt(v.slice(4, 6), 16) / 255
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const l = (max + min) / 2
  const d = max - min
  if (d === 0) return { h: 0, s: 0, l: l * 100 }
  const s = d / (1 - Math.abs(2 * l - 1))
  let h: number
  if (max === r) h = ((g - b) / d) % 6
  else if (max === g) h = (b - r) / d + 2
  else h = (r - g) / d + 4
  h = (h * 60 + 360) % 360
  return { h, s: s * 100, l: l * 100 }
}

function hslToRgb({ h, s, l }: Hsl): [number, number, number] {
  const sat = s / 100
  const lig = l / 100
  const k = (n: number) => (n + h / 30) % 12
  const a = sat * Math.min(lig, 1 - lig)
  const f = (n: number) => lig - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)))
  return [f(0), f(8), f(4)]
}

function relativeLuminance([r, g, b]: [number, number, number]): number {
  const lin = (c: number) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b)
}

function round(n: number): number {
  return Math.round(n * 10) / 10
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}
