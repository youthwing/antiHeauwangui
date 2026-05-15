import { reactive, watchEffect } from 'vue'

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'wangui:theme'

function detectInitial(): Theme {
  const saved = localStorage.getItem(STORAGE_KEY) as Theme | null
  if (saved === 'light' || saved === 'dark') return saved
  if (window.matchMedia?.('(prefers-color-scheme: dark)').matches) return 'dark'
  return 'light'
}

const state = reactive<{ theme: Theme }>({
  theme: detectInitial(),
})

function applyTheme(t: Theme) {
  const html = document.documentElement
  html.classList.toggle('dark', t === 'dark')
  html.style.colorScheme = t
}

// Apply immediately at module load (before app mounts) to avoid flash.
applyTheme(state.theme)

watchEffect(() => {
  applyTheme(state.theme)
  localStorage.setItem(STORAGE_KEY, state.theme)
})

export function useTheme() {
  function set(t: Theme) {
    state.theme = t
  }
  function toggle() {
    state.theme = state.theme === 'dark' ? 'light' : 'dark'
  }
  return { state, set, toggle }
}
