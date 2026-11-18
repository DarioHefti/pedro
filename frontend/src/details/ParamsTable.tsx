import { formatValue } from './payload'

export function ParamsTable({ params, model }: { params: Record<string, unknown>; model: string }) {
  const entries: [string, unknown][] = [
    ...(model ? [['model', model] as [string, unknown]] : []),
    ...Object.entries(params),
  ]

  if (entries.length === 0) {
    return <div className="rd-empty-block">No request parameters were set.</div>
  }

  return (
    <div className="rd-table-wrap">
      <table className="rd-table rd-table--kv">
        <tbody>
          {entries.map(([k, v]) => (
            <tr key={k}>
              <th scope="row"><code className="rd-table-key">{k}</code></th>
              <td><code className="rd-table-value">{formatValue(v)}</code></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
