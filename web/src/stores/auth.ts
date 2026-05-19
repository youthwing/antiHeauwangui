import { reactive } from 'vue'
import type { Me } from '../types'
import { api } from '../api'

interface AuthState {
  me: Me | null
  initialized: boolean
  loading: boolean
}

const state = reactive<AuthState>({
  me: null,
  initialized: false,
  loading: false,
})

export function useAuth() {
  async function init() {
    if (state.initialized && !state.loading) return
    state.loading = true
    try {
      state.me = await api.me()
    } catch {
      state.me = null
    } finally {
      state.initialized = true
      state.loading = false
    }
  }

  async function refresh() {
    try {
      state.me = await api.me()
    } catch {
      state.me = null
    }
  }

  function clear() {
    state.me = null
  }

  return {
    state,
    init,
    refresh,
    clear,
  }
}

interface AdminAuthState {
  isAdmin: boolean
  initialized: boolean
}
const adminState = reactive<AdminAuthState>({
  isAdmin: false,
  initialized: false,
})

export function useAdminAuth() {
  async function init() {
    try {
      const r = await fetch('/api/v1/airvel/me', { credentials: 'include' })
      adminState.isAdmin = r.ok
    } catch {
      adminState.isAdmin = false
    } finally {
      adminState.initialized = true
    }
  }
  function setAdmin(v: boolean) {
    adminState.isAdmin = v
    adminState.initialized = true
  }
  return { state: adminState, init, setAdmin }
}
