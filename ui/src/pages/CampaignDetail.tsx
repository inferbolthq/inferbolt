import { useMemo } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { StopCircle } from 'lucide-react'
import { campaignsApi, isCampaignTerminal, type Campaign } from '../api/client'
import { useCampaign, useCampaignEvents } from '../hooks/useCampaign'
import {
  StateBadge, TrialCard, NarrationRow, buildFeed, configSummary,
} from '../components/campaign'
import {
  Panel, MonoLabel, Pill, Fact, EmptyState, ErrorText, LoadingText, Ping,
} from '../components/ui'

export default function CampaignDetail() {
  const { campaignId } = useParams<{ campaignId: string }>()
  const queryClient = useQueryClient()

  const { data: campaign, isLoading, isError } = useCampaign(campaignId)
  const { events } = useCampaignEvents(campaignId)

  const feed = useMemo(() => buildFeed(events), [events])

  const cancel = useMutation({
    mutationFn: () => campaignsApi.cancel(campaignId!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['campaign', campaignId] }),
  })

  if (isLoading) return <LoadingText />
  if (isError || !campaign) return <ErrorText>Campaign not found.</ErrorText>

  const running = !isCampaignTerminal(campaign.state)
  const trialsRun = feed.filter(i => i.kind === 'trial').length

  return (
    <div className="flex flex-col gap-space-lg">
      {/* Header */}
      <div className="flex items-start justify-between gap-space-lg">
        <div className="min-w-0">
          <div className="flex items-center gap-space-sm flex-wrap">
            <h1 className="font-headline-lg text-headline-lg text-on-surface">{campaign.goal}</h1>
            <StateBadge state={campaign.state} />
          </div>
          <div className="flex items-center gap-space-sm mt-1 font-code-sm text-code-sm text-on-surface-variant flex-wrap">
            <span className="text-primary">{campaign.model}</span>
            <span className="text-outline">/</span>
            <span>{campaign.gpu_profile}</span>
            <span className="text-outline">/</span>
            <span>{campaign.engines.join(' · ')}</span>
            <span className="text-outline">/</span>
            <span className="text-outline">{campaign.id.slice(0, 8)}</span>
          </div>
        </div>

        {running && (
          <button
            onClick={() => cancel.mutate()}
            disabled={cancel.isPending}
            className="shrink-0 h-8 px-space-sm rounded-xl bg-surface-container-high text-on-surface-variant hover:text-error flex items-center gap-space-xs font-label-mono text-label-mono uppercase active:scale-95 transition-all disabled:opacity-50"
          >
            <StopCircle className="w-4 h-4" />
            {cancel.isPending ? 'cancelling' : 'cancel'}
          </button>
        )}
      </div>

      {campaign.error_msg && (
        <div className="flex items-start gap-space-sm p-space-md rounded-full bg-error/10 text-error font-body-sm text-body-sm">
          <span className="font-semibold shrink-0">Campaign failed:</span>
          <span className="break-words">{campaign.error_msg}</span>
        </div>
      )}

      {/* What it was allowed to do */}
      <Panel>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-space-lg">
          <Fact label="trials" value={`${trialsRun} / ${campaign.budget.max_trials}`} mono />
          <Fact label="time budget" value={campaign.budget.max_duration} mono />
          <Fact
            label="workload"
            value={`${campaign.workload.concurrency}× ${campaign.workload.prompt_tokens}/${campaign.workload.output_tokens}`}
            mono
          />
          <Fact label="planner" value={campaign.planner_model} mono />
        </div>
      </Panel>

      {campaign.recommendation && <Recommendation campaign={campaign} />}

      {/* Execution feed */}
      <div className="flex flex-col gap-space-sm">
        <div className="flex items-center justify-between px-space-xs">
          <div className="flex items-center gap-space-sm">
            <MonoLabel className="text-on-surface-variant">Execution feed</MonoLabel>
            {running && (
              <span className="flex items-center gap-1.5">
                <Ping tone="warning" />
                <MonoLabel className="text-tertiary">live</MonoLabel>
              </span>
            )}
          </div>
          <MonoLabel className="text-outline">{events.length} entries</MonoLabel>
        </div>

        {feed.length === 0 ? (
          <Panel>
            <EmptyState>
              {running ? 'Waiting for the campaign to start…' : 'No steps were recorded.'}
            </EmptyState>
          </Panel>
        ) : (
          <div className="flex flex-col gap-space-sm">
            {feed.map(item =>
              item.kind === 'trial' ? (
                <TrialCard key={item.key} trial={item.trial} />
              ) : (
                <ul key={item.key} className="rounded-full bg-surface-container-low">
                  <NarrationRow event={item.event} />
                </ul>
              ),
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function Recommendation({ campaign }: { campaign: Campaign }) {
  const rec = campaign.recommendation!
  const m = rec.measured

  return (
    <Panel className="bg-surface-container" padded={false}>
      <div className="flex items-center justify-between gap-space-md px-space-lg py-space-md bg-surface-container-high flex-wrap">
        <div className="flex items-center gap-space-sm min-w-0">
          <MonoLabel className="text-secondary">Recommendation</MonoLabel>
          <span className="font-code-lg text-code-lg text-on-surface truncate">
            <span className="text-primary font-medium">{rec.engine}</span>{' '}
            {configSummary(rec.engine_config)}
          </span>
        </div>
        <Pill tone={rec.confidence === 'high' ? 'success' : rec.confidence === 'low' ? 'warning' : 'neutral'}>
          {rec.confidence} confidence
        </Pill>
      </div>

      <div className="flex flex-col gap-space-md p-space-lg">
        {rec.fallback && (
          <Pill tone="warning" className="self-start normal-case">
            selected mechanically — the planner did not conclude
          </Pill>
        )}

        {m && (
          <div className="grid grid-cols-3 gap-space-lg">
            <Headline label="throughput" value={m.tok_per_s.toFixed(0)} unit="tok/s" />
            <Headline label="p99 ttft" value={m.ttft_p99_ms.toFixed(0)} unit="ms" />
            <Headline label="cost" value={`$${m.cost_per_mtok.toFixed(4)}`} unit="/Mtok" />
          </div>
        )}

        <p className="font-body-md text-body-md text-on-surface whitespace-pre-wrap">{rec.reasoning}</p>

        {rec.runner_up && (
          <div>
            <MonoLabel className="text-on-surface-variant">Runner-up</MonoLabel>
            <p className="font-body-md text-body-md text-on-surface-variant mt-0.5">{rec.runner_up}</p>
          </div>
        )}
        {rec.caveats && (
          <div>
            <MonoLabel className="text-tertiary">Caveats</MonoLabel>
            <p className="font-body-md text-body-md text-on-surface-variant mt-0.5">{rec.caveats}</p>
          </div>
        )}

        {campaign.usage && (
          <p className="font-code-sm text-code-sm text-outline pt-space-xs">
            planning: {campaign.usage.turns} turns ·{' '}
            {(campaign.usage.input_tokens + campaign.usage.output_tokens + campaign.usage.cache_read_tokens).toLocaleString()} tokens
          </p>
        )}
      </div>
    </Panel>
  )
}

function Headline({ label, value, unit }: { label: string; value: string; unit: string }) {
  return (
    <div className="min-w-0">
      <MonoLabel className="text-on-surface-variant">{label}</MonoLabel>
      <div className="font-headline-lg text-headline-lg text-on-surface tabular-nums truncate mt-0.5">
        {value}
        <span className="font-code-sm text-code-sm text-on-surface-variant ml-1">{unit}</span>
      </div>
    </div>
  )
}
