<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  ArrowRight,
  AlertCircle,
  HelpCircle,
  Shield,
  Eye,
  Lock,
  UserCheck,
  X,
  Check,
  Ticket,
  KeyRound,
  Hash,
  ShieldCheck,
} from 'lucide-vue-next'
import Logo from '../components/Logo.vue'
import { api } from '../api'
import { useAuth } from '../stores/auth'
import { showToast } from '../lib/toast'

const router = useRouter()
const route = useRoute()
const auth = useAuth()

type Mode = 'login' | 'activate'
const mode = ref<Mode>('login')

// Login fields
const loginNumber = ref('')
const loginPin = ref('')

// Activate fields
const inviteCode = ref('')
const token = ref('')
const pinA = ref('')
const pinB = ref('')
const agreed = ref(localStorage.getItem('wangui:disclaimer') === 'yes')
const showDisclaimerDetail = ref(false)
const showHelp = ref(false)

// Disclaimer must be expanded for a few seconds before the user is allowed
// to tick the agreement box. If they've previously agreed (localStorage flag)
// we skip the gate.
const READ_DELAY_MS = 4000
const canCheckAgreement = ref(agreed.value)
const readCountdown = ref(0)
let readTickInterval: ReturnType<typeof setInterval> | null = null
let readUnlockTimer: ReturnType<typeof setTimeout> | null = null

function startReadGate() {
  if (canCheckAgreement.value) return
  if (readUnlockTimer) return // already counting
  readCountdown.value = Math.ceil(READ_DELAY_MS / 1000)
  readTickInterval = setInterval(() => {
    readCountdown.value = Math.max(0, readCountdown.value - 1)
  }, 1000)
  readUnlockTimer = setTimeout(() => {
    canCheckAgreement.value = true
    if (readTickInterval) {
      clearInterval(readTickInterval)
      readTickInterval = null
    }
    readUnlockTimer = null
  }, READ_DELAY_MS)
}

function toggleDisclaimer() {
  showDisclaimerDetail.value = !showDisclaimerDetail.value
  if (showDisclaimerDetail.value) startReadGate()
}

onBeforeUnmount(() => {
  if (readUnlockTimer) clearTimeout(readUnlockTimer)
  if (readTickInterval) clearInterval(readTickInterval)
})

const submitting = ref(false)
const error = ref<string | null>(null)

const disclaimerItems = [
  { icon: Shield, color: 'text-zinc-500 dark:text-zinc-400', text: '本工具非官方产品，使用风险由你自行承担。' },
  { icon: Eye, color: 'text-zinc-500 dark:text-zinc-400', text: '开启自动签到即授权服务器在每天 22:00–22:30 期间替你调用学校签到接口。' },
  { icon: X, color: 'text-red-400', text: '你必须保证签到时本人确实在校。不在校时仍让脚本签到等同于谎报位置，后果由你独自承担。' },
  { icon: Lock, color: 'text-zinc-500 dark:text-zinc-400', text: 'Token 在服务器以 AES-GCM 加密存储，PIN 用 bcrypt 哈希存储。' },
  { icon: UserCheck, color: 'text-zinc-500 dark:text-zinc-400', text: '本工具仅向受邀的个人提供，每张邀请码永久绑定到首次使用者。' },
]

const isPinValid = (p: string) => /^\d{4,6}$/.test(p)

const canSubmit = computed(() => {
  if (submitting.value) return false
  if (mode.value === 'login') {
    return loginNumber.value.trim().length > 0 && isPinValid(loginPin.value)
  }
  return (
    inviteCode.value.trim().length > 0 &&
    token.value.trim().length > 0 &&
    isPinValid(pinA.value) &&
    pinA.value === pinB.value &&
    agreed.value
  )
})

const pinMatchHint = computed(() => {
  if (mode.value !== 'activate') return ''
  if (!pinA.value || !pinB.value) return ''
  if (pinA.value === pinB.value) return ''
  return '两次输入的 PIN 不一致'
})

async function submit() {
  error.value = null
  if (!canSubmit.value) return
  submitting.value = true
  try {
    if (mode.value === 'login') {
      await api.login(loginNumber.value.trim(), loginPin.value)
      showToast('ok', '欢迎回来')
    } else {
      await api.activate(
        inviteCode.value.trim().toUpperCase(),
        token.value.trim(),
        pinA.value,
        true,
      )
      localStorage.setItem('wangui:disclaimer', 'yes')
      showToast('ok', '激活成功')
    }
    await auth.refresh()
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  } catch (e: any) {
    error.value = e.message || (mode.value === 'login' ? '登录失败' : '激活失败')
  } finally {
    submitting.value = false
  }
}

function switchTo(m: Mode) {
  if (mode.value === m) return
  mode.value = m
  error.value = null
}

// Restrict PIN inputs to digits only as user types.
function pinDigits(s: string): string {
  return s.replace(/\D/g, '').slice(0, 6)
}

onMounted(async () => {
  await auth.init()

  // tokengrab hands tokens off via URL fragment to keep them out of the
  // server's access log / Referer header. Format:
  //   #activate=<token>&code=<invite>  (URLSearchParams-style)
  const rawHash = window.location.hash.replace(/^#/, '')
  const params = rawHash ? new URLSearchParams(rawHash) : null
  const tok = params?.get('activate') || ''
  const code = params?.get('code') || ''

  // Wipe the hash so refresh doesn't repopulate, and so the token doesn't
  // linger in the URL bar after the user reads it. Do this before any
  // router navigation so the next page doesn't inherit the hash.
  if (rawHash) {
    history.replaceState(null, '', window.location.pathname + window.location.search)
  }

  if (auth.state.me) {
    // Already logged in.
    //   - With a fresh token in hand → user wants to refresh, not activate.
    //     Hand off to /account via sessionStorage (avoid putting the token
    //     in the URL where it lands in browser history).
    //   - Without → normal redirect to dashboard.
    if (tok) {
      sessionStorage.setItem('wangui:prefill_token', tok)
      router.replace({ path: '/account', query: { prefill: 'token' } })
    } else {
      router.replace('/')
    }
    return
  }

  // Not logged in: pre-fill the activate form.
  if (tok) {
    mode.value = 'activate'
    token.value = tok
    if (code) inviteCode.value = code
  }
})
</script>

<template>
  <div class="min-h-screen flex items-center justify-center px-4 py-10 relative">
    <div class="w-full max-w-md">
      <div class="flex flex-col items-center mb-8">
        <Logo :size="56" />
        <h1 class="text-3xl font-bold tracking-tight mt-5 text-center">
          勿外传
        </h1>
        <p class="text-sm text-zinc-500 mt-2">内部工具</p>
      </div>

      <div class="bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-2xl backdrop-blur-sm overflow-hidden">
        <!-- Tabs -->
        <div class="flex relative border-b border-black/[0.08] dark:border-white/[0.06]">
          <button
            @click="switchTo('login')"
            :class="mode === 'login' ? 'text-zinc-900 dark:text-zinc-100' : 'text-zinc-500 hover:text-zinc-500 dark:hover:text-zinc-400 dark:text-zinc-300'"
            class="flex-1 py-3.5 text-sm font-medium transition-colors relative"
          >
            登录
          </button>
          <button
            @click="switchTo('activate')"
            :class="mode === 'activate' ? 'text-zinc-900 dark:text-zinc-100' : 'text-zinc-500 hover:text-zinc-500 dark:hover:text-zinc-400 dark:text-zinc-300'"
            class="flex-1 py-3.5 text-sm font-medium transition-colors relative"
          >
            激活
          </button>
          <div
            class="absolute bottom-0 h-[2px] bg-emerald-400 transition-all duration-300 ease-out"
            :style="{ left: mode === 'login' ? '0%' : '50%', width: '50%' }"
          />
        </div>

        <!-- Login form -->
        <div v-if="mode === 'login'" class="p-6">
          <p class="text-sm text-zinc-500 dark:text-zinc-400 mb-5">已激活账号请用学号 + PIN 登录</p>

          <div class="space-y-4">
            <div>
              <label class="flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">
                <Hash class="w-3.5 h-3.5" />
                学号
              </label>
              <input
                v-model="loginNumber"
                placeholder="2521241111"
                inputmode="numeric"
                autocomplete="username"
                class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2.5 text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-300 dark:placeholder:text-zinc-700 focus-ring font-mono-token tracking-wider text-base"
              />
            </div>
            <div>
              <label class="flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">
                <ShieldCheck class="w-3.5 h-3.5" />
                PIN <span class="text-zinc-500 dark:text-zinc-600">(4–6 位数字)</span>
              </label>
              <input
                :value="loginPin"
                @input="(e: any) => (loginPin = pinDigits(e.target.value))"
                placeholder="••••"
                type="password"
                inputmode="numeric"
                autocomplete="current-password"
                maxlength="6"
                class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2.5 text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-300 dark:placeholder:text-zinc-700 focus-ring font-mono-token tracking-[0.4em] text-center text-base"
                @keyup.enter="submit"
              />
            </div>
          </div>

          <button
            @click="submit"
            :disabled="!canSubmit"
            class="mt-5 w-full inline-flex items-center justify-center gap-2 bg-emerald-500 hover:bg-emerald-400 disabled:bg-zinc-200 dark:disabled:bg-zinc-800 disabled:text-zinc-500 text-zinc-950 font-medium py-2.5 rounded-xl transition-all disabled:cursor-not-allowed"
          >
            <span>{{ submitting ? '登录中…' : '登录' }}</span>
            <ArrowRight v-if="!submitting" class="w-4 h-4" />
          </button>

          <div
            v-if="error"
            class="mt-3 flex items-start gap-2 px-3 py-2 rounded-lg bg-red-500/10 ring-1 ring-red-500/20 text-sm text-red-400"
          >
            <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
            <span>{{ error }}</span>
          </div>

          <button
            @click="switchTo('activate')"
            class="mt-5 w-full text-xs text-zinc-500 hover:text-emerald-400 transition-colors"
          >
            首次使用？前往激活 →
          </button>
        </div>

        <!-- Activate form -->
        <div v-else class="p-6">
          <p class="text-sm text-zinc-500 dark:text-zinc-400 mb-5">首次使用，用邀请码 + 学校 Token 激活，并设置登录 PIN</p>

          <div class="space-y-4">
            <div>
              <label class="flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">
                <Ticket class="w-3.5 h-3.5" />
                邀请码
              </label>
              <input
                v-model="inviteCode"
                placeholder="ABC-DEF-XYZ9"
                autocomplete="off"
                autocapitalize="characters"
                class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2.5 font-mono-token text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-300 dark:placeholder:text-zinc-700 focus-ring tracking-wider text-center text-base"
              />
            </div>

            <div>
              <label class="flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">
                <KeyRound class="w-3.5 h-3.5" />
                学校 Token
              </label>
              <textarea
                v-model="token"
                placeholder="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
                class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2.5 font-mono-token h-24 resize-none text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-300 dark:placeholder:text-zinc-700 focus-ring"
              />
              <button
                @click="showHelp = !showHelp"
                type="button"
                class="mt-1.5 flex items-center gap-1.5 text-[11px] text-zinc-500 hover:text-zinc-500 dark:hover:text-zinc-400 dark:text-zinc-300 transition-colors"
              >
                <HelpCircle class="w-3 h-3" />
                <span>怎么获取 Token？</span>
              </button>
              <Transition name="expand">
                <ol
                  v-if="showHelp"
                  class="mt-2 overflow-hidden text-[11px] text-zinc-500 dark:text-zinc-400 space-y-1.5 list-decimal list-inside leading-relaxed bg-white/70 dark:bg-zinc-950/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04] rounded-lg p-3"
                >
                  <li>跟管理员要 <code class="bg-zinc-200 dark:bg-zinc-800 px-1 rounded text-zinc-700 dark:text-zinc-300">wangui.exe</code>（配套抓取工具）。</li>
                  <li>电脑微信打开晚归签到入口、正常登录。</li>
                  <li>双击 <code class="text-zinc-700 dark:text-zinc-300">wangui.exe</code> → 点「开始抓取」→ 回微信按 <code class="bg-zinc-200 dark:bg-zinc-800 px-1 rounded text-zinc-700 dark:text-zinc-300">Ctrl+R</code> 刷新一下。</li>
                  <li>Token 自动复制到剪贴板，回到本页粘贴到上面的输入框即可。</li>
                </ol>
              </Transition>
            </div>

            <!-- PIN setup -->
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">
                  <ShieldCheck class="w-3.5 h-3.5" />
                  设置 PIN
                </label>
                <input
                  :value="pinA"
                  @input="(e: any) => (pinA = pinDigits(e.target.value))"
                  placeholder="4–6 位"
                  type="password"
                  inputmode="numeric"
                  autocomplete="new-password"
                  maxlength="6"
                  class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2.5 text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-300 dark:placeholder:text-zinc-700 focus-ring font-mono-token tracking-[0.4em] text-center"
                />
              </div>
              <div>
                <label class="flex items-center gap-1.5 text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">
                  <ShieldCheck class="w-3.5 h-3.5" />
                  再输一次
                </label>
                <input
                  :value="pinB"
                  @input="(e: any) => (pinB = pinDigits(e.target.value))"
                  placeholder="重复"
                  type="password"
                  inputmode="numeric"
                  maxlength="6"
                  class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2.5 text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-300 dark:placeholder:text-zinc-700 focus-ring font-mono-token tracking-[0.4em] text-center"
                />
              </div>
            </div>
            <p
              v-if="pinMatchHint"
              class="-mt-2 text-[11px] text-amber-400 inline-flex items-center gap-1"
            >
              <AlertCircle class="w-3 h-3" />
              {{ pinMatchHint }}
            </p>

            <!-- Disclaimer inline -->
            <div class="rounded-lg bg-white/50 dark:bg-zinc-950/50 ring-1 ring-black/[0.05] dark:ring-white/[0.04] p-3">
              <button
                type="button"
                @click="toggleDisclaimer"
                class="w-full flex items-center justify-between text-xs text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors"
              >
                <span class="inline-flex items-center gap-1.5">
                  <AlertCircle class="w-3.5 h-3.5 text-amber-400" />
                  使用须知
                </span>
                <span class="text-[10px] text-zinc-500 dark:text-zinc-600">{{ showDisclaimerDetail ? '收起 ↑' : '展开 ↓' }}</span>
              </button>

              <Transition name="expand">
                <ul v-if="showDisclaimerDetail" class="mt-3 space-y-2.5 overflow-hidden">
                  <li v-for="(it, i) in disclaimerItems" :key="i" class="flex gap-2.5">
                    <component :is="it.icon" class="w-3.5 h-3.5 shrink-0 mt-0.5" :class="it.color" />
                    <p class="text-[11px] text-zinc-500 dark:text-zinc-400 leading-relaxed">{{ it.text }}</p>
                  </li>
                </ul>
              </Transition>

              <label
                class="mt-3 flex items-center gap-2 select-none"
                :class="canCheckAgreement ? 'cursor-pointer' : 'cursor-not-allowed opacity-60'"
              >
                <input
                  type="checkbox"
                  v-model="agreed"
                  :disabled="!canCheckAgreement"
                  class="sr-only peer"
                />
                <div class="w-4 h-4 rounded border-2 border-zinc-600 peer-checked:border-emerald-500 peer-checked:bg-emerald-500 transition-colors flex items-center justify-center shrink-0">
                  <Check v-if="agreed" class="w-2.5 h-2.5 text-zinc-950" :stroke-width="3" />
                </div>
                <span class="text-xs text-zinc-700 dark:text-zinc-300">
                  <template v-if="canCheckAgreement">我已阅读并同意</template>
                  <template v-else-if="showDisclaimerDetail">阅读中… ({{ readCountdown }}s)</template>
                  <template v-else>请先展开「使用须知」并阅读完</template>
                </span>
              </label>
            </div>
          </div>

          <button
            @click="submit"
            :disabled="!canSubmit"
            class="mt-5 w-full inline-flex items-center justify-center gap-2 bg-emerald-500 hover:bg-emerald-400 disabled:bg-zinc-200 dark:disabled:bg-zinc-800 disabled:text-zinc-500 text-zinc-950 font-medium py-2.5 rounded-xl transition-all disabled:cursor-not-allowed"
          >
            <span>{{ submitting ? '激活中…' : '激活账号' }}</span>
            <ArrowRight v-if="!submitting" class="w-4 h-4" />
          </button>

          <div
            v-if="error"
            class="mt-3 flex items-start gap-2 px-3 py-2 rounded-lg bg-red-500/10 ring-1 ring-red-500/20 text-sm text-red-400"
          >
            <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
            <span>{{ error }}</span>
          </div>

          <button
            @click="switchTo('login')"
            class="mt-5 w-full text-xs text-zinc-500 hover:text-emerald-400 transition-colors"
          >
            ← 已激活账号？返回登录
          </button>
        </div>
      </div>

      <p class="text-center text-[11px] text-zinc-500 dark:text-zinc-600 mt-5">
        登录限速 5 次 / 分钟 · Token 加密存储 · PIN bcrypt 哈希
      </p>
    </div>
  </div>
</template>

<style scoped>
.expand-enter-active, .expand-leave-active { transition: all 0.25s ease; max-height: 260px; }
.expand-enter-from, .expand-leave-to { max-height: 0; opacity: 0; overflow: hidden; }
</style>
