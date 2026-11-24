import type { ReactNode } from 'react'
import { Check, ChevronDown } from 'lucide-react'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectItemIndicator,
  SelectTrigger,
  SelectValue,
} from '@ui-sdk/react/select'

export interface SelectOption {
  value: string
  label: string
  description?: string
  icon?: ReactNode
}

interface AppSelectProps {
  value: string
  onValueChange: (value: string) => void
  options: SelectOption[]
  /** Omit inside a Field: its FieldLabel names the trigger. */
  'aria-label'?: string
  placeholder?: string
  disabled?: boolean
  side?: 'top' | 'bottom'
  align?: 'start' | 'center' | 'end'
  className?: string
}

/** Single-value select with the design system's trigger, list and check indicator. */
export function AppSelect({
  value,
  onValueChange,
  options,
  placeholder,
  disabled,
  side = 'bottom',
  align = 'start',
  className,
  ...rest
}: AppSelectProps) {
  return (
    <Select<string>
      items={options.map(o => ({ value: o.value, label: o.label }))}
      value={value}
      onValueChange={next => {
        if (next != null) onValueChange(next)
      }}
      disabled={disabled}
      placeholder={placeholder}
    >
      <SelectTrigger aria-label={rest['aria-label']} className={className}>
        <SelectValue />
        <ChevronDown aria-hidden="true" />
      </SelectTrigger>
      <SelectContent side={side} align={align} sideOffset={6}>
        {options.map(o => (
          <SelectItem key={o.value} value={o.value} label={o.label}>
            {o.icon}
            <span className="select-item-body">
              <span>{o.label}</span>
              {o.description ? <span className="select-item-description">{o.description}</span> : null}
            </span>
            <SelectItemIndicator>
              <Check aria-hidden="true" />
            </SelectItemIndicator>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
