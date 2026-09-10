import clsx from 'clsx'
import type { ReactNode } from 'react'

/**
 * The surface every block of content sits on. Extracted because the card
 * treatment appeared a dozen times across the pages, and the design system
 * expresses depth through surface tiers rather than borders and shadows.
 */
export function Panel({
  children,
  className,
  padded = true,
}: {
  children: ReactNode
  className?: string
  padded?: boolean
}) {
  return (
    <div
      className={clsx(
        'rounded-full bg-surface-container-low overflow-hidden',
        padded && 'p-space-lg',
        className,
      )}
    >
      {children}
    </div>
  )
}

export function PanelTitle({ children, right }: { children: ReactNode; right?: ReactNode }) {
  return (
    <div className="flex items-center justify-between mb-space-md">
      <h2 className="font-headline-md text-headline-md text-on-surface">{children}</h2>
      {right}
    </div>
  )
}

export function PageHeader({ title, subtitle, right }: { title: string; subtitle?: string; right?: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-space-lg">
      <div className="min-w-0">
        <h1 className="font-headline-lg text-headline-lg text-on-surface">{title}</h1>
        {subtitle && (
          <p className="font-body-sm text-body-sm text-on-surface-variant mt-1">{subtitle}</p>
        )}
      </div>
      {right}
    </div>
  )
}

/** Uppercase mono label — the design's workhorse for metadata. */
export function MonoLabel({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={clsx('font-label-mono text-label-mono uppercase', className)}>{children}</span>
  )
}

export type Tone = 'neutral' | 'primary' | 'success' | 'warning' | 'error'

const TONE_CLASSES: Record<Tone, string> = {
  neutral: 'bg-surface-container-high text-on-surface-variant',
  primary: 'bg-primary/10 text-primary',
  success: 'bg-secondary/15 text-secondary',
  warning: 'bg-tertiary-container/20 text-tertiary',
  error: 'bg-error/15 text-error',
}

export function Pill({
  children,
  tone = 'neutral',
  pulse,
  className,
}: {
  children: ReactNode
  tone?: Tone
  /** Ping animation for a live state. */
  pulse?: boolean
  className?: string
}) {
  return (
    <span
      className={clsx(
        'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full font-label-mono text-label-mono uppercase whitespace-nowrap',
        TONE_CLASSES[tone],
        className,
      )}
    >
      {pulse && <Ping tone={tone} />}
      {children}
    </span>
  )
}

const PING_CLASSES: Record<Tone, string> = {
  neutral: 'bg-outline',
  primary: 'bg-primary',
  success: 'bg-secondary',
  warning: 'bg-tertiary',
  error: 'bg-error',
}

/** The mockup's live indicator: a solid dot under an expanding ghost. */
export function Ping({ tone = 'success' }: { tone?: Tone }) {
  return (
    <span className="relative flex h-2 w-2">
      <span className={clsx('animate-ping absolute inline-flex h-full w-full rounded-circle opacity-75', PING_CLASSES[tone])} />
      <span className={clsx('relative inline-flex rounded-circle h-2 w-2', PING_CLASSES[tone])} />
    </span>
  )
}

export function Spinner({ className }: { className?: string }) {
  return (
    <span
      className={clsx(
        'inline-block border-2 border-primary border-t-transparent rounded-circle animate-spin',
        className ?? 'w-4 h-4',
      )}
    />
  )
}

/** A labelled figure. `mono` for anything the eye scans column-wise. */
export function Fact({ label, value, mono }: { label: string; value: ReactNode; mono?: boolean }) {
  return (
    <div className="min-w-0">
      <MonoLabel className="text-on-surface-variant">{label}</MonoLabel>
      <div
        className={clsx(
          'text-on-surface mt-0.5 truncate',
          mono ? 'font-code-md text-code-md' : 'font-body-md text-body-md',
        )}
      >
        {value}
      </div>
    </div>
  )
}

export function EmptyState({ children }: { children: ReactNode }) {
  return (
    <div className="py-space-xl text-center font-body-sm text-body-sm text-on-surface-variant">
      {children}
    </div>
  )
}

export function ErrorText({ children }: { children: ReactNode }) {
  return <p className="font-body-sm text-body-sm text-error">{children}</p>
}

export function LoadingText() {
  return (
    <p className="flex items-center gap-space-sm font-body-sm text-body-sm text-on-surface-variant">
      <Spinner className="w-3 h-3" />
      Loading…
    </p>
  )
}

/**
 * Shared Recharts styling. The charts were written against a light theme with
 * hardcoded hexes; keeping the palette in one place stops them drifting apart
 * from the tokens again.
 */
export const chart = {
  primary: '#8ed5ff',
  secondary: '#4ae176',
  tertiary: '#ffc174',
  error: '#ffb4ab',
  grid: '#3e484f',
  axis: '#87929a',
  tick: { fontSize: 11, fill: '#87929a', fontFamily: 'JetBrains Mono' },
  tooltip: {
    contentStyle: {
      background: '#1c2026',
      border: '1px solid #3e484f',
      borderRadius: '0.5rem',
      fontSize: 12,
      color: '#dfe2eb',
    },
    labelStyle: { color: '#bdc8d1' },
  },
  legend: { wrapperStyle: { fontSize: 11, color: '#bdc8d1' } },
}
