import clsx from 'clsx'
import {
  Brain, Search, AlertTriangle, Flag,
} from 'lucide-react'
import type {
  BenchmarkResult, CampaignEvent, CampaignState, EngineConfig,
} from '../api/client'
import { MonoLabel, Pill, type Tone } from './ui'

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

export function formatElapsed(ms: number): string {
  const total = Math.floor(ms / 1000)
  const m = Math.floor(total / 60)
  const s = total % 60
  return m > 0 ? `${m}m${String(s).padStart(2, '0')}s` : `${s}s`
}

const STATE_TONE: Record<CampaignState, Tone> = {
  pending: 'neutral',
  running: 'warning',
  completed: 'success',
  failed: 'error',
  cancelled: 'warning',
}

export function StateBadge({ state }: { state: CampaignState }) {
  return (
    <Pill tone={STATE_TONE[state]} pulse={state === 'running'}>
      {state}
    </Pill>
  )
}

// ── feed model ────────────────────────────────────────────────────────────────

export interface TrialCardData {
  engine: string
  gpuProfile: string
  config: EngineConfig
  hypothesis?: string
  result?: BenchmarkResult
  error?: string
  elapsedMs: number
  running: boolean
}

export type FeedItem =
  | { key: string; kind: 'trial'; trial: TrialCardData }
  | { key: string; kind: 'narration'; event: CampaignEvent }

/**
 * Folds the raw event stream into what a reader actually wants to see.
 *
 * The server emits a trial's start and its outcome as two separate events. A
 * card that appears as "running" and then fills in with its measurements is one
 * thing happening, not two, so they are merged — which is also what makes the
 * live view show work in flight rather than only after it lands.
 */
export function buildFeed(events: CampaignEvent[]): FeedItem[] {
  const items: FeedItem[] = []
  let openTrial: Extract<FeedItem, { kind: 'trial' }> | undefined

  for (const e of events) {
    switch (e.kind) {
      case 'trial': {
        const item: Extract<FeedItem, { kind: 'trial' }> = {
          key: `trial-${e.seq}`,
          kind: 'trial',
          trial: {
            engine: e.trial?.engine ?? 'unknown',
            gpuProfile: e.trial?.gpu_profile ?? '',
            config: e.trial?.engine_config ?? {},
            hypothesis: e.text,
            elapsedMs: e.elapsed_ms,
            running: true,
          },
        }
        items.push(item)
        openTrial = item
        break
      }
      case 'result':
      case 'failed': {
        if (openTrial) {
          openTrial.trial.running = false
          openTrial.trial.elapsedMs = e.elapsed_ms
          if (e.kind === 'failed') openTrial.trial.error = e.text
          else openTrial.trial.result = e.result
          openTrial = undefined
        } else {
          // An outcome with no start — shouldn't happen, but showing it as
          // narration is better than dropping a measurement on the floor.
          items.push({ key: `n-${e.seq}`, kind: 'narration', event: e })
        }
        break
      }
      default:
        items.push({ key: `n-${e.seq}`, kind: 'narration', event: e })
    }
  }

  return items
}

// ── trial card ────────────────────────────────────────────────────────────────

/**
 * A benchmark trial, in the mockup's execution-card shape: a status header with
 * badge and duration, the "command" being run, and its output beneath.
 *
 * A campaign's trials are the closest thing it has to shell commands, and the
 * card is the one place a reader looks for evidence rather than narration — so
 * the measurements sit on it, never behind a click.
 */
export function TrialCard({ trial }: { trial: TrialCardData }) {
  const { running, error } = trial
  const tone: Tone = running ? 'warning' : error ? 'error' : 'success'
  const accent = running ? 'text-tertiary' : error ? 'text-error' : 'text-secondary'

  return (
    <article className="flex flex-col w-full rounded-full bg-surface-container-low overflow-hidden">
      {running && (
        <div className="w-full h-1 bg-surface-container-high overflow-hidden">
          <div className="h-full bg-tertiary w-2/3 animate-pulse" />
        </div>
      )}

      <div className="flex flex-col p-space-md bg-surface-container gap-space-xs">
        <div className="flex items-center justify-between gap-space-xs">
          <div className="flex items-center gap-space-xs min-w-0">
            <Pill tone={tone} pulse={running}>
              {running ? 'running' : error ? 'failed' : 'measured'}
            </Pill>
            <span className={clsx('font-code-sm text-code-sm font-medium', accent)}>
              {formatElapsed(trial.elapsedMs)}
            </span>
          </div>
          {trial.gpuProfile && (
            <MonoLabel className="text-on-surface-variant shrink-0">{trial.gpuProfile}</MonoLabel>
          )}
        </div>

        <div className="flex items-start gap-1.5 font-code-md text-code-md text-on-surface break-all pt-1">
          <span className={clsx('select-none font-bold', accent)}>$</span>
          <span>
            benchmark <span className="text-primary font-medium">{trial.engine}</span>{' '}
            <span className="text-on-surface-variant">{configSummary(trial.config)}</span>
          </span>
        </div>
      </div>

      <div className="flex flex-col p-space-md bg-surface-container-lowest gap-space-sm">
        {trial.result && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-space-md">
            <Measurement label="throughput" value={trial.result.tok_per_s.toFixed(0)} unit="tok/s" tone="primary" />
            <Measurement label="p99 ttft" value={trial.result.ttft_p99_ms.toFixed(0)} unit="ms" />
            <Measurement label="cost" value={`$${trial.result.cost_per_mtok.toFixed(4)}`} unit="/Mtok" tone="success" />
            <Measurement
              label="errors"
              value={(trial.result.error_rate * 100).toFixed(1)}
              unit="%"
              tone={trial.result.error_rate > 0 ? 'error' : undefined}
            />
          </div>
        )}

        {error && (
          <p className="font-code-sm text-code-sm text-error whitespace-pre-wrap break-words">
            <span className="select-none opacity-70">stderr: </span>
            {error}
          </p>
        )}

        {running && (
          <div className="flex items-center gap-space-xs font-code-sm text-code-sm text-tertiary">
            <span className="animate-spin inline-block">◐</span>
            <span>starting engine, running requests…</span>
          </div>
        )}

        {trial.hypothesis && (
          <p className="font-code-sm text-code-sm text-outline whitespace-pre-wrap break-words">
            <span className="select-none">▸ </span>
            {trial.hypothesis}
          </p>
        )}
      </div>
    </article>
  )
}

function Measurement({
  label, value, unit, tone,
}: {
  label: string
  value: string
  unit: string
  tone?: 'primary' | 'success' | 'error'
}) {
  const color =
    tone === 'primary' ? 'text-primary'
      : tone === 'success' ? 'text-secondary'
        : tone === 'error' ? 'text-error'
          : 'text-on-surface'
  return (
    <div className="min-w-0">
      <MonoLabel className="text-on-surface-variant">{label}</MonoLabel>
      <div className={clsx('font-code-lg text-code-lg tabular-nums truncate', color)}>
        {value}
        <span className="text-on-surface-variant font-code-sm text-code-sm ml-0.5">{unit}</span>
      </div>
    </div>
  )
}

// ── narration ─────────────────────────────────────────────────────────────────

const NARRATION_STYLES: Record<string, { icon: typeof Brain; color: string }> = {
  plan: { icon: Brain, color: 'text-on-surface' },
  think: { icon: Brain, color: 'text-outline' },
  lookup: { icon: Search, color: 'text-on-surface-variant' },
  warn: { icon: AlertTriangle, color: 'text-tertiary' },
  done: { icon: Flag, color: 'text-secondary' },
}

/** Everything that is the planner talking rather than a measurement. */
export function NarrationRow({ event }: { event: CampaignEvent }) {
  const style = NARRATION_STYLES[event.kind] ?? NARRATION_STYLES.plan
  const Icon = style.icon
  const thinking = event.kind === 'think'

  return (
    <li className="flex gap-space-sm py-space-sm px-space-md">
      <Icon className={clsx('w-4 h-4 mt-0.5 shrink-0', style.color)} />
      <p
        className={clsx(
          'flex-1 min-w-0 font-body-md text-body-md whitespace-pre-wrap break-words',
          thinking ? 'text-outline italic' : 'text-on-surface-variant',
        )}
      >
        {event.text}
      </p>
      <span className="font-code-sm text-code-sm text-outline shrink-0 tabular-nums">
        {formatElapsed(event.elapsed_ms)}
      </span>
    </li>
  )
}
