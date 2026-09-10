import axios from 'axios'

export const API_KEY_STORAGE_KEY = 'inferbolt_api_key'

/** Fired when the gateway rejects the stored token, so the UI can ask again. */
export const AUTH_FAILED_EVENT = 'inferbolt:auth-failed'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? 'http://localhost:8080',
})

function readApiKey(): string | null {
  try {
    return localStorage.getItem(API_KEY_STORAGE_KEY)
  } catch {
    return null // private mode, or storage blocked
  }
}

export function setApiKey(token: string) {
  try {
    localStorage.setItem(API_KEY_STORAGE_KEY, token)
  } catch {
    // Non-persistent is still usable for this tab; the interceptor re-reads
    // each request and will simply find nothing next time.
  }
}

export function clearApiKey() {
  try {
    localStorage.removeItem(API_KEY_STORAGE_KEY)
  } catch { /* nothing to clear */ }
}

api.interceptors.request.use(config => {
  const key = readApiKey()
  if (key) config.headers.Authorization = `Bearer ${key}`
  return config
})

// A rejected token should send the user back to the prompt. Without this every
// panel just reads "failed to load", which says nothing about the cause.
api.interceptors.response.use(
  response => response,
  error => {
    if (error?.response?.status === 401) {
      clearApiKey()
      window.dispatchEvent(new Event(AUTH_FAILED_EVENT))
    }
    return Promise.reject(error)
  },
)

export interface Job {
  id: string
  tenant_id: string
  model: string
  engines: string[]
  state: string
  gpu_profile: string
  created_at: string
  updated_at: string
  error_msg?: string
}

export interface BenchmarkResult {
  job_id: string
  engine: string
  model: string
  ttft_p50_ms: number
  ttft_p99_ms: number
  itl_ms: number
  tok_per_s: number
  gpu_mem_mb: number
  kv_cache_hit: number
  error_rate: number
  cost_per_mtok: number
  config: Record<string, unknown>
}

export interface Recommendation {
  job_id: string
  best_engine: string
  best_config: Record<string, unknown>
  cost_per_mtok: number
  tok_per_sec: number
  reasoning: string
}

// ── Campaigns ─────────────────────────────────────────────────────────────────

export interface WorkloadConfig {
  concurrency: number
  prompt_tokens: number
  output_tokens: number
  num_requests: number
}

export interface EngineConfig {
  quantization?: string
  tensor_parallel?: number
  max_batch_size?: number
  max_model_len?: number
  gpu_memory_utilization?: number
}

export interface Budget {
  max_trials: number
  max_duration: string
  max_turns: number
}

export interface TrialSpec {
  model: string
  engine: string
  gpu_profile: string
  workload: WorkloadConfig
  engine_config: EngineConfig
}

export interface Trial {
  spec: TrialSpec
  result: BenchmarkResult
  error?: string
  duration: number
  hypothesis?: string
  reused?: boolean
}

export interface CampaignRecommendation {
  engine: string
  engine_config: EngineConfig
  reasoning: string
  runner_up?: string
  confidence: string
  caveats?: string
  measured?: BenchmarkResult
  /** True when the campaign ran out of budget and the pick was made mechanically. */
  fallback?: boolean
}

export interface CampaignUsage {
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  turns: number
}

export type CampaignState = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'

export interface Campaign {
  id: string
  tenant_id: string
  goal: string
  model: string
  gpu_profile: string
  engines: string[]
  workload: WorkloadConfig
  budget: Budget
  planner_model: string
  state: CampaignState
  recommendation?: CampaignRecommendation
  trials?: Trial[]
  usage?: CampaignUsage
  error_msg?: string
  created_at: string
  updated_at: string
  started_at?: string
  completed_at?: string
}

export type CampaignEventKind =
  | 'plan' | 'think' | 'trial' | 'result' | 'failed' | 'lookup' | 'warn' | 'done'

export interface CampaignEvent {
  seq: number
  kind: CampaignEventKind
  text?: string
  trial?: TrialSpec
  result?: BenchmarkResult
  elapsed_ms: number
  created_at: string
}

/** One page of campaign progress, plus enough state to know when to stop polling. */
export interface CampaignEventPage {
  events: CampaignEvent[]
  last_seq: number
  state: CampaignState
  done: boolean
}

export const TERMINAL_CAMPAIGN_STATES: CampaignState[] = ['completed', 'failed', 'cancelled']

export function isCampaignTerminal(state: CampaignState): boolean {
  return TERMINAL_CAMPAIGN_STATES.includes(state)
}

export const campaignsApi = {
  list: (state?: CampaignState, limit = 20) =>
    api.get<{ campaigns: Campaign[]; total: number }>('/v1/campaigns', { params: { state, limit } }),
  get: (id: string) =>
    api.get<Campaign>(`/v1/campaigns/${id}`),
  create: (body: object) =>
    api.post<{ campaign_id: string; state: CampaignState }>('/v1/campaigns', body),
  cancel: (id: string) =>
    api.delete(`/v1/campaigns/${id}`),
  /** Progress recorded after afterSeq. Poll with the returned last_seq to follow a run. */
  events: (id: string, afterSeq = 0, limit = 200) =>
    api.get<CampaignEventPage>(`/v1/campaigns/${id}/events`, { params: { after_seq: afterSeq, limit } }),
}

export const workersApi = {
  list: () => api.get<Array<{ id: string; gpu_type: string; status: string }>>('/v1/workers'),
}

export const jobsApi = {
  list: (state?: string, limit = 20) =>
    api.get<{ jobs: Job[]; total: number }>('/v1/jobs', { params: { state, limit } }),
  get: (id: string) =>
    api.get<Job>(`/v1/jobs/${id}`),
  getResults: (id: string) =>
    api.get<BenchmarkResult[]>(`/v1/jobs/${id}/results`),
  create: (body: object) =>
    api.post<{ job_id: string; state: string }>('/v1/jobs', body),
  cancel: (id: string) =>
    api.delete(`/v1/jobs/${id}`),
}

export const metricsApi = {
  query: (engine: string, model: string, since: string) =>
    api.get<BenchmarkResult[]>('/v1/metrics', { params: { engine, model, since } }),
}

export const healthApi = {
  check: () =>
    api.get<{ status: string; postgres: string; version: string }>('/health'),
}
