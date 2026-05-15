import { reactive } from 'vue'

export interface Toast {
  id: number
  kind: 'ok' | 'err'
  text: string
}

interface ToastState {
  current: Toast | null
}

const state = reactive<ToastState>({ current: null })
let nextId = 1
let timer: number | null = null

export function showToast(kind: 'ok' | 'err', text: string, durationMs = 3500) {
  state.current = { id: nextId++, kind, text }
  if (timer) clearTimeout(timer)
  timer = window.setTimeout(() => (state.current = null), durationMs)
}

export function useToast() {
  return { state, show: showToast }
}
