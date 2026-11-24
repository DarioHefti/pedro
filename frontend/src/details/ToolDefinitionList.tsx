import { ChevronRight } from 'lucide-react'
import { Collapsible, CollapsiblePanel, CollapsibleTrigger } from '@ui-sdk/react/collapsible'
import { CopyButton } from './DetailsButton'
import { summarizeToolParams } from './payload'
import type { OAIToolDef } from './types'

function ToolDefinitionCard({ tool }: { tool: OAIToolDef }) {
  const name = tool.function?.name ?? 'unknown'
  const desc = tool.function?.description
  const schema = tool.function?.parameters
  const params = summarizeToolParams(tool)
  const requiredCount = params.filter(p => p.required).length

  return (
    <Collapsible className="rd-tooldef">
      <CollapsibleTrigger className="rd-tooldef-head">
        <span className="rd-tooldef-title">
          <ChevronRight className="rd-chevron" aria-hidden="true" />
          <span className="rd-tooldef-name">{name}</span>
          <span className="rd-tooldef-count">
            {params.length} param{params.length === 1 ? '' : 's'}
            {requiredCount > 0 && ` · ${requiredCount} required`}
          </span>
        </span>
        {desc && <span className="rd-tooldef-desc">{desc}</span>}
      </CollapsibleTrigger>

      <CollapsiblePanel className="rd-tooldef-body">
        {params.length > 0 ? (
          <div className="rd-table-wrap">
            <table className="rd-table">
              <thead>
                <tr>
                  <th>Parameter</th>
                  <th>Type</th>
                  <th>Description</th>
                </tr>
              </thead>
              <tbody>
                {params.map(p => (
                  <tr key={p.name}>
                    <td>
                      <code className="rd-table-key">{p.name}</code>
                      {p.required && <span className="rd-tag rd-tag--required">required</span>}
                    </td>
                    <td><code className="rd-table-type">{p.type}</code></td>
                    <td className="rd-table-desc">{p.description || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="rd-muted-note">This tool takes no parameters.</div>
        )}

        {schema && (
          <Collapsible className="rd-schema">
            <CollapsibleTrigger className="rd-schema-head">
              <ChevronRight className="rd-chevron" aria-hidden="true" />
              JSON schema
            </CollapsibleTrigger>
            <CollapsiblePanel className="rd-schema-body">
              <div className="rd-schema-actions">
                <CopyButton text={() => JSON.stringify(schema, null, 2)} label="Copy schema" />
              </div>
              <pre className="rd-code">{JSON.stringify(schema, null, 2)}</pre>
            </CollapsiblePanel>
          </Collapsible>
        )}
      </CollapsiblePanel>
    </Collapsible>
  )
}

export function ToolDefinitionList({ tools }: { tools: OAIToolDef[] }) {
  if (tools.length === 0) {
    return <div className="rd-empty-block">No tools were offered to the model in this request.</div>
  }
  return (
    <div className="rd-tooldefs">
      {tools.map((t, i) => (
        <ToolDefinitionCard key={`${t.function?.name ?? 'tool'}-${i}`} tool={t} />
      ))}
    </div>
  )
}
