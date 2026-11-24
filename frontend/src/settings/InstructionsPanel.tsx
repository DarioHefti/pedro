import { ChevronRight, RotateCcw, ScrollText } from 'lucide-react'
import { Collapsible, CollapsiblePanel, CollapsibleTrigger } from '@ui-sdk/react/collapsible'
import { Field, FieldDescription, FieldLabel } from '@ui-sdk/react/field'
import { Textarea } from '@ui-sdk/react/textarea'
import { Button } from '../ui/button'
import { TextAreaField } from '../ui/fields'
import type { SettingsDraftState } from './useSettingsDraft'

const CUSTOM_PLACEHOLDER = `Examples:
- Always respond in a formal tone
- Prioritize Swiss sources when available
- Use bullet points for lists`

export function InstructionsPanel({ s }: { s: SettingsDraftState }) {
  const { draft, update, defaultSystemPrompt } = s
  const effectivePrompt = draft.systemPrompt || defaultSystemPrompt
  const isDefault = !draft.systemPrompt || draft.systemPrompt === defaultSystemPrompt

  return (
    <div className="settings-stack">
      <TextAreaField
        label="Additional instructions"
        description="Appended after the system prompt on every request — tone, preferred sources, formatting."
        value={draft.customSystemPrompt}
        onValueChange={v => update({ customSystemPrompt: v })}
        placeholder={CUSTOM_PLACEHOLDER}
        rows={7}
      />

      <Collapsible className="settings-disclosure">
        <CollapsibleTrigger className="settings-disclosure-trigger">
          <ScrollText aria-hidden="true" />
          <span className="settings-disclosure-text">
            <strong>System prompt</strong>
            <span>The base instructions that define Pedro's identity and behavior.</span>
          </span>
          {isDefault ? (
            <span data-badge="neutral">Default</span>
          ) : (
            <span data-badge="brand">Customized</span>
          )}
          <ChevronRight className="settings-disclosure-chevron" aria-hidden="true" />
        </CollapsibleTrigger>
        <CollapsiblePanel>
          <div className="settings-disclosure-body">
            <Field>
              <div className="settings-label-row">
                <FieldLabel>Prompt</FieldLabel>
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => update({ systemPrompt: defaultSystemPrompt })}
                  disabled={isDefault}
                >
                  <RotateCcw aria-hidden="true" />
                  Reset to default
                </Button>
              </div>
              <Textarea
                className="prompt-editor"
                value={effectivePrompt}
                onValueChange={v => update({ systemPrompt: v })}
                rows={16}
                spellCheck={false}
              />
              <FieldDescription>Changes apply to new messages after you save.</FieldDescription>
            </Field>
          </div>
        </CollapsiblePanel>
      </Collapsible>
    </div>
  )
}
