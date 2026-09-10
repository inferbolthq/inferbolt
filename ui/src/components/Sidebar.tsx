import { NavLink } from 'react-router-dom'
import { LayoutDashboard, Briefcase, BarChart2, Cpu, Sparkles, TerminalSquare, LogOut } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { healthApi, clearApiKey } from '../api/client'
import { MonoLabel, Ping } from './ui'
import clsx from 'clsx'

const links = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/campaigns', label: 'Campaigns', icon: Sparkles },
  { to: '/jobs', label: 'Jobs', icon: Briefcase },
  { to: '/metrics', label: 'Metrics', icon: BarChart2 },
  { to: '/engines', label: 'Engines', icon: Cpu },
]

export default function Sidebar() {
  const { data } = useQuery({
    queryKey: ['health'],
    queryFn: () => healthApi.check().then(r => r.data),
    refetchInterval: 30_000,
    retry: false,
  })

  const connected = data?.status === 'ok' && data?.postgres === 'ok'

  return (
    <nav className="w-56 bg-surface-container-lowest flex flex-col py-space-xl px-space-md shrink-0">
      {/* Brand block, in the mockup's mono-over-caption treatment */}
      <div className="flex items-center gap-space-sm mb-space-xl px-space-xs">
        <TerminalSquare className="w-5 h-5 text-primary shrink-0" />
        <div className="flex flex-col leading-none min-w-0">
          <MonoLabel className="text-primary">INFERBOLT</MonoLabel>
          <span className="font-code-sm text-code-sm text-on-surface-variant truncate">
            inference optimizer
          </span>
        </div>
      </div>

      <ul className="flex flex-col gap-1 flex-1">
        {links.map(({ to, label, icon: Icon }) => (
          <li key={to}>
            <NavLink
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                clsx(
                  'flex items-center gap-space-sm px-space-sm py-space-sm rounded-xl font-body-md text-body-md transition-colors',
                  isActive
                    ? 'bg-surface-container-high text-primary'
                    : 'text-on-surface-variant hover:bg-surface-container-low hover:text-on-surface',
                )
              }
            >
              <Icon className="w-4 h-4 shrink-0" />
              {label}
            </NavLink>
          </li>
        ))}
      </ul>

      <div className="flex items-center justify-between gap-space-xs px-space-sm py-space-xs mt-space-lg rounded-xl bg-surface-container-low">
        <span className="flex items-center gap-space-xs">
          <Ping tone={connected ? 'success' : 'error'} />
          <MonoLabel className={connected ? 'text-secondary' : 'text-error'}>
            {connected ? 'Connected' : 'Offline'}
          </MonoLabel>
        </span>
        <button
          aria-label="Forget API token"
          title="Forget API token"
          onClick={() => { clearApiKey(); window.location.reload() }}
          className="text-outline hover:text-error transition-colors"
        >
          <LogOut className="w-3.5 h-3.5" />
        </button>
      </div>
    </nav>
  )
}
