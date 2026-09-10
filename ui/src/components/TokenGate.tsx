import { useEffect, useState, type ReactNode } from 'react'
import { KeyRound } from 'lucide-react'
import { API_KEY_STORAGE_KEY, AUTH_FAILED_EVENT, setApiKey } from '../api/client'
import { MonoLabel, Panel } from './ui'

/**
 * Holds the API token the client sends on every request.
 *
 * This is deliberately the smallest thing that works: the token lives in
 * localStorage and the user pastes it in. It is not a login — sessions,
 * cookies and the CSRF question that comes with them are an open decision, and
 * this does not foreclose any of it. Until then a browser client needs *some*
 * way to hold a credential, and an unexplained 401 is a worse answer.
 */
export default function TokenGate({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => {
    try {
      return localStorage.getItem(API_KEY_STORAGE_KEY)
    } catch {
      return null // private mode, or storage blocked
    }
  })

  // A token that has expired or been revoked should return the user here
  // rather than leaving every panel stuck on "failed to load".
  useEffect(() => {
    const onAuthFailed = () => setToken(null)
    window.addEventListener(AUTH_FAILED_EVENT, onAuthFailed)
    return () => window.removeEventListener(AUTH_FAILED_EVENT, onAuthFailed)
  }, [])

  if (token) return <>{children}</>

  return <TokenPrompt onSubmit={t => { setApiKey(t); setToken(t) }} />
}

function TokenPrompt({ onSubmit }: { onSubmit: (token: string) => void }) {
  const [value, setValue] = useState('')
  const trimmed = value.trim()

  return (
    <div className="flex items-center justify-center min-h-screen bg-background p-space-xl">
      <Panel className="w-full max-w-md" padded={false}>
        <div className="flex items-center gap-space-sm px-space-lg py-space-md bg-surface-container-high">
          <KeyRound className="w-4 h-4 text-primary" />
          <MonoLabel className="text-primary">InferBolt — API token</MonoLabel>
        </div>

        <form
          className="flex flex-col gap-space-md p-space-lg"
          onSubmit={e => {
            e.preventDefault()
            if (trimmed) onSubmit(trimmed)
          }}
        >
          <p className="font-body-md text-body-md text-on-surface-variant">
            The dashboard talks to the gateway with a bearer token. Paste one to continue.
          </p>

          <input
            autoFocus
            type="password"
            value={value}
            onChange={e => setValue(e.target.value)}
            placeholder="eyJhbGciOi…"
            spellCheck={false}
            className="bg-surface-container-lowest rounded-xl px-space-md py-space-sm font-code-md text-code-md text-on-surface placeholder:text-outline border-none outline-none focus:ring-1 focus:ring-primary"
          />

          <button
            type="submit"
            disabled={!trimmed}
            className="h-9 rounded-xl bg-primary-container text-on-primary-container font-label-mono text-label-mono uppercase active:scale-95 transition-all disabled:opacity-40 disabled:active:scale-100"
          >
            Connect
          </button>

          <div className="flex flex-col gap-space-xs pt-space-xs font-code-sm text-code-sm text-outline">
            <span>Where to find one:</span>
            <code className="text-on-surface-variant">cat .inferbolt-dev/dev-token</code>
            <code className="text-on-surface-variant">inferbolt admin apikeys create</code>
          </div>
        </form>
      </Panel>
    </div>
  )
}
