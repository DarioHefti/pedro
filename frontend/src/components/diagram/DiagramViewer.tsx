import { useMemo, useRef, type CSSProperties, type KeyboardEvent } from 'react'
import { Copy, Scan, Workflow, X, ZoomIn, ZoomOut } from 'lucide-react'
import {
  Dialog,
  DialogBackdrop,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
  createDialogController,
} from '@ui-sdk/react/dialog'
import { useTheme } from '../../ThemeContext'
import { Button, buttonStyle } from '../../ui/button'
import { Hint } from '../../ui/hint'
import { DiagramDownloadMenu } from './DiagramDownloadMenu'
import { copyDiagramImage } from './diagramExport'
import { withFreshIds, type RenderedDiagram } from './mermaidRenderer'
import { usePanZoom } from './usePanZoom'

export interface DiagramViewerPayload {
  diagram: RenderedDiagram
  code: string
}

/** Opens a diagram full screen: `diagramViewer.open({ payload })`. Mount <DiagramViewer /> once. */
export const diagramViewer = createDialogController<DiagramViewerPayload>()

const PAN_STEP = 60

function ViewerBody({ diagram: source, code }: DiagramViewerPayload) {
  const { theme } = useTheme()
  const diagram = useMemo(() => withFreshIds(source, 'viewer'), [source])
  const canvasRef = useRef<HTMLDivElement>(null)
  const pz = usePanZoom(canvasRef, diagram)

  /** Viewer keys work wherever focus is in the viewer, except inside an open menu. */
  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if ((e.target as HTMLElement).closest('[role="menu"]')) return
    const actions: Record<string, () => void> = {
      '+': pz.zoomIn,
      '=': pz.zoomIn,
      '-': pz.zoomOut,
      '0': pz.fit,
      '1': pz.actualSize,
      ArrowLeft: () => pz.panBy(PAN_STEP, 0),
      ArrowRight: () => pz.panBy(-PAN_STEP, 0),
      ArrowUp: () => pz.panBy(0, PAN_STEP),
      ArrowDown: () => pz.panBy(0, -PAN_STEP),
    }
    const action = actions[e.key]
    if (!action || e.metaKey || e.ctrlKey || e.altKey) return
    e.preventDefault()
    action()
  }

  const stageStyle = {
    '--diagram-w': `${diagram.width}px`,
    '--diagram-h': `${diagram.height}px`,
    '--pz-x': `${pz.view.x}px`,
    '--pz-y': `${pz.view.y}px`,
    '--pz-scale': pz.view.scale,
  } as CSSProperties

  return (
    <div className="diagram-viewer-body" onKeyDown={onKeyDown}>
      <header className="diagram-viewer-bar">
        <span className="dialog-icon" aria-hidden="true">
          <Workflow />
        </span>
        <div className="diagram-viewer-title">
          <DialogTitle>Diagram</DialogTitle>
          <DialogDescription>Drag to move · ⌘/Ctrl + scroll to zoom</DialogDescription>
        </div>

        <div className="diagram-zoom" role="group" aria-label="Zoom">
          <Hint label="Zoom out (−)" side="bottom">
            <Button variant="ghost" size="icon-sm" onClick={pz.zoomOut} disabled={!pz.canZoomOut}>
              <ZoomOut aria-hidden="true" />
            </Button>
          </Hint>
          <Hint label="Actual size (1)" side="bottom">
            <button type="button" className="diagram-zoom-value" onClick={pz.actualSize}>
              {Math.round(pz.view.scale * 100)}%
            </button>
          </Hint>
          <Hint label="Zoom in (+)" side="bottom">
            <Button variant="ghost" size="icon-sm" onClick={pz.zoomIn} disabled={!pz.canZoomIn}>
              <ZoomIn aria-hidden="true" />
            </Button>
          </Hint>
          <Hint label="Fit to screen (0)" side="bottom">
            <Button variant="ghost" size="icon-sm" onClick={pz.fit}>
              <Scan aria-hidden="true" />
            </Button>
          </Hint>
        </div>

        <span className="toolbar-divider" aria-hidden="true" />

        <Hint label="Copy as image" side="bottom">
          <Button variant="ghost" size="icon-sm" onClick={() => void copyDiagramImage(code, theme)}>
            <Copy aria-hidden="true" />
          </Button>
        </Hint>
        <DiagramDownloadMenu code={code} size="icon-sm" />
        <Hint label="Close" shortcut="escape" side="bottom">
          <DialogClose {...buttonStyle({ variant: 'ghost', size: 'icon-sm' })} aria-label="Close viewer">
            <X aria-hidden="true" />
          </DialogClose>
        </Hint>
      </header>

      <div
        ref={canvasRef}
        className="diagram-canvas"
        tabIndex={0}
        role="application"
        aria-roledescription="diagram canvas"
        aria-label="Diagram. Arrow keys move it, plus and minus zoom, 0 fits it to the screen."
        data-dragging={pz.dragging ? '' : undefined}
        {...pz.handlers}
      >
        <div className="diagram-stage" style={stageStyle} dangerouslySetInnerHTML={{ __html: diagram.svg }} />
      </div>
    </div>
  )
}

export function DiagramViewer() {
  return (
    <Dialog controller={diagramViewer}>
      <DialogBackdrop />
      <DialogContent<DiagramViewerPayload> data-size="viewer" className="diagram-viewer" initialFocus=".diagram-canvas">
        {state => (state.payload ? <ViewerBody {...state.payload} /> : null)}
      </DialogContent>
    </Dialog>
  )
}
