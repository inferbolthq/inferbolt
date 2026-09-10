import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { campaignsApi, type Campaign, type CampaignState } from '../api/client'
import { StateBadge, configSummary } from '../components/campaign'

export default function Campaigns() {
  const { data, isLoading, isError } = useQuery<Campaign[]>({
    queryKey: ['campaigns'],
    queryFn: () => campaignsApi.list(undefined, 50).then(r => r.data.campaigns),
    // Cheap, and it keeps a running campaign's row moving without a refresh.
    refetchInterval: 5_000,
  })

  if (isLoading) return <div className="text-gray-400 text-sm">Loading…</div>
  if (isError) return <div className="text-red-500 text-sm">Could not load campaigns.</div>

  const campaigns = data ?? []

  if (campaigns.length === 0) {
    return (
      <div className="space-y-6">
        <Header />
        <div className="bg-white rounded-xl border border-gray-200 p-10 text-center">
          <p className="text-sm text-gray-500">No campaigns yet.</p>
          <p className="text-sm text-gray-400 mt-2">
            Start one from the CLI:{' '}
            <code className="font-mono text-xs bg-gray-50 border border-gray-200 rounded px-1.5 py-0.5">
              inferbolt agent "cheapest engine under 200ms p99 TTFT" --model … --gpu …
            </code>
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <Header />
      <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-gray-500 border-b border-gray-100">
              <th className="py-3 px-5 font-medium">Goal</th>
              <th className="py-3 px-5 font-medium">Model</th>
              <th className="py-3 px-5 font-medium">State</th>
              <th className="py-3 px-5 font-medium">Recommended</th>
              <th className="py-3 px-5 font-medium">Started</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-50">
            {campaigns.map(c => (
              <tr key={c.id} className="hover:bg-gray-50 transition-colors">
                <td className="py-3 px-5">
                  <Link to={`/campaigns/${c.id}`} className="text-gray-900 hover:text-blue-600 font-medium">
                    {c.goal}
                  </Link>
                  <div className="font-mono text-xs text-gray-400 mt-0.5">{c.id.slice(0, 8)}</div>
                </td>
                <td className="py-3 px-5 text-gray-600">
                  {c.model}
                  <div className="text-xs text-gray-400 mt-0.5">{c.gpu_profile}</div>
                </td>
                <td className="py-3 px-5"><StateBadge state={c.state} /></td>
                <td className="py-3 px-5">
                  {c.recommendation ? (
                    <span className="text-gray-700">
                      {c.recommendation.engine}{' '}
                      <span className="text-gray-400">{configSummary(c.recommendation.engine_config)}</span>
                    </span>
                  ) : (
                    <span className="text-gray-300">—</span>
                  )}
                </td>
                <td className="py-3 px-5 text-gray-500">
                  {new Date(c.created_at).toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function Header() {
  return (
    <div>
      <h1 className="text-xl font-semibold text-gray-900">Campaigns</h1>
      <p className="text-sm text-gray-500 mt-1">
        Goal-directed optimization runs. The agent plans the benchmarks; every number is measured.
      </p>
    </div>
  )
}

export type { CampaignState }
