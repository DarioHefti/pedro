import type { ReactNode } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { Field, FieldDescription, FieldLabel } from '@ui-sdk/react/field'
import { Input } from '@ui-sdk/react/input'
import {
  PasswordInput,
  PasswordInputControl,
  PasswordInputToggle,
} from '@ui-sdk/react/password-input'
import { Textarea } from '@ui-sdk/react/textarea'

interface BaseFieldProps {
  label: ReactNode
  description?: ReactNode
  optional?: boolean
  disabled?: boolean
  className?: string
}

function Label({ label, optional }: { label: ReactNode; optional?: boolean }) {
  return (
    <FieldLabel>
      {label}
      {optional ? <span className="field-optional">Optional</span> : null}
    </FieldLabel>
  )
}

interface TextFieldProps extends BaseFieldProps {
  value: string
  onValueChange: (value: string) => void
  placeholder?: string
  mono?: boolean
  autoFocus?: boolean
}

export function TextField({
  label,
  description,
  optional,
  disabled,
  className,
  value,
  onValueChange,
  placeholder,
  mono,
  autoFocus,
}: TextFieldProps) {
  return (
    <Field disabled={disabled} className={className}>
      <Label label={label} optional={optional} />
      <Input
        value={value}
        onValueChange={next => onValueChange(next)}
        placeholder={placeholder}
        className={mono ? 'input-mono' : undefined}
        spellCheck={mono ? false : undefined}
        autoComplete="off"
        autoFocus={autoFocus}
      />
      {description ? <FieldDescription>{description}</FieldDescription> : null}
    </Field>
  )
}

interface SecretFieldProps extends BaseFieldProps {
  value: string
  onValueChange: (value: string) => void
  placeholder?: string
}

export function SecretField({
  label,
  description,
  optional,
  disabled,
  className,
  value,
  onValueChange,
  placeholder,
}: SecretFieldProps) {
  return (
    <Field disabled={disabled} className={className}>
      <Label label={label} optional={optional} />
      <PasswordInput value={value} onValueChange={next => onValueChange(next)}>
        <PasswordInputControl placeholder={placeholder} spellCheck={false} />
        <PasswordInputToggle aria-label="Show secret">
          <Eye className="icon-show" aria-hidden="true" />
          <EyeOff className="icon-hide" aria-hidden="true" />
        </PasswordInputToggle>
      </PasswordInput>
      {description ? <FieldDescription>{description}</FieldDescription> : null}
    </Field>
  )
}

interface TextAreaFieldProps extends BaseFieldProps {
  value: string
  onValueChange: (value: string) => void
  placeholder?: string
  rows?: number
  mono?: boolean
}

export function TextAreaField({
  label,
  description,
  optional,
  disabled,
  className,
  value,
  onValueChange,
  placeholder,
  rows = 5,
  mono,
}: TextAreaFieldProps) {
  return (
    <Field disabled={disabled} className={className}>
      <Label label={label} optional={optional} />
      <Textarea
        value={value}
        onValueChange={next => onValueChange(next)}
        placeholder={placeholder}
        rows={rows}
        data-mono={mono ? '' : undefined}
      />
      {description ? <FieldDescription>{description}</FieldDescription> : null}
    </Field>
  )
}
