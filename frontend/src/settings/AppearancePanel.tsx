import type { CSSProperties } from 'react'
import { Check, RotateCcw } from 'lucide-react'
import {
  ColorPicker,
  ColorPickerArea,
  ColorPickerAreaThumb,
  ColorPickerInput,
  ColorPickerSlider,
  ColorPickerSliderThumb,
  ColorPickerSwatch,
} from '@ui-sdk/react/color-picker'
import { Field, FieldDescription, FieldLabel } from '@ui-sdk/react/field'
import { Popover, PopoverContent, PopoverTrigger } from '@ui-sdk/react/popover'
import { Slider, SliderControl, SliderIndicator, SliderThumb, SliderTrack } from '@ui-sdk/react/slider'
import {
  DESIGN_COLOR_PRESETS,
  MESSAGE_FONT_SIZE_SLIDER_MAX_PX,
  MESSAGE_FONT_SIZE_SLIDER_MIN_PX,
  normalizeHex,
  UI_FONT_SIZE_SLIDER_MAX_PX,
  UI_FONT_SIZE_SLIDER_MIN_PX,
} from '../designTheme'
import { Button } from '../ui/button'
import { TextField } from '../ui/fields'
import { DEFAULT_WELCOME_MESSAGE } from './settingsModel'
import type { SettingsDraftState } from './useSettingsDraft'

/** The preset's own colors are data, handed to CSS as custom properties. */
function swatchVars(light: string, dark: string): CSSProperties {
  return { '--swatch-light': light, '--swatch-dark': dark } as CSSProperties
}

function BaseColorControl({ label, value, onChange }: { label: string; value: string; onChange: (hex: string) => void }) {
  return (
    <ColorPicker aria-label={label} value={value} onValueChange={v => onChange(normalizeHex(v, value))} space="hsv" format="hex">
      <Popover>
        <PopoverTrigger className="color-trigger">
          <ColorPickerSwatch className="color-trigger-swatch" />
          <span className="color-trigger-text">
            <strong>{label}</strong>
            <code>{value}</code>
          </span>
        </PopoverTrigger>
        <PopoverContent side="bottom" align="start" sideOffset={8} className="color-popover">
          <ColorPickerArea>
            <ColorPickerAreaThumb />
          </ColorPickerArea>
          <ColorPickerSlider channel="hue">
            <ColorPickerSliderThumb />
          </ColorPickerSlider>
          <ColorPickerInput aria-label={`${label} hex value`} />
        </PopoverContent>
      </Popover>
    </ColorPicker>
  )
}

function ThemePreview({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <div className="theme-preview" data-theme={theme} aria-hidden="true">
      <div className="tp-sidebar">
        <span className="tp-brand" />
        <span className="tp-new" />
        <span className="tp-row" data-active="" />
        <span className="tp-row" />
        <span className="tp-row" />
      </div>
      <div className="tp-main">
        <span className="tp-bubble tp-user" />
        <span className="tp-bubble tp-assistant">
          <span />
          <span />
          <span />
        </span>
        <span className="tp-composer">
          <span className="tp-send" />
        </span>
      </div>
      <span className="tp-label">{theme === 'light' ? 'Light' : 'Dark'}</span>
    </div>
  )
}

function SizeSlider({
  label,
  description,
  value,
  min,
  max,
  onChange,
}: {
  label: string
  description: string
  value: number
  min: number
  max: number
  onChange: (px: number) => void
}) {
  return (
    <Field className="size-field">
      <div className="settings-label-row">
        <FieldLabel>{label}</FieldLabel>
        <output className="size-value">{value}px</output>
      </div>
      <Slider<number> value={value} min={min} max={max} step={1} onValueChange={v => onChange(v)}>
        <SliderControl>
          <SliderTrack>
            <SliderIndicator />
            <SliderThumb />
          </SliderTrack>
        </SliderControl>
      </Slider>
      <FieldDescription>{description}</FieldDescription>
    </Field>
  )
}

export function AppearancePanel({ s }: { s: SettingsDraftState }) {
  const { draft, update } = s
  const activePreset = DESIGN_COLOR_PRESETS.find(
    p => p.light === draft.designLightBaseColor && p.dark === draft.designDarkBaseColor,
  )

  return (
    <div className="settings-stack">
      <TextField
        label="Welcome message"
        description="Greets you on every new chat."
        value={draft.welcomeMessage}
        onValueChange={v => update({ welcomeMessage: v })}
        placeholder={DEFAULT_WELCOME_MESSAGE}
      />

      <section className="settings-section" aria-labelledby="accent-heading">
        <div className="settings-section-head">
          <h4 id="accent-heading">Accent color</h4>
          <p>Tints the whole interface. Pick a preset or fine-tune each theme.</p>
        </div>
        <div className="preset-grid" role="group" aria-label="Color presets">
          {DESIGN_COLOR_PRESETS.map(p => {
            const selected = activePreset?.name === p.name
            return (
              <button
                key={p.name}
                type="button"
                className="preset"
                aria-pressed={selected}
                onClick={() => update({ designLightBaseColor: p.light, designDarkBaseColor: p.dark })}
              >
                <span className="preset-swatch" style={swatchVars(p.light, p.dark)} aria-hidden="true">
                  {selected ? <Check /> : null}
                </span>
                <span className="preset-name">{p.name}</span>
              </button>
            )
          })}
        </div>
        <div className="color-controls">
          <BaseColorControl label="Light theme" value={draft.designLightBaseColor} onChange={hex => update({ designLightBaseColor: hex })} />
          <BaseColorControl label="Dark theme" value={draft.designDarkBaseColor} onChange={hex => update({ designDarkBaseColor: hex })} />
        </div>
        <div className="theme-previews">
          <ThemePreview theme="light" />
          <ThemePreview theme="dark" />
        </div>
      </section>

      <section className="settings-section" aria-labelledby="type-heading">
        <div className="settings-section-head">
          <h4 id="type-heading">Text size</h4>
          <p>Scales the interface and chat independently.</p>
        </div>
        <div className="settings-grid">
          <SizeSlider
            label="Interface"
            description="Sidebar, buttons, composer and dialogs."
            value={draft.uiFontSizePx}
            min={UI_FONT_SIZE_SLIDER_MIN_PX}
            max={UI_FONT_SIZE_SLIDER_MAX_PX}
            onChange={px => update({ uiFontSizePx: px })}
          />
          <SizeSlider
            label="Chat messages"
            description="The text of every message."
            value={draft.messageFontSizePx}
            min={MESSAGE_FONT_SIZE_SLIDER_MIN_PX}
            max={MESSAGE_FONT_SIZE_SLIDER_MAX_PX}
            onChange={px => update({ messageFontSizePx: px })}
          />
        </div>
        <p className="type-sample">Pedro hums quietly while it reads your files and drafts a careful answer.</p>
      </section>

      <div>
        <Button variant="ghost" size="sm" onClick={s.resetAppearance}>
          <RotateCcw aria-hidden="true" />
          Reset appearance to defaults
        </Button>
      </div>
    </div>
  )
}
