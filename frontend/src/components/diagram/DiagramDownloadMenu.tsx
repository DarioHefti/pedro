import { Download, FileCode, FileImage, Image as ImageIcon } from 'lucide-react'
import { Menu, MenuContent, MenuItem, MenuTrigger } from '@ui-sdk/react/menu'
import { useTheme } from '../../ThemeContext'
import { buttonStyle, type ButtonSize } from '../../ui/button'
import { Hint } from '../../ui/hint'
import { saveDiagram, type DiagramFormat } from './diagramExport'

interface DiagramDownloadMenuProps {
  /** Mermaid source of the diagram. */
  code: string
  size?: ButtonSize
}

const FORMATS: { value: DiagramFormat; label: string; icon: typeof Download }[] = [
  { value: 'svg', label: 'SVG image', icon: FileImage },
  { value: 'png', label: 'PNG image', icon: ImageIcon },
  { value: 'mmd', label: 'Mermaid source', icon: FileCode },
]

/** Saves a diagram as SVG, PNG or Mermaid source through the native save dialog. */
export function DiagramDownloadMenu({ code, size = 'icon-xs' }: DiagramDownloadMenuProps) {
  const { theme } = useTheme()
  return (
    <Menu onAction={value => void saveDiagram(code, theme, value as DiagramFormat)}>
      <Hint label="Download">
        <MenuTrigger {...buttonStyle({ variant: 'ghost', size })} aria-label="Download diagram">
          <Download aria-hidden="true" />
        </MenuTrigger>
      </Hint>
      <MenuContent align="end" sideOffset={6}>
        {FORMATS.map(f => {
          const Icon = f.icon
          return (
            <MenuItem key={f.value} value={f.value}>
              <Icon aria-hidden="true" />
              {f.label}
              <span className="menu-item-hint">.{f.value}</span>
            </MenuItem>
          )
        })}
      </MenuContent>
    </Menu>
  )
}
