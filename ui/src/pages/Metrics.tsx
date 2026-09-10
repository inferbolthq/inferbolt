import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer, Legend,
} from 'recharts'
import clsx from 'clsx'
import { metricsApi, type BenchmarkResult } from '../api/client'
import {
  Panel, PanelTitle, PageHeader, MonoLabel, EmptyState, ErrorText, LoadingText, chart,
} from '../components/ui'

const TIME_RANGES = ['1h', '6h', '24h', '7d'] as const
type TimeRange = typeof TIME_RANGES[number]

function MetricsLineChart({ data, dataKey, name, color }: {
  data: BenchmarkResult[]
  dataKey: keyof BenchmarkResult
  name: string
  color: string
}) {
  if (data.length === 0) return <EmptyState>No data yet</EmptyState>
  const chartData = data.map((r, i) => ({ index: i + 1, [dataKey]: r[dataKey] }))
  return (
    <ResponsiveContainer width="100%" height={200}>
      <LineChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" stroke={chart.grid} />
        <XAxis dataKey="index" tick={chart.tick} stroke={chart.axis} />
        <YAxis tick={chart.tick} stroke={chart.axis} />
        <Tooltip {...chart.tooltip} />
        <Legend {...chart.legend} />
        <Line type="monotone" dataKey={dataKey as string} name={name} stroke={color} dot={false} />
      </LineChart>
    </ResponsiveContainer>
  )
}

function LatencyChart({ data }: { data: BenchmarkResult[] }) {
  if (data.length === 0) return <EmptyState>No data yet</EmptyState>
  const chartData = data.map((r, i) => ({
    index: i + 1,
    ttft_p50: r.ttft_p50_ms,
    ttft_p99: r.ttft_p99_ms,
  }))
  return (
    <ResponsiveContainer width="100%" height={200}>
      <LineChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" stroke={chart.grid} />
        <XAxis dataKey="index" tick={chart.tick} stroke={chart.axis} />
        <YAxis tick={chart.tick} stroke={chart.axis} />
        <Tooltip {...chart.tooltip} />
        <Legend {...chart.legend} />
        <Line type="monotone" dataKey="ttft_p50" name="TTFT P50 (ms)" stroke={chart.primary} dot={false} />
        <Line type="monotone" dataKey="ttft_p99" name="TTFT P99 (ms)" stroke={chart.error} dot={false} />
      </LineChart>
    </ResponsiveContainer>
  )
}

function pct(arr: number[], p: number): number {
  if (arr.length === 0) return 0
  const sorted = [...arr].sort((a, b) => a - b)
  const idx = Math.ceil((p / 100) * sorted.length) - 1
  return sorted[Math.max(0, idx)]
}

function SummaryTable({ data }: { data: BenchmarkResult[] }) {
  if (data.length === 0) return null

  const toks = data.map(r => r.tok_per_s)
  const ttfts = data.map(r => r.ttft_p50_ms)
  const costs = data.map(r => r.cost_per_mtok)
  const avg = (arr: number[]) => arr.reduce((a, b) => a + b, 0) / arr.length

  const rows = [
    { metric: 'Tok/s', min: Math.min(...toks).toFixed(0), max: Math.max(...toks).toFixed(0), mean: avg(toks).toFixed(0), p95: pct(toks, 95).toFixed(0) },
    { metric: 'TTFT P50 (ms)', min: Math.min(...ttfts).toFixed(1), max: Math.max(...ttfts).toFixed(1), mean: avg(ttfts).toFixed(1), p95: pct(ttfts, 95).toFixed(1) },
    { metric: 'Cost/MTok ($)', min: Math.min(...costs).toFixed(4), max: Math.max(...costs).toFixed(4), mean: avg(costs).toFixed(4), p95: pct(costs, 95).toFixed(4) },
  ]

  return (
    <div className="overflow-x-auto -mx-space-lg -mb-space-lg">
      <table className="w-full text-left min-w-[520px]">
        <thead>
          <tr className="bg-surface-container-high text-on-surface-variant">
            {['Metric', 'Min', 'Max', 'Mean', 'P95'].map(h => (
              <th key={h} className="py-space-sm px-space-md font-label-mono text-label-mono uppercase font-medium">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={r.metric} className={i % 2 === 0 ? 'bg-surface-container-lowest' : 'bg-surface-container/50'}>
              <td className="py-space-sm px-space-md font-code-md text-code-md text-on-surface">{r.metric}</td>
              {[r.min, r.max, r.mean, r.p95].map((v, j) => (
                <td key={j} className="py-space-sm px-space-md font-code-md text-code-md text-on-surface-variant tabular-nums">
                  {v}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

const inputClasses =
  'bg-surface-container-lowest rounded-xl px-space-md py-space-sm font-code-md text-code-md text-on-surface ' +
  'placeholder:text-outline border-none outline-none focus:ring-1 focus:ring-primary'

export default function Metrics() {
  const [engine, setEngine] = useState('')
  const [model, setModel] = useState('')
  const [timeRange, setTimeRange] = useState<TimeRange>('24h')

  const enabled = engine.trim() !== '' && model.trim() !== ''

  const { data, isLoading, isError } = useQuery({
    queryKey: ['metrics', engine, model, timeRange],
    queryFn: () => metricsApi.query(engine, model, timeRange).then(r => r.data),
    enabled,
  })

  const results = useMemo(() => data ?? [], [data])

  return (
    <div className="flex flex-col gap-space-lg">
      <PageHeader title="Metrics Explorer" subtitle="Query historical benchmark results" />

      <Panel>
        <div className="flex flex-wrap gap-space-lg items-end">
          <div className="flex flex-col gap-space-xs">
            <MonoLabel className="text-on-surface-variant">Engine</MonoLabel>
            <input
              className={inputClasses}
              placeholder="vllm"
              value={engine}
              onChange={e => setEngine(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-space-xs flex-1 min-w-[240px]">
            <MonoLabel className="text-on-surface-variant">Model</MonoLabel>
            <input
              className={inputClasses}
              placeholder="meta-llama/Llama-3.1-8B"
              value={model}
              onChange={e => setModel(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-space-xs">
            <MonoLabel className="text-on-surface-variant">Time range</MonoLabel>
            <div className="flex gap-1">
              {TIME_RANGES.map(r => (
                <button
                  key={r}
                  onClick={() => setTimeRange(r)}
                  className={clsx(
                    'px-space-md py-space-sm rounded-xl font-label-mono text-label-mono uppercase transition-colors active:scale-95',
                    timeRange === r
                      ? 'bg-primary-container text-on-primary-container'
                      : 'bg-surface-container-high text-on-surface-variant hover:text-on-surface',
                  )}
                >
                  {r}
                </button>
              ))}
            </div>
          </div>
        </div>
      </Panel>

      {!enabled && (
        <p className="font-body-sm text-body-sm text-on-surface-variant">
          Enter engine and model to query metrics.
        </p>
      )}

      {isLoading && <LoadingText />}
      {isError && <ErrorText>Failed to load metrics.</ErrorText>}

      {enabled && !isLoading && (
        <>
          <Panel>
            <PanelTitle>Throughput (tok/s)</PanelTitle>
            <MetricsLineChart data={results} dataKey="tok_per_s" name="Tok/s" color={chart.primary} />
          </Panel>

          <Panel>
            <PanelTitle>Latency</PanelTitle>
            <LatencyChart data={results} />
          </Panel>

          <Panel>
            <PanelTitle>Summary statistics</PanelTitle>
            {results.length === 0
              ? <EmptyState>No data for this filter.</EmptyState>
              : <SummaryTable data={results} />}
          </Panel>
        </>
      )}
    </div>
  )
}
