import { useParams } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { campaignsApi, isCampaignTerminal, type Campaign } from '../api/client'
import { useCampaign, useCampaignEvents } from '../hooks/useCampaign'
import { StateBadge, EventRow, configSummary } from '../components/campaign'

export default function CampaignDetail() {
  const { campaignId } = useParams<{ campaignId: string }>()
  const queryClient = useQueryClient()

  const { data: campaign, isLoading, isError } = useCampaign(campaignId)
  const { events } = useCampaignEvents(campaignId)

  const cancel = useMutation({
    mutationFn: () => campaignsApi.cancel(campaignId!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['campaign', campaignId] }),
  })

  if (isLoading) return <div className="text-gray-400 text-sm">Loading…</div>
  if (isError || !campaign) return <div className="text-red-500 text-sm">Campaign not found.</div>

  const running = !isCampaignTerminal(campaign.state)
  const trialsRun = campaign.trials?.filter(t => !t.reused).length ?? countTrials(events)

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold text-gray-900">{campaign.goal}</h1>
            <StateBadge state={campaign.state} />
          </div>
          <p className="text-sm text-gray-500 mt-1">
            {campaign.model} on {campaign.gpu_profile} · {campaign.engines.join(', ')} ·{' '}
            <span className="font-mono text-xs">{campaign.id.slice(0, 8)}</span>
          </p>
        </div>
        {running && (
          <button
            onClick={() => cancel.mutate()}
            disabled={cancel.isPending}
            className="shrink-0 text-sm px-3 py-1.5 rounded-lg border border-gray-200 text-gray-600 hover:bg-gray-50 hover:text-red-600 hover:border-red-200 transition-colors disabled:opacity-50"
          >
            {cancel.isPending ? 'Cancelling…' : 'Cancel'}
          </button>
        )}
      </div>

      {campaign.error_msg && (
        <div className="bg-red-50 border border-red-200 rounded-xl p-4 text-sm text-red-700">
          {campaign.error_msg}
        </div>
      )}

      {/* Budget and workload — what the campaign was allowed to do */}
      <div className="bg-white rounded-xl border border-gray-200 p-5 grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <Fact label="Trials" value={`${trialsRun} of ${campaign.budget.max_trials}`} />
        <Fact label="Time budget" value={campaign.budget.max_duration} />
        <Fact
          label="Workload"
          value={`${campaign.workload.concurrency} concurrent, ${campaign.workload.prompt_tokens}/${campaign.workload.output_tokens} tokens`}
        />
        <Fact label="Planner" value={campaign.planner_model} />
      </div>

      {/* Recommendation */}
      {campaign.recommendation && <Recommendation campaign={campaign} />}

      {/* Live feed */}
      <div className="bg-white rounded-xl border border-gray-200 p-5">
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-sm font-semibold text-gray-700">Progress</h2>
          {running && (
            <span className="flex items-center gap-2 text-xs text-blue-600">
              <span className="w-3 h-3 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
              live
            </span>
          )}
        </div>

        {events.length === 0 ? (
          <p className="text-sm text-gray-400 py-4">
            {running ? 'Waiting for the campaign to start…' : 'No steps were recorded.'}
          </p>
        ) : (
          <ul className="divide-y divide-gray-50">
            {events.map(e => <EventRow key={e.seq} event={e} />)}
          </ul>
        )}
      </div>
    </div>
  )
}

function Recommendation({ campaign }: { campaign: Campaign }) {
  const rec = campaign.recommendation!
  const m = rec.measured

  return (
    <div className="bg-blue-50 border border-blue-200 rounded-xl p-5">
      <div className="flex items-baseline justify-between gap-4 flex-wrap">
        <h2 className="text-sm font-semibold text-blue-800">
          Recommendation: {rec.engine}{' '}
          <span className="font-mono font-normal">{configSummary(rec.engine_config)}</span>
        </h2>
        <span className="text-xs text-blue-600">confidence: {rec.confidence}</span>
      </div>

      {rec.fallback && (
        <p className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded px-2 py-1 mt-2 inline-block">
          Selected mechanically — the planner did not conclude within its budget.
        </p>
      )}

      {m && (
        <div className="flex gap-6 flex-wrap mt-3 text-sm">
          <Headline label="throughput" value={`${m.tok_per_s.toFixed(0)} tok/s`} />
          <Headline label="p99 TTFT" value={`${m.ttft_p99_ms.toFixed(0)} ms`} />
          <Headline label="cost" value={`$${m.cost_per_mtok.toFixed(4)} / Mtok`} />
        </div>
      )}

      <p className="text-sm text-blue-900 mt-3 whitespace-pre-wrap">{rec.reasoning}</p>

      {rec.runner_up && (
        <p className="text-sm text-blue-700 mt-2">
          <span className="font-medium">Runner-up:</span> {rec.runner_up}
        </p>
      )}
      {rec.caveats && (
        <p className="text-sm text-blue-700 mt-2">
          <span className="font-medium">Caveats:</span> {rec.caveats}
        </p>
      )}

      {campaign.usage && (
        <p className="text-xs text-blue-500 mt-4">
          Planning: {campaign.usage.turns} turns,{' '}
          {(campaign.usage.input_tokens + campaign.usage.output_tokens + campaign.usage.cache_read_tokens).toLocaleString()} tokens.
        </p>
      )}
    </div>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span className="text-gray-500">{label}</span>
      <br />
      <span className="font-medium text-gray-800">{value}</span>
    </div>
  )
}

function Headline({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs text-blue-500">{label}</div>
      <div className="text-lg font-semibold text-blue-900 tabular-nums">{value}</div>
    </div>
  )
}

/** Before the campaign finishes there is no trials array yet, so count the feed. */
function countTrials(events: { kind: string }[]): number {
  return events.filter(e => e.kind === 'trial').length
}
