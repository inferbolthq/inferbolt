/** @type {import('tailwindcss').Config} */

// Design tokens from the Stitch mockup — a Material 3 dark scheme.
//
// Two deliberate departures from the export:
//
//  1. borderRadius.full is 0.75rem in the source, which is the intended look
//     (soft rectangles, not pills) but stops being a circle on anything larger
//     than ~24px. `rounded-circle` is added back for the handful of elements
//     that must actually be round — spinners, status dots.
//  2. Icons stay on lucide-react rather than the Material Symbols font the
//     export links. Same visual weight, no render-blocking font request, and
//     it is already a dependency.
export default {
  darkMode: 'class',
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        'surface-container': '#1c2026',
        'surface': '#10141a',
        'error': '#ffb4ab',
        'tertiary': '#ffc174',
        'on-primary-container': '#004965',
        'surface-bright': '#353940',
        'on-tertiary': '#472a00',
        'background': '#10141a',
        'outline-variant': '#3e484f',
        'tertiary-container': '#f59e0b',
        'on-primary-fixed': '#001e2c',
        'primary': '#8ed5ff',
        'on-secondary-fixed-variant': '#005321',
        'surface-dim': '#10141a',
        'secondary': '#4ae176',
        'primary-fixed': '#c4e7ff',
        'on-tertiary-fixed': '#2a1700',
        'secondary-container': '#00b954',
        'tertiary-fixed': '#ffddb8',
        'on-surface-variant': '#bdc8d1',
        'on-background': '#dfe2eb',
        'on-tertiary-fixed-variant': '#653e00',
        'surface-container-highest': '#31353c',
        'primary-fixed-dim': '#7bd0ff',
        'outline': '#87929a',
        'secondary-fixed': '#6bff8f',
        'inverse-on-surface': '#2d3137',
        'surface-container-low': '#181c22',
        'on-tertiary-container': '#613b00',
        'secondary-fixed-dim': '#4ae176',
        'on-surface': '#dfe2eb',
        'inverse-primary': '#00668a',
        'on-primary-fixed-variant': '#004c69',
        'surface-tint': '#7bd0ff',
        'on-secondary': '#003915',
        'surface-variant': '#31353c',
        'on-primary': '#00354a',
        'error-container': '#93000a',
        'on-secondary-container': '#004119',
        'surface-container-lowest': '#0a0e14',
        'on-secondary-fixed': '#002109',
        'tertiary-fixed-dim': '#ffb95f',
        'primary-container': '#38bdf8',
        'surface-container-high': '#262a31',
        'on-error-container': '#ffdad6',
        'on-error': '#690005',
        'inverse-surface': '#dfe2eb',
      },
      borderRadius: {
        DEFAULT: '0.125rem',
        lg: '0.25rem',
        xl: '0.5rem',
        full: '0.75rem',
        circle: '9999px',
      },
      spacing: {
        'space-xs': '0.25rem',
        'space-sm': '0.5rem',
        'space-md': '0.75rem',
        'space-lg': '1rem',
        'space-xl': '1.5rem',
        'gutter-compact': '0.5rem',
        'gutter': '1rem',
        'margin-mobile': '0.75rem',
        'margin': '1rem',
      },
      fontFamily: {
        'headline-xl': ['Inter', 'system-ui', 'sans-serif'],
        'headline-xl-mobile': ['Inter', 'system-ui', 'sans-serif'],
        'headline-lg': ['Inter', 'system-ui', 'sans-serif'],
        'headline-md': ['Inter', 'system-ui', 'sans-serif'],
        'body-md': ['Inter', 'system-ui', 'sans-serif'],
        'body-sm': ['Inter', 'system-ui', 'sans-serif'],
        'label-md': ['Inter', 'system-ui', 'sans-serif'],
        'code-lg': ['JetBrains Mono', 'ui-monospace', 'monospace'],
        'code-md': ['JetBrains Mono', 'ui-monospace', 'monospace'],
        'code-sm': ['JetBrains Mono', 'ui-monospace', 'monospace'],
        'label-mono': ['JetBrains Mono', 'ui-monospace', 'monospace'],
      },
      fontSize: {
        'headline-xl': ['32px', { lineHeight: '40px', letterSpacing: '-0.02em', fontWeight: '600' }],
        'headline-xl-mobile': ['24px', { lineHeight: '32px', letterSpacing: '-0.01em', fontWeight: '600' }],
        'headline-lg': ['20px', { lineHeight: '28px', letterSpacing: '-0.01em', fontWeight: '600' }],
        'headline-md': ['16px', { lineHeight: '24px', fontWeight: '600' }],
        'body-md': ['14px', { lineHeight: '20px', fontWeight: '400' }],
        'body-sm': ['12px', { lineHeight: '16px', fontWeight: '400' }],
        'label-md': ['12px', { lineHeight: '16px', letterSpacing: '0.02em', fontWeight: '500' }],
        'label-mono': ['11px', { lineHeight: '14px', letterSpacing: '0.04em', fontWeight: '500' }],
        'code-lg': ['14px', { lineHeight: '22px', fontWeight: '500' }],
        'code-md': ['13px', { lineHeight: '20px', fontWeight: '400' }],
        'code-sm': ['11px', { lineHeight: '16px', fontWeight: '400' }],
      },
    },
  },
  plugins: [],
}
