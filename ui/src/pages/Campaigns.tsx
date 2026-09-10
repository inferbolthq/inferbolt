import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { campaignsApi, type Campaign } from '../api/client'
import { StateBadge, configSummary } from '../components/campaign'
import {
  Panel, PageHeader, MonoLabel, EmptyState, ErrorText, LoadingText,
} from '../components/ui'

export default function Campaigns() {
  const { data, isLoading, isError } = useQuery<Campaign[]>({
    queryKey: ['campaigns'],
    queryFn: () => campaignsApi.list(undefined, 50).then(r => r.data.campaigns),
    // Cheap, and it keeps a running campaign's row moving without a refresh.
    refetchInterval: 5_000,
  })

  if (isLoading) return <LoadingText />
  if (isError) return <ErrorText>Could not load campaigns.</ErrorText>

  const campaigns = data ?? []

  return (
    <div className="flex flex-col gap-space-lg">
      <PageHeader
        title="Campaigns"
        subtitle="Goal-directed optimization runs. The agent plans the benchmarks; every number is measured."
        right={<MonoLabel className="text-outline">{campaigns.length} total</MonoLabel>}
      />

      {campaigns.length === 0 ? (
        <Panel>
          <EmptyState>
            <p>No campaigns yet.</p>
            <p className="mt-space-sm text-outline">Start one from the CLI:</p>
            <code className="inline-block mt-space-xs px-space-sm py-space-xs rounded-xl bg-surface-container-lowest font-code-sm text-code-sm text-primary">
              inferbolt agent "cheapest engine under 200ms p99 TTFT" --model … --gpu …
            </code>
          </EmptyState>
        </Panel>
      ) : (
        <Panel padded={false}>
          <div className="overflow-x-auto">
            <table className="w-full text-left min-w-[720px]">
              <thead>
                <tr className="bg-surface-container-high text-on-surface-variant">
                  {['Goal', 'Model', 'State', 'Recommended', 'Started'].map(h => (
                    <th key={h} className="py-space-sm px-space-md font-label-mono text-label-mono uppercase font-medium">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {campaigns.map((c, i) => (
                  <tr
                    key={c.id}
                    className={clsxRow(i)}
                  >
                    <td className="py-space-sm px-space-md">
                      <Link
                        to={`/campaigns/${c.id}`}
                        className="font-body-md text-body-md text-on-surface hover:text-primary transition-colors"
                      >
                        {c.goal}
                      </Link>
                      <div className="font-code-sm text-code-sm text-outline mt-0.5">{c.id.slice(0, 8)}</div>
                    </td>
                    <td className="py-space-sm px-space-md">
                      <div className="font-code-md text-code-md text-primary truncate max-w-[220px]">{c.model}</div>
                      <div className="font-code-sm text-code-sm text-on-surface-variant mt-0.5">{c.gpu_profile}</div>
                    </td>
                    <td className="py-space-sm px-space-md">
                      <StateBadge state={c.state} />
                    </td>
                    <td className="py-space-sm px-space-md font-code-md text-code-md">
                      {c.recommendation ? (
                        <>
                          <span className="text-secondary">{c.recommendation.engine}</span>{' '}
                          <span className="text-on-surface-variant">
                            {configSummary(c.recommendation.engine_config)}
                          </span>
                        </>
                      ) : (
                        <span className="text-outline">—</span>
                      )}
                    </td>
                    <td className="py-space-sm px-space-md font-code-sm text-code-sm text-on-surface-variant whitespace-nowrap">
                      {new Date(c.created_at).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      )}
    </div>
  )
}

/** Zebra striping, as the mockup's table does it — surface tiers, not borders. */
function clsxRow(index: number): string {
  return index % 2 === 0
    ? 'bg-surface-container-lowest hover:bg-surface-container-low transition-colors'
    : 'bg-surface-container/50 hover:bg-surface-container transition-colors'
}
