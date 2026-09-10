import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  campaignsApi,
  isCampaignTerminal,
  type Campaign,
  type CampaignEvent,
} from '../api/client'

/**
 * How often a follower asks for new steps. A campaign emits a step every few
 * minutes at most, so this governs how quickly output appears, not throughput.
 */
const POLL_INTERVAL_MS = 2_000

/** Polls one campaign, stopping once it reaches a terminal state. */
export function useCampaign(campaignId: string | undefined) {
  return useQuery<Campaign>({
    queryKey: ['campaign', campaignId],
    queryFn: () => campaignsApi.get(campaignId!).then(r => r.data),
    enabled: !!campaignId,
    refetchInterval: query => {
      const c = query.state.data
      return c && isCampaignTerminal(c.state) ? false : POLL_INTERVAL_MS
    },
  })
}

export interface CampaignFeed {
  events: CampaignEvent[]
  done: boolean
  isLoading: boolean
  error: unknown
}

/**
 * Follows a campaign's progress, accumulating steps across polls.
 *
 * The server assigns each step a per-campaign sequence number and serves
 * everything after a cursor, so this asks only for what it has not seen. On
 * mount the cursor starts at zero, which replays the campaign from the
 * beginning — that is what makes opening a half-finished campaign show the
 * whole story rather than only what happens next.
 */
export function useCampaignEvents(campaignId: string | undefined): CampaignFeed {
  const [events, setEvents] = useState<CampaignEvent[]>([])
  const [done, setDone] = useState(false)
  const cursor = useRef(0)

  // A different campaign is a different story; never let one bleed into another.
  useEffect(() => {
    setEvents([])
    setDone(false)
    cursor.current = 0
  }, [campaignId])

  const { data, isLoading, error } = useQuery({
    queryKey: ['campaign-events', campaignId],
    queryFn: () => campaignsApi.events(campaignId!, cursor.current).then(r => r.data),
    enabled: !!campaignId && !done,
    refetchInterval: done ? false : POLL_INTERVAL_MS,
    // The accumulated list is the state that matters; caching pages would only
    // risk replaying one after a remount.
    gcTime: 0,
  })

  useEffect(() => {
    if (!data) return
    if (data.events.length > 0) {
      cursor.current = data.last_seq
      setEvents(prev => [...prev, ...data.events])
    }
    if (data.done) setDone(true)
  }, [data])

  return { events, done, isLoading, error }
}
