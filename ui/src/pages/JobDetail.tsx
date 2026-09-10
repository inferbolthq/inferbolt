import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  ScatterChart, Scatter, Legend,
} from 'recharts'
import { jobsApi, type BenchmarkResult, type Job } from '../api/client'
import {
  Panel, PanelTitle, MonoLabel, Pill, Fact, EmptyState, ErrorText, LoadingText, Spinner, chart,
  type Tone,
} from '../components/ui'

const TERMINAL = new Set(['completed', 'failed', 'cancelled'])

function isTerminal(state: string) {
  return TERMINAL.has(state)
}

const STATE_TONE: Record<string, Tone> = {
  completed: 'success',
  running: 'warning',
  collecting: 'warning',
  analyzing: 'warning',
  pending: 'neutral',
  failed: 'error',
  cancelled: 'neutral',
}

// ── best-value highlighting ───────────────────────────────────────────────────

function bestIdx(results: BenchmarkResult[], key: keyof BenchmarkResult, higherBetter: boolean): number {
  if (results.length === 0) return 0
  let best = 0
  for (let i = 1; i < results.length; i++) {
    const v = results[i][key] as number
    const bv = results[best][key] as number
    if (higherBetter ? v > bv : v < bv) best = i
  }
  return best
}

function ResultsTable({ results }: { results: BenchmarkResult[] }) {
  if (results.length === 0) return <EmptyState>No results yet.</EmptyState>

  const cols: Array<{ label: string; key: keyof BenchmarkResult; higherBetter: boolean; fmt: (v: number) => string }> = [
    { label: 'TTFT P50', key: 'ttft_p50_ms', higherBetter: false, fmt: v => `${v.toFixed(1)} ms` },
    { label: 'TTFT P99', key: 'ttft_p99_ms', higherBetter: false, fmt: v => `${v.toFixed(1)} ms` },
    { label: 'Tok/s', key: 'tok_per_s', higherBetter: true, fmt: v => v.toFixed(0) },
    { label: 'GPU Mem', key: 'gpu_mem_mb', higherBetter: false, fmt: v => `${v} MB` },
    { label: 'Cost/MTok', key: 'cost_per_mtok', higherBetter: false, fmt: v => `$${v.toFixed(4)}` },
    { label: 'KV Hit', key: 'kv_cache_hit', higherBetter: true, fmt: v => `${(v * 100).toFixed(1)}%` },
  ]

  const bests = cols.map(c => bestIdx(results, c.key, c.higherBetter))

  return (
    <div className="overflow-x-auto -mx-space-lg -mb-space-lg">
      <table className="w-full text-left min-w-[640px]">
        <thead>
          <tr className="bg-surface-container-high text-on-surface-variant">
            <th className="py-space-sm px-space-md font-label-mono text-label-mono uppercase font-medium">Engine</th>
            {cols.map(c => (
              <th key={c.key} className="py-space-sm px-space-md font-label-mono text-label-mono uppercase font-medium">
                {c.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {results.map((r, ri) => (
            <tr
              key={r.engine}
              className={ri % 2 === 0
                ? 'bg-surface-container-lowest'
                : 'bg-surface-container/50'}
            >
              <td className="py-space-sm px-space-md font-code-md text-code-md text-primary font-medium">{r.engine}</td>
              {cols.map((c, ci) => (
                <td
                  key={c.key}
                  className={`py-space-sm px-space-md font-code-md text-code-md tabular-nums ${
                    bests[ci] === ri ? 'text-secondary font-medium' : 'text-on-surface'
                  }`}
                >
                  {c.fmt(r[c.key] as number)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ── charts ────────────────────────────────────────────────────────────────────

function CostComparisonChart({ results }: { results: BenchmarkResult[] }) {
  if (results.length === 0) return <EmptyState>No data yet</EmptyState>
  const data = results.map(r => ({ engine: r.engine, cost: r.cost_per_mtok }))
  return (
    <ResponsiveContainer width="100%" height={180}>
      <BarChart data={data}>
        <CartesianGrid strokeDasharray="3 3" stroke={chart.grid} />
        <XAxis dataKey="engine" tick={chart.tick} stroke={chart.axis} />
        <YAxis tick={chart.tick} stroke={chart.axis} />
        <Tooltip {...chart.tooltip} formatter={(v: number) => `$${v.toFixed(4)}`} cursor={{ fill: '#ffffff08' }} />
        <Bar dataKey="cost" name="Cost/MTok ($)" fill={chart.primary} radius={[4, 4, 0, 0]} />
      </BarChart>
    </ResponsiveContainer>
  )
}

function ThroughputVsLatencyChart({ results }: { results: BenchmarkResult[] }) {
  if (results.length === 0) return <EmptyState>No data yet</EmptyState>
  const data = results.map(r => ({ tok_per_s: r.tok_per_s, ttft_p50_ms: r.ttft_p50_ms, engine: r.engine }))
  return (
    <ResponsiveContainer width="100%" height={180}>
      <ScatterChart>
        <CartesianGrid strokeDasharray="3 3" stroke={chart.grid} />
        <XAxis type="number" dataKey="tok_per_s" name="Tok/s" tick={chart.tick} stroke={chart.axis} />
        <YAxis type="number" dataKey="ttft_p50_ms" name="TTFT P50 (ms)" tick={chart.tick} stroke={chart.axis} />
        <Tooltip {...chart.tooltip} cursor={{ strokeDasharray: '3 3' }} />
        <Legend {...chart.legend} />
        <Scatter data={data} fill={chart.primary} name="Engine" />
      </ScatterChart>
    </ResponsiveContainer>
  )
}

function RecommendationCard({ results }: { results: BenchmarkResult[] }) {
  if (results.length === 0) return null
  const best = results.reduce((a, b) => (a.tok_per_s > b.tok_per_s ? a : b))
  return (
    <Panel className="bg-surface-container">
      <MonoLabel className="text-secondary">Best measured</MonoLabel>
      <p className="font-body-md text-body-md text-on-surface mt-space-xs">
        <span className="text-primary font-medium">{best.engine}</span> — highest throughput at{' '}
        <span className="tabular-nums">{best.tok_per_s.toFixed(0)} tok/s</span> (
        <span className="tabular-nums">${best.cost_per_mtok.toFixed(4)}</span>/MTok)
      </p>
      {Object.keys(best.config).length > 0 && (
        <pre className="mt-space-md p-space-md rounded-xl bg-surface-container-lowest overflow-x-auto font-code-sm text-code-sm text-on-surface-variant">
          {JSON.stringify(best.config, null, 2)}
        </pre>
      )}
    </Panel>
  )
}

// ── JobDetail page ────────────────────────────────────────────────────────────

export default function JobDetail() {
  const { jobId } = useParams<{ jobId: string }>()

  const { data: job, isLoading: jobLoading, isError: jobError } = useQuery<Job>({
    queryKey: ['job', jobId],
    queryFn: () => jobsApi.get(jobId!).then(r => r.data),
    refetchInterval: query => {
      const j = query.state.data
      return j && isTerminal(j.state) ? false : 5_000
    },
    enabled: !!jobId,
  })

  const { data: results } = useQuery<BenchmarkResult[]>({
    queryKey: ['results', jobId],
    queryFn: () => jobsApi.getResults(jobId!).then(r => r.data),
    enabled: !!job && isTerminal(job.state),
    refetchOnWindowFocus: false,
  })

  if (jobLoading) return <LoadingText />
  if (jobError || !job) return <ErrorText>Job not found.</ErrorText>

  const safeResults = results ?? []

  return (
    <div className="flex flex-col gap-space-lg">
      <div className="flex items-center gap-space-sm flex-wrap">
        <h1 className="font-code-lg text-code-lg text-primary">{job.id.slice(0, 8)}</h1>
        <Pill tone={STATE_TONE[job.state] ?? 'neutral'} pulse={!isTerminal(job.state)}>
          {job.state}
        </Pill>
        <span className="font-body-sm text-body-sm text-on-surface-variant">{job.model}</span>
      </div>

      <Panel>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-space-lg">
          <Fact label="engines" value={job.engines.join(', ')} mono />
          <Fact label="gpu profile" value={job.gpu_profile} mono />
          <Fact label="created" value={new Date(job.created_at).toLocaleString()} mono />
          <Fact label="updated" value={new Date(job.updated_at).toLocaleString()} mono />
        </div>
        {job.error_msg && (
          <div className="mt-space-lg p-space-md rounded-xl bg-error/10 font-code-sm text-code-sm text-error break-words">
            <span className="select-none opacity-70">stderr: </span>
            {job.error_msg}
          </div>
        )}
      </Panel>

      {!isTerminal(job.state) && (
        <Panel className="bg-surface-container">
          <div className="flex items-center gap-space-sm">
            <Spinner className="w-4 h-4 border-tertiary border-t-transparent" />
            <span className="font-code-md text-code-md text-tertiary">Benchmark in progress…</span>
          </div>
        </Panel>
      )}

      {job.state === 'completed' && (
        <>
          <Panel>
            <PanelTitle>Results</PanelTitle>
            <ResultsTable results={safeResults} />
          </Panel>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-space-lg">
            <Panel>
              <PanelTitle>Cost per MTok</PanelTitle>
              <CostComparisonChart results={safeResults} />
            </Panel>
            <Panel>
              <PanelTitle>Throughput vs latency</PanelTitle>
              <ThroughputVsLatencyChart results={safeResults} />
            </Panel>
          </div>

          <RecommendationCard results={safeResults} />
        </>
      )}
    </div>
  )
}
