import { useEffect, useState } from 'react'
import { Activity, ArrowRight, Coins } from 'lucide-react'
import { Button } from '../ui/button'
import type { SettingsServices } from './settingsModel'

interface UsagePanelProps {
  services: Pick<SettingsServices, 'getLifetimeStats'>
  onViewDetails: () => void
}

export function UsagePanel({ services, onViewDetails }: UsagePanelProps) {
  const [stats, setStats] = useState<{ totalRequests: number; totalTokens: number } | null>(null)

  useEffect(() => {
    let cancelled = false
    services
      .getLifetimeStats()
      .then(s => {
        if (!cancelled) setStats({ totalRequests: s?.totalRequests ?? 0, totalTokens: s?.totalTokens ?? 0 })
      })
      .catch(() => {
        if (!cancelled) setStats({ totalRequests: 0, totalTokens: 0 })
      })
    return () => {
      cancelled = true
    }
  }, [services])

  const perRequest = stats && stats.totalRequests > 0 ? Math.round(stats.totalTokens / stats.totalRequests) : 0

  return (
    <div className="settings-stack">
      <div className="stat-cards">
        <div className="stat-card">
          <span className="stat-card-icon" aria-hidden="true">
            <Activity />
          </span>
          <span className="stat-card-label">LLM requests</span>
          {stats ? <strong className="stat-card-value">{stats.totalRequests.toLocaleString()}</strong> : <span className="skeleton stat-card-skeleton" />}
          <span className="stat-card-hint">Across every chat, since install</span>
        </div>
        <div className="stat-card">
          <span className="stat-card-icon" aria-hidden="true">
            <Coins />
          </span>
          <span className="stat-card-label">Estimated tokens</span>
          {stats ? <strong className="stat-card-value">{stats.totalTokens.toLocaleString()}</strong> : <span className="skeleton stat-card-skeleton" />}
          <span className="stat-card-hint">{perRequest ? `≈ ${perRequest.toLocaleString()} per request` : 'Prompt and response combined'}</span>
        </div>
      </div>

      <div className="settings-card settings-callout">
        <div>
          <strong>Inspect what Pedro sends</strong>
          <span className="settings-muted">See the full payload of the last request for each chat — messages, tools and parameters.</span>
        </div>
        <Button variant="secondary" size="sm" onClick={onViewDetails}>
          Request details
          <ArrowRight aria-hidden="true" />
        </Button>
      </div>
    </div>
  )
}
