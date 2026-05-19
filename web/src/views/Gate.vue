<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { AlertCircle, ArrowRight, KeyRound, ShieldCheck } from 'lucide-vue-next'
import Logo from '../components/Logo.vue'
import { enterSite } from '../api'
import { showToast } from '../lib/toast'

const route = useRoute()
const router = useRouter()

const code = ref('')
const submitting = ref(false)
const error = ref<string | null>(null)

const cleanCode = computed(() => code.value.trim())
const canSubmit = computed(() => cleanCode.value.length >= 8 && !submitting.value)

function redirectTarget() {
  const raw = typeof route.query.redirect === 'string' ? route.query.redirect : '/login'
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/airvel')) return '/login'
  return raw
}

async function submit() {
  if (!canSubmit.value) return
  submitting.value = true
  error.value = null
  try {
    await enterSite(cleanCode.value)
    showToast('ok', '入口已打开')
    router.replace(redirectTarget())
  } catch (e: any) {
    error.value = e.message || '访问码无效'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="min-h-dvh flex items-center justify-center px-4 py-10">
    <section class="w-full max-w-sm">
      <div class="flex flex-col items-center mb-8">
        <Logo :size="58" />
        <h1 class="mt-5 text-3xl font-bold tracking-tight text-center">antiWG</h1>
        <p class="mt-2 text-sm text-zinc-500 dark:text-zinc-400">入口验证</p>
      </div>

      <form
        class="bg-white/90 dark:bg-[#161b22]/70 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-2xl p-6 backdrop-blur-sm"
        @submit.prevent="submit"
      >
        <label for="site-gate-code" class="flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">
          <KeyRound class="w-3.5 h-3.5" />
          访问码
        </label>
        <input
          id="site-gate-code"
          v-model="code"
          autocomplete="one-time-code"
          autocapitalize="characters"
          spellcheck="false"
          placeholder="AWG-XXXX-XXXX-XXXX"
          class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-3 text-[#161b22] dark:text-zinc-100 placeholder:text-zinc-300 dark:placeholder:text-zinc-700 focus-ring font-mono-token text-center text-base"
          @keyup.enter="submit"
        />

        <button
          type="submit"
          :disabled="!canSubmit"
          class="mt-4 w-full inline-flex items-center justify-center gap-2 bg-[#e50914] hover:bg-red-500 disabled:bg-zinc-200 dark:disabled:bg-zinc-800 disabled:text-zinc-500 text-white font-medium py-2.5 rounded-xl transition-all disabled:cursor-not-allowed"
        >
          <ShieldCheck v-if="submitting" class="w-4 h-4 wangui-spin" />
          <span>{{ submitting ? '校验中…' : '进入' }}</span>
          <ArrowRight v-if="!submitting" class="w-4 h-4" />
        </button>

        <div
          v-if="error"
          class="mt-3 flex items-start gap-2 px-3 py-2 rounded-lg bg-red-500/10 ring-1 ring-red-500/20 text-sm text-red-500 dark:text-red-400"
        >
          <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
          <span>{{ error }}</span>
        </div>
      </form>
    </section>
  </main>
</template>
