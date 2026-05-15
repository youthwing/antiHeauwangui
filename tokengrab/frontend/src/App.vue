<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, computed } from 'vue'
import {
  Loader2,
  Check,
  AlertCircle,
  Copy,
  ExternalLink,
  RotateCcw,
  ChevronRight,
  ShieldOff,
  Ticket,
} from 'lucide-vue-next'

type Phase = 'idle' | 'capturing' | 'captured' | 'error'

const phase = ref<Phase>('idle')
const progressDone = ref<string[]>([])
const progressStep = ref('')
const progressError = ref('')
const captured = ref<null | {
  token: string
  userId: string
  expiresAt: number
  remainingSec: number
  clipboardOk: boolean
  userName: string
  userNumber: string
  userSection: string
  userClass: string
  userAvatarUrl: string
}>(null)
const residual = ref(false)
const caInstalled = ref(true) // optimistic; we re-check in onMounted

// wangui base URL is hard-coded — every alpha user points at the same
// instance. If we ever fork or self-host another wangui, this is the one
// constant to change.
const WANGUI_URL = 'https://wangui.gptcodex.top'

// Invite code is only needed for the first-time activation. Token refreshes
// (token expired → grab again) don't need it. Optional input, NOT persisted
// across launches.
const inviteCode = ref('')

const captureTimer = ref(60)
let timerInterval: number | null = null

const STEPS = ['加载临时证书', '信任 CA', '设置系统代理', '等待签到入口请求']

function startTimer() {
  captureTimer.value = 60
  if (timerInterval) window.clearInterval(timerInterval)
  timerInterval = window.setInterval(() => {
    captureTimer.value = Math.max(0, captureTimer.value - 1)
  }, 1000)
}
function stopTimer() {
  if (timerInterval) {
    window.clearInterval(timerInterval)
    timerInterval = null
  }
}

async function start() {
  progressDone.value = []
  progressStep.value = ''
  progressError.value = ''
  captured.value = null
  phase.value = 'capturing'
  startTimer()
  try {
    await window.go.main.App.Start()
  } catch (e) {
    progressError.value = String(e)
    phase.value = 'error'
    stopTimer()
  }
}

async function cancel() {
  await window.go.main.App.Cancel()
  await reset()
}

async function reset() {
  stopTimer()
  await window.go.main.App.Reset()
  phase.value = 'idle'
  progressDone.value = []
  progressStep.value = ''
  progressError.value = ''
  captured.value = null
}

async function copyAgain() {
  if (!captured.value) return
  try {
    await window.go.main.App.SetClipboard(captured.value.token)
  } catch {
    // ignore — Go side already copied at capture time
  }
}

async function openWangui() {
  if (!captured.value) return
  try {
    await window.go.main.App.OpenWanguiActivate(
      WANGUI_URL,
      captured.value.token,
      inviteCode.value.trim(),
    )
  } catch (e: any) {
    alert(e?.message || '打开失败')
  }
}

async function cleanResidual() {
  await window.go.main.App.CleanResidual()
  residual.value = false
}

async function uninstallCA() {
  const ok = window.confirm(
    '确认卸载持久 CA？\n\n' +
      '本机将不再信任 wangui-tokengrab 签发的证书。\n' +
      '下次抓取时需要重新装一次（Windows 会再次弹安全确认窗口）。\n\n' +
      '此操作会触发 Windows 的“是否删除证书”确认窗口。',
  )
  if (!ok) return
  try {
    await window.go.main.App.UninstallPersistentCA()
    caInstalled.value = false
  } catch (e: any) {
    alert(e?.message || '卸载失败')
  }
}

// Wails event subscriptions
let unsubProgress: (() => void) | null = null
let unsubCaptured: (() => void) | null = null
let unsubResidual: (() => void) | null = null
let unsubCAInstalled: (() => void) | null = null

onMounted(async () => {
  phase.value = (await window.go.main.App.GetPhase()) as Phase

  unsubProgress = window.runtime.EventsOn('progress', (evt: any) => {
    phase.value = evt.phase
    progressStep.value = evt.step || ''
    progressDone.value = evt.done || []
    progressError.value = evt.message || ''
    if (evt.phase !== 'capturing') stopTimer()
  })
  unsubCaptured = window.runtime.EventsOn('captured', (evt: any) => {
    captured.value = evt
    phase.value = 'captured'
    stopTimer()
  })
  unsubResidual = window.runtime.EventsOn('residual', () => {
    residual.value = true
  })
  unsubCAInstalled = window.runtime.EventsOn('ca-installed', (v: any) => {
    caInstalled.value = !!v
  })

  residual.value = await window.go.main.App.CheckResidual()
  caInstalled.value = await window.go.main.App.CAInstalled()
})

onBeforeUnmount(() => {
  unsubProgress?.()
  unsubCaptured?.()
  unsubResidual?.()
  unsubCAInstalled?.()
  stopTimer()
})

const expiryRemaining = computed(() => {
  if (!captured.value) return ''
  const s = captured.value.remainingSec
  if (s <= 0) return '已过期'
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  if (d > 0) return `${d} 天 ${h} 小时`
  const m = Math.floor((s % 3600) / 60)
  return `${h} 小时 ${m} 分钟`
})

const expiryDate = computed(() => {
  if (!captured.value) return ''
  const d = new Date(captured.value.expiresAt * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
})
</script>

<template>
  <div class="h-screen flex flex-col">
    <!-- Header -->
    <header class="flex items-center gap-3 px-6 py-4 border-b border-white/[0.06] shrink-0">
      <div class="w-9 h-9 rounded-xl bg-emerald-500/15 ring-1 ring-emerald-500/30 flex items-center justify-center shrink-0">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="text-emerald-400">
          <path d="M20 3 A3 3 0 1 0 20 7.4 A2.2 2.2 0 0 1 20 3 Z" fill="currentColor" stroke="none" />
          <path d="M3 14 L12 7 L21 14" />
          <path d="M5 13 V21 H19 V13" />
          <path d="M10 21 V16 H14 V21" />
        </svg>
      </div>
      <div>
        <h1 class="text-base font-bold tracking-tight">wangui</h1>
        <p class="text-[10px] text-zinc-500 tracking-wider">勿外传</p>
      </div>
    </header>

    <!-- Residual banner -->
    <Transition name="fade">
      <div v-if="residual && phase === 'idle'" class="mx-6 mt-3 rounded-xl bg-amber-500/10 ring-1 ring-amber-500/30 p-3 flex items-start gap-2 shrink-0">
        <AlertCircle class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
        <div class="flex-1 min-w-0">
          <p class="text-xs font-medium text-amber-300">检测到上次未清理</p>
          <p class="text-[11px] text-amber-400/80 mt-0.5">系统代理或根证书库可能仍处于上一次会话的状态</p>
        </div>
        <button @click="cleanResidual" class="shrink-0 text-xs px-3 py-1 rounded-md bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 transition-colors">立即清理</button>
      </div>
    </Transition>

    <main class="flex-1 px-6 py-5 overflow-hidden">
      <!-- Idle -->
      <section v-if="phase === 'idle'" class="space-y-5">
        <div class="flex flex-col items-center pt-4">
          <div class="w-16 h-16 rounded-2xl bg-emerald-500/15 ring-1 ring-emerald-500/30 flex items-center justify-center mb-4">
            <svg width="38" height="38" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="text-emerald-400">
              <path d="M20 3 A3 3 0 1 0 20 7.4 A2.2 2.2 0 0 1 20 3 Z" fill="currentColor" stroke="none" />
              <path d="M3 14 L12 7 L21 14" />
              <path d="M5 13 V21 H19 V13" />
              <path d="M10 21 V16 H14 V21" />
            </svg>
          </div>
          <h2 class="text-xl font-bold tracking-tight">wangui</h2>
          <p class="text-xs text-zinc-500 mt-1">勿外传</p>
        </div>

        <div class="rounded-xl bg-white/[0.02] ring-1 ring-white/[0.05] p-4">
          <p class="text-[10px] text-zinc-500 tracking-wider uppercase mb-3">使用步骤</p>
          <ol class="space-y-2 text-sm text-zinc-300">
            <li class="flex gap-2"><span class="text-emerald-400 font-semibold">1.</span><span>在微信 PC 端打开签到入口</span></li>
            <li class="flex gap-2"><span class="text-emerald-400 font-semibold">2.</span><span>点下方「开始抓取」</span></li>
            <li class="flex gap-2"><span class="text-emerald-400 font-semibold">3.</span><span>回微信，按 <kbd class="px-1.5 py-0.5 rounded bg-zinc-800 text-[10px] font-mono-token">Ctrl+R</kbd> 刷新页面</span></li>
          </ol>
        </div>

        <button @click="start" class="w-full bg-emerald-500 hover:bg-emerald-400 text-zinc-950 font-semibold py-3 rounded-xl transition-colors inline-flex items-center justify-center gap-2 shadow-[0_8px_20px_-8px_rgba(16,185,129,0.5)]">
          开始抓取
          <ChevronRight class="w-4 h-4" />
        </button>

        <!-- First-run notice / CA management -->
        <div v-if="!caInstalled" class="rounded-xl bg-amber-500/[0.07] ring-1 ring-amber-500/25 p-3 flex items-start gap-2">
          <AlertCircle class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
          <div class="text-[11px] text-amber-200 leading-relaxed">
            <p class="font-medium">首次启动需要确认信任 CA</p>
            <p class="mt-1 text-amber-300/80">
              点「开始抓取」后 Windows 会弹一次安全确认窗口，请点「是」。<br />
              之后每次启动都不再弹窗。
            </p>
          </div>
        </div>

        <p v-else class="text-[10px] text-zinc-500 leading-relaxed text-center">
          ⓘ 持久 CA 已信任。开始抓取时只设置系统代理，<span class="text-zinc-400">无弹窗</span>。
        </p>

        <!-- Uninstall CA button (subtle, in the "settings" zone) -->
        <button
          v-if="caInstalled"
          @click="uninstallCA"
          class="w-full text-[10px] text-zinc-600 hover:text-red-400 transition-colors inline-flex items-center justify-center gap-1 pt-1"
        >
          <ShieldOff class="w-3 h-3" />
          完全卸载持久 CA（取消信任）
        </button>
      </section>

      <!-- Capturing -->
      <section v-else-if="phase === 'capturing'" class="space-y-5">
        <div class="flex items-center gap-3 pt-2">
          <Loader2 class="w-5 h-5 text-emerald-400 tg-spin" />
          <h2 class="text-lg font-bold">抓取中…</h2>
        </div>

        <div class="space-y-2.5">
          <div v-for="s in STEPS" :key="s" class="flex items-center gap-3 text-sm">
            <span v-if="progressDone.includes(s)" class="w-5 h-5 rounded-full bg-emerald-500/15 ring-1 ring-emerald-500/30 flex items-center justify-center shrink-0">
              <Check class="w-3 h-3 text-emerald-400" :stroke-width="3" />
            </span>
            <span v-else-if="progressStep === s" class="w-5 h-5 rounded-full bg-emerald-500/15 ring-1 ring-emerald-500/30 flex items-center justify-center shrink-0">
              <Loader2 class="w-3 h-3 text-emerald-400 tg-spin" />
            </span>
            <span v-else class="w-5 h-5 rounded-full ring-1 ring-zinc-700 shrink-0" />
            <span :class="progressDone.includes(s) ? 'text-zinc-300' : progressStep === s ? 'text-zinc-200 font-medium' : 'text-zinc-500'">
              {{ s }}
              <span v-if="progressStep === s && s === '等待签到入口请求'" class="ml-2 font-mono-token text-emerald-400 text-xs">({{ captureTimer }}s)</span>
            </span>
          </div>
        </div>

        <div class="rounded-xl bg-white/[0.02] ring-1 ring-white/[0.05] p-4">
          <p class="text-xs text-zinc-400 leading-relaxed">
            请现在<span class="text-emerald-300">在微信 PC 端打开签到入口</span>，并按
            <kbd class="px-1.5 py-0.5 rounded bg-zinc-800 text-[10px] font-mono-token">Ctrl+R</kbd>
            刷新一下页面。
          </p>
        </div>

        <button @click="cancel" class="w-full bg-red-500/10 hover:bg-red-500/20 ring-1 ring-red-500/30 text-red-300 font-medium py-2.5 rounded-xl transition-colors">
          取消并清理
        </button>
      </section>

      <!-- Captured -->
      <section v-else-if="phase === 'captured' && captured" class="space-y-4">
        <div class="flex items-center gap-3 pt-2">
          <div class="w-7 h-7 rounded-full bg-emerald-500/15 ring-1 ring-emerald-500/30 flex items-center justify-center">
            <Check class="w-4 h-4 text-emerald-400" :stroke-width="3" />
          </div>
          <h2 class="text-lg font-bold">抓取成功</h2>
        </div>

        <!-- User profile card (best-effort enrichment from /auth/user) -->
        <div v-if="captured.userName" class="rounded-xl bg-white/[0.02] ring-1 ring-white/[0.05] p-4">
          <div class="flex items-center gap-3">
            <!-- Avatar (name initial — webview can't reliably load the remote
                 avatar URL without cookies/referer, so we skip the image) -->
            <div class="w-14 h-14 rounded-xl bg-emerald-500/15 ring-1 ring-emerald-500/30 flex items-center justify-center shrink-0">
              <span class="text-emerald-400 font-bold text-xl">{{ captured.userName.slice(0, 1) }}</span>
            </div>
            <div class="min-w-0 flex-1">
              <p class="text-base font-bold tracking-tight truncate">{{ captured.userName }}</p>
              <p class="text-[11px] text-zinc-500 mt-1 truncate">
                <span>{{ captured.userSection || '—' }}</span>
                <span class="mx-1.5 text-zinc-700">·</span>
                <span>{{ captured.userClass || '—' }}</span>
              </p>
              <p class="text-[11px] text-zinc-500 font-mono-token tabular-nums mt-0.5">{{ captured.userNumber || captured.userId }}</p>
            </div>
          </div>
        </div>
        <!-- Fallback when /auth/user couldn't be reached -->
        <div v-else class="rounded-xl bg-white/[0.02] ring-1 ring-white/[0.05] p-4 flex items-center justify-between text-sm">
          <span class="text-zinc-500 text-xs">用户 ID</span>
          <span class="font-mono-token tabular-nums">{{ captured.userId }}</span>
        </div>

        <dl class="rounded-xl bg-white/[0.02] ring-1 ring-white/[0.05] p-4 space-y-2.5 text-sm">
          <div class="flex items-center justify-between">
            <dt class="text-zinc-500 text-xs">有效期</dt>
            <dd class="text-emerald-300 font-medium">{{ expiryRemaining }}</dd>
          </div>
          <div class="flex items-center justify-between">
            <dt class="text-zinc-500 text-xs">过期时间</dt>
            <dd class="text-zinc-300 font-mono-token tabular-nums text-xs">{{ expiryDate }}</dd>
          </div>
        </dl>

        <div class="rounded-xl bg-white/[0.02] ring-1 ring-white/[0.05] p-3 overflow-hidden">
          <p class="text-[10px] text-zinc-500 tracking-wider uppercase mb-1">Token</p>
          <p class="font-mono-token text-xs text-zinc-300 truncate">{{ captured.token }}</p>
        </div>

        <div v-if="captured.clipboardOk" class="text-xs text-emerald-300 flex items-center gap-1.5">
          <Check class="w-3.5 h-3.5" />
          已复制到剪贴板
        </div>
        <div v-else class="text-xs text-amber-300 flex items-center gap-1.5">
          <AlertCircle class="w-3.5 h-3.5" />
          剪贴板写入失败，请点下方「复制」按钮
        </div>

        <!-- Invite code — first-time activation only. Punched-up styling
             because new users actually need to look here. -->
        <div class="rounded-xl bg-emerald-500/[0.05] ring-1 ring-emerald-500/25 p-4">
          <div class="flex items-center justify-between mb-2">
            <label class="text-xs text-emerald-300 font-medium inline-flex items-center gap-1.5">
              <Ticket class="w-3.5 h-3.5" />
              邀请码
            </label>
            <span class="text-[10px] text-zinc-500">可选 · 仅首次激活需要</span>
          </div>
          <input
            v-model="inviteCode"
            placeholder="ABC-DEF-XYZ9"
            autocapitalize="characters"
            autocomplete="off"
            class="w-full bg-zinc-950 ring-1 ring-emerald-500/30 focus:!ring-emerald-500/60 rounded-lg px-3 py-2.5 text-base font-mono-token tracking-[0.2em] text-center text-zinc-100 placeholder:text-zinc-700 focus-ring uppercase"
          />
          <p class="text-[10px] text-zinc-500 mt-1.5">已激活账号只是更新 token，留空即可</p>
        </div>

        <div class="grid grid-cols-2 gap-2">
          <button @click="copyAgain" class="bg-white/[0.04] hover:bg-white/[0.08] ring-1 ring-white/[0.05] text-zinc-200 text-sm font-medium py-2.5 rounded-lg transition-colors inline-flex items-center justify-center gap-1.5">
            <Copy class="w-3.5 h-3.5" />
            复制
          </button>
          <button @click="reset" class="bg-white/[0.04] hover:bg-white/[0.08] ring-1 ring-white/[0.05] text-zinc-200 text-sm font-medium py-2.5 rounded-lg transition-colors inline-flex items-center justify-center gap-1.5">
            <RotateCcw class="w-3.5 h-3.5" />
            再抓一次
          </button>
        </div>

        <button @click="openWangui" class="w-full bg-emerald-500 hover:bg-emerald-400 text-zinc-950 font-semibold py-2.5 rounded-xl transition-colors inline-flex items-center justify-center gap-2">
          <ExternalLink class="w-4 h-4" />
          打开 wangui 激活页
        </button>
      </section>

      <!-- Error -->
      <section v-else-if="phase === 'error'" class="space-y-4 pt-4">
        <div class="flex items-center gap-3">
          <div class="w-7 h-7 rounded-full bg-red-500/15 ring-1 ring-red-500/30 flex items-center justify-center">
            <AlertCircle class="w-4 h-4 text-red-400" />
          </div>
          <h2 class="text-lg font-bold">抓取失败</h2>
        </div>

        <div class="rounded-xl bg-red-500/[0.05] ring-1 ring-red-500/20 p-4">
          <p class="text-sm text-red-300 mb-1">{{ progressStep || '错误' }}</p>
          <p class="text-xs text-zinc-400 leading-relaxed break-words">{{ progressError || '未知错误' }}</p>
        </div>

        <button @click="reset" class="w-full bg-emerald-500 hover:bg-emerald-400 text-zinc-950 font-semibold py-2.5 rounded-xl transition-colors inline-flex items-center justify-center gap-2">
          <RotateCcw class="w-4 h-4" />
          重试
        </button>
      </section>
    </main>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
