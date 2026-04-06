import type { Config } from 'tailwindcss'

export default {
  content: ['./src/**/*.{vue,ts,tsx}'],
  darkMode: ['variant', '&:where([data-theme="dark"], [data-theme="dark"] *)'],
  theme: {
    screens: {
      sm: '640px',
      md: '768px',
      lg: '1024px',
      xl: '1280px',
    },
    extend: {
      colors: {
        primary: {
          300: 'var(--primary-300)',
          500: 'var(--primary-500)',
          600: 'var(--primary-600)',
        },
        success: {
          500: 'var(--success-500)',
        },
        warning: {
          500: 'var(--warning-500)',
        },
        error: {
          500: 'var(--error-500)',
        },
        surface: {
          base: 'var(--surface-base)',
          raised: 'var(--surface-raised)',
          overlay: 'var(--surface-overlay)',
          canvas: 'var(--bg-canvas)',
          subtle: 'var(--bg-subtle)',
        },
        content: {
          primary: 'var(--text-primary)',
          secondary: 'var(--text-secondary)',
          muted: 'var(--text-muted)',
        },
        outline: {
          DEFAULT: 'var(--border-default)',
          subtle: 'var(--border-subtle)',
        },
      },
      boxShadow: {
        glow: '0 22px 60px rgba(47, 199, 240, 0.20)',
        panel: '0 18px 48px rgba(16, 32, 51, 0.12)',
      },
      fontFamily: {
        sans: ['IBM Plex Sans', 'Noto Sans SC', 'PingFang SC', 'Segoe UI', 'sans-serif'],
        mono: ['IBM Plex Mono', 'JetBrains Mono', 'SFMono-Regular', 'monospace'],
      },
    },
  },
} satisfies Config