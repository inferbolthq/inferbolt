import clsx from 'clsx'
import {
  Brain, Search, FlaskConical, CheckCircle2, XCircle, AlertTriangle, Flag,
} from 'lucide-react'
import type { CampaignEvent, CampaignEventKind, CampaignState, EngineConfig } from '../api/client'

/** Renders an engine config the way the CLI does, so the two agree. */
export function configSummary(c: EngineConfig | undefined): string {
  if (!c) return 'engine defaults'
  const parts: string[] = []
  if (c.quantization) parts.push(c.quantization)
  if (c.tensor_parallel) parts.push(`TP${c.tensor_parallel}`)
  if (c.max_batch_size) parts.push(`bs${c.max_batch_size}`)
  if (c.max_model_len) parts.push(`len${c.max_model_len}`)
  return parts.length > 0 ? parts.join(' ') : 'engine defaults'
}

const STATE_STYLES: Record<CampaignState, string> = {
  pending: 'bg-gray-100 text-gray-600',
  running: 'bg-blue-50 text-blue-700',
  completed: 'bg-green-50 text-green-700',
  failed: 'bg-red-50 text-red-700',
  cancelled: 'bg-amber-50 text-amber-700',
}

export function StateBadge({ state }: { state: CampaignState }) {
  return (
    <span className={clsx('inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium', STATE_STYLES[state])}>
      {state === 'running' && (
        <span className="w-1.5 h-1.5 rounded-full bg-blue-500 animate-pulse" />
      )}
      {state}
    </span>
  )
}

const EVENT_STYLES: Record<CampaignEventKind, { icon: typeof Brain; color: string; label: string }> = {
  plan:   { icon: Brain,         color: 'text-gray-700',   label: 'plan' },
  think:  { icon: Brain,         color: 'text-gray-400',   label: 'thinking' },
  lookup: { icon: Search,        color: 'text-gray-400',   label: 'lookup' },
  trial:  { icon: FlaskConical,  color: 'text-blue-600',   label: 'benchmark' },
  result: { icon: CheckCircle2,  color: 'text-green-600',  label: 'measured' },
  failed: { icon: XCircle,       color: 'text-red-600',    label: 'failed' },
  warn:   { icon: AlertTriangle, color: 'text-amber-600',  label: 'warning' },
  done:   { icon: Flag,          color: 'text-green-700',  label: 'complete' },
}

function formatElapsed(ms: number): string {
  const total = Math.floor(ms / 1000)
  const m = Math.floor(total / 60)
  const s = total % 60
  return m > 0 ? `${m}m${String(s).padStart(2, '0')}s` : `${s}s`
}

/**
 * One step of a campaign. A measured trial shows its numbers inline — the
 * measurement is the point, and burying it behind a click would hide the only
 * thing in the feed that is evidence rather than narration.
 */
export function EventRow({ event }: { event: CampaignEvent }) {
  const style = EVENT_STYLES[event.kind] ?? EVENT_STYLES.plan
  const Icon = style.icon

  return (
    <li className="flex gap-3 py-2.5">
      <Icon className={clsx('w-4 h-4 mt-0.5 shrink-0', style.color)} />
      <div className="min-w-0 flex-1">
        {event.trial && (
          <div className="flex items-baseline gap-2 flex-wrap">
            <span className="font-medium text-gray-800">{event.trial.engine}</span>
            <span className="font-mono text-xs text-gray-500">{configSummary(event.trial.engine_config)}</span>
          </div>
        )}

        {event.result && (
          <div className="flex gap-4 flex-wrap mt-1 text-xs">
            <Metric label="throughput" value={`${event.result.tok_per_s.toFixed(0)} tok/s`} />
            <Metric label="p99 TTFT" value={`${event.result.ttft_p99_ms.toFixed(0)} ms`} />
            <Metric label="cost" value={`$${event.result.cost_per_mtok.toFixed(4)}/Mtok`} />
            {event.result.error_rate > 0 && (
              <Metric label="errors" value={`${(event.result.error_rate * 100).toFixed(1)}%`} warn />
            )}
          </div>
        )}

        {event.text && (
          <p className={clsx(
            'text-sm mt-0.5 whitespace-pre-wrap break-words',
            event.kind === 'think' ? 'text-gray-400 italic' : 'text-gray-600',
          )}>
            {event.text}
          </p>
        )}
      </div>

      <span className="text-xs text-gray-300 shrink-0 tabular-nums">
        {formatElapsed(event.elapsed_ms)}
      </span>
    </li>
  )
}

function Metric({ label, value, warn }: { label: string; value: string; warn?: boolean }) {
  return (
    <span className={clsx('tabular-nums', warn ? 'text-red-600' : 'text-gray-700')}>
      <span className="text-gray-400">{label} </span>
      <span className="font-medium">{value}</span>
    </span>
  )
}
