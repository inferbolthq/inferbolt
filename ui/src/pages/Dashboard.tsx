import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'
import { jobsApi, type Job } from '../api/client'
import {
  Panel, PanelTitle, PageHeader, MonoLabel, Pill, EmptyState, ErrorText, LoadingText, chart,
  type Tone,
} from '../components/ui'

// ── stat card ─────────────────────────────────────────────────────────────────

function StatCard({ label, value, sub, tone }: {
  label: string
  value: string | number
  sub?: string
  tone?: 'primary' | 'warning' | 'success'
}) {
  const color =
    tone === 'primary' ? 'text-primary'
      : tone === 'warning' ? 'text-tertiary'
        : tone === 'success' ? 'text-secondary'
          : 'text-on-surface'
  return (
    <Panel>
      <MonoLabel className="text-on-surface-variant">{label}</MonoLabel>
      <p className={`font-headline-xl text-headline-xl tabular-nums mt-1 ${color}`}>{value}</p>
      {sub && <p className="font-code-sm text-code-sm text-outline mt-1">{sub}</p>}
    </Panel>
  )
}

// ── recent jobs ───────────────────────────────────────────────────────────────

const STATE_TONE: Record<string, Tone> = {
  completed: 'success',
  running: 'warning',
  collecting: 'warning',
  analyzing: 'warning',
  pending: 'neutral',
  failed: 'error',
  cancelled: 'neutral',
}

function RecentJobsTable({ jobs }: { jobs: Job[] }) {
  if (jobs.length === 0) {
    return <EmptyState>No jobs yet. Run your first benchmark with the CLI.</EmptyState>
  }
  return (
    <div className="overflow-x-auto -mx-space-lg -mb-space-lg">
      <table className="w-full text-left min-w-[640px]">
        <thead>
          <tr className="bg-surface-container-high text-on-surface-variant">
            {['Job', 'Model', 'Engines', 'State', 'Created'].map(h => (
              <th key={h} className="py-space-sm px-space-md font-label-mono text-label-mono uppercase font-medium">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {jobs.slice(0, 10).map((job, i) => (
            <tr
              key={job.id}
              className={i % 2 === 0
                ? 'bg-surface-container-lowest hover:bg-surface-container-low transition-colors'
                : 'bg-surface-container/50 hover:bg-surface-container transition-colors'}
            >
              <td className="py-space-sm px-space-md">
                <Link to={`/jobs/${job.id}`} className="font-code-md text-code-md text-primary hover:underline">
                  {job.id.slice(0, 8)}
                </Link>
              </td>
              <td className="py-space-sm px-space-md font-code-md text-code-md text-on-surface truncate max-w-[220px]">
                {job.model}
              </td>
              <td className="py-space-sm px-space-md font-code-sm text-code-sm text-on-surface-variant">
                {job.engines.join(', ')}
              </td>
              <td className="py-space-sm px-space-md">
                <Pill tone={STATE_TONE[job.state] ?? 'neutral'} pulse={job.state === 'running'}>
                  {job.state}
                </Pill>
              </td>
              <td className="py-space-sm px-space-md font-code-sm text-code-sm text-outline whitespace-nowrap">
                {new Date(job.created_at).toLocaleString()}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ── throughput chart ──────────────────────────────────────────────────────────

interface ChartPoint {
  time: string
  tok_per_s: number
  engine: string
}

function ThroughputChart({ data }: { data: ChartPoint[] }) {
  if (data.length === 0) return <EmptyState>No data yet</EmptyState>
  return (
    <ResponsiveContainer width="100%" height={200}>
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" stroke={chart.grid} />
        <XAxis dataKey="time" tick={chart.tick} stroke={chart.axis} />
        <YAxis tick={chart.tick} stroke={chart.axis} />
        <Tooltip {...chart.tooltip} />
        <Legend {...chart.legend} />
        <Line type="monotone" dataKey="tok_per_s" name="Tok/s" stroke={chart.primary} dot={false} />
      </LineChart>
    </ResponsiveContainer>
  )
}

// ── Dashboard page ────────────────────────────────────────────────────────────

export default function Dashboard() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['jobs', 'dashboard'],
    queryFn: () => jobsApi.list(undefined, 100).then(r => r.data),
    refetchInterval: 10_000,
  })

  if (isLoading) return <LoadingText />
  if (isError) return <ErrorText>Failed to load jobs.</ErrorText>

  const jobs = data?.jobs ?? []
  const today = new Date().toDateString()

  const totalJobs = jobs.length
  const runningJobs = jobs.filter(
    j => j.state === 'running' || j.state === 'collecting' || j.state === 'analyzing',
  ).length
  const completedToday = jobs.filter(
    j => j.state === 'completed' && new Date(j.created_at).toDateString() === today,
  ).length

  const chartData: ChartPoint[] = jobs
    .filter(j => j.state === 'completed')
    .sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime())
    .slice(-20)
    .map(j => ({
      time: new Date(j.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      tok_per_s: 0, // would need a results fetch — placeholder
      engine: j.engines[0] ?? '',
    }))

  return (
    <div className="flex flex-col gap-space-lg">
      <PageHeader title="Dashboard" subtitle="InferBolt benchmark activity" />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-space-lg">
        <StatCard label="total jobs" value={totalJobs} />
        <StatCard label="running" value={runningJobs} tone={runningJobs > 0 ? 'warning' : undefined} />
        <StatCard label="completed today" value={completedToday} tone="success" />
        <StatCard label="avg tok/s" value="—" sub="select a job for details" />
      </div>

      <Panel>
        <PanelTitle>Recent jobs</PanelTitle>
        <RecentJobsTable jobs={jobs} />
      </Panel>

      <Panel>
        <PanelTitle>Throughput (last 24h)</PanelTitle>
        <ThroughputChart data={chartData} />
      </Panel>
    </div>
  )
}
