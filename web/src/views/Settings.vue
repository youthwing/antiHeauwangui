<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import {
  Power,
  Building2,
  Smartphone,
  Save,
  AlertTriangle,
  Clock,
  RotateCcw,
  MapPin,
  Mail,
  Bell,
  Send,
  Eye,
  EyeOff,
  HelpCircle,
  Network,
  Activity,
  Shuffle,
} from 'lucide-vue-next'
import type { Settings, Dorm, ProxyTestResult, ProxyNodesResult } from '../types'
import { useAuth } from '../stores/auth'
import { api } from '../api'
import { showToast } from '../lib/toast'

const auth = useAuth()

const form = reactive<Settings>({
  autoSign: true,
  dormId: null,
  latitude: 0,
  longitude: 0,
  address: '',
  city: '',
  road: '',
  poi: '',
  deviceModel: 'iPhone',
  deviceSystem: 'iOS',
  triggerMinute: 2,
  jitterSec: 180,
  retryCount: 3,
  retryGapMin: 5,
  savedLocations: [],
  notifyEmail: '',
  notifyEnabled: false,
  signDays: 127,
})

// Server酱 lives outside `form` because the input is write-only: the server
// never echoes the SendKey back, so we keep a transient ref and only
// submit it when the user has typed something fresh.
const serverChanKey = ref('')
const serverChanEnabled = ref(false)
const serverChanKeySet = ref(false)
const showServerChanKey = ref(false)
const testingServerChan = ref(false)
const showServerChanFaq = ref(false)

const proxyEnabled = ref(false)
const proxyScheme = ref<'socks5' | 'http' | 'https'>('socks5')
const proxyHost = ref('')
const proxyPort = ref<number | null>(null)
const proxyUsername = ref('')
const proxyPassword = ref('')
const proxyPasswordSet = ref(false)
const showProxyPassword = ref(false)
const testingProxy = ref(false)
const proxyTestResult = ref<ProxyTestResult | null>(null)
const proxyNodes = ref<ProxyNodesResult | null>(null)
const loadingProxyNodes = ref(false)
const switchingProxyNode = ref(false)
const selectedProxyNode = ref('')

// signDays bitmask helpers. bit 0 = Mon … bit 6 = Sun.
const PRESETS = {
  every: 127, // 0b1111111
  weekday: 31, // 0b0011111
  weekend: 96, // 0b1100000
} as const
const WEEK_LABELS = ['一', '二', '三', '四', '五', '六', '日']
function hasDay(mask: number, bit: number) {
  return (mask & (1 << bit)) !== 0
}
function toggleDay(bit: number) {
  form.signDays = (form.signDays & 0x7f) ^ (1 << bit)
}
function applyPreset(mask: number) {
  form.signDays = mask
}
const showScheduleFaq = ref(false)
const activePreset = computed<'every' | 'weekday' | 'weekend' | 'custom'>(() => {
  const m = form.signDays & 0x7f
  if (m === PRESETS.every) return 'every'
  if (m === PRESETS.weekday) return 'weekday'
  if (m === PRESETS.weekend) return 'weekend'
  return 'custom'
})

const dorms = ref<Dorm[]>([])
const loadingDorms = ref(false)

function hydrate(s: Settings) {
  form.autoSign = s.autoSign
  form.dormId = s.dormId
  form.latitude = s.latitude
  form.longitude = s.longitude
  form.address = s.address
  form.city = s.city
  form.road = s.road
  form.poi = s.poi
  form.deviceModel = s.deviceModel
  form.deviceSystem = s.deviceSystem
  form.triggerMinute = s.triggerMinute
  form.jitterSec = s.jitterSec
  form.retryCount = s.retryCount
  form.retryGapMin = s.retryGapMin
  form.savedLocations = Array.isArray(s.savedLocations) ? [...s.savedLocations] : []
  form.notifyEmail = s.notifyEmail || ''
  form.notifyEnabled = !!s.notifyEnabled
  serverChanEnabled.value = !!s.serverChanEnabled
  serverChanKeySet.value = !!s.serverChanKeySet
  serverChanKey.value = '' // reset on hydrate so saved key isn't re-sent
  proxyEnabled.value = !!s.proxyEnabled
  proxyScheme.value = s.proxyScheme || 'socks5'
  proxyHost.value = s.proxyHost || ''
  proxyPort.value = typeof s.proxyPort === 'number' && s.proxyPort > 0 ? s.proxyPort : null
  proxyUsername.value = s.proxyUsername || ''
  proxyPasswordSet.value = !!s.proxyPasswordSet
  proxyPassword.value = ''
  // Server returns 0 for "never sign" but UI keeps the form's default-127
  // so an empty value doesn't accidentally wipe the schedule. We trust the
  // server's value here, treating 0 as a real "no days selected" state.
  form.signDays = typeof s.signDays === 'number' ? s.signDays : 127
}

const saving = ref(false)
const savingDorm = ref(false)

async function loadDorms() {
  loadingDorms.value = true
  try {
    dorms.value = await api.dorms()
  } catch {
    dorms.value = []
  } finally {
    loadingDorms.value = false
  }
}

onMounted(async () => {
  await auth.init()
  if (auth.state.me) hydrate(auth.state.me.settings)
  await loadDorms()
  await loadProxyNodes()
})

watch(
  () => auth.state.me,
  m => m && hydrate(m.settings),
)

async function saveAll() {
  saving.value = true
  try {
    // Save everything except dormId (handled separately) — and explicitly
    // skip lat/lng/address/city/road/poi when the user has a dorm selected
    // (those are the dorm's, not user-editable).
    const payload: Partial<Settings> = {
      autoSign: form.autoSign,
      deviceModel: form.deviceModel,
      deviceSystem: form.deviceSystem,
      triggerMinute: form.triggerMinute,
      jitterSec: form.jitterSec,
      retryCount: form.retryCount,
      retryGapMin: form.retryGapMin,
      notifyEmail: form.notifyEmail.trim(),
      notifyEnabled: form.notifyEnabled,
      serverChanEnabled: serverChanEnabled.value,
      proxyEnabled: proxyEnabled.value,
      proxyScheme: proxyScheme.value,
      proxyHost: proxyHost.value.trim(),
      proxyPort: proxyPort.value || 0,
      proxyUsername: proxyUsername.value.trim(),
      signDays: form.signDays & 0x7f,
    }
    // Only send the SendKey if the user typed something fresh — empty means
    // "keep what's already saved" (same as the SMTP password convention).
    const sck = serverChanKey.value.trim()
    if (sck) payload.serverChanKey = sck
    const proxyPw = proxyPassword.value
    if (proxyPw) payload.proxyPassword = proxyPw
    await api.updateSettings(payload)
    showToast('ok', '配置已保存')
    if (sck) {
      serverChanKey.value = ''
      serverChanKeySet.value = true
    }
    if (proxyPw) {
      proxyPassword.value = ''
      proxyPasswordSet.value = true
    }
    await auth.refresh()
  } catch (e: any) {
    showToast('err', e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function testProxy() {
  if (testingProxy.value) return
  testingProxy.value = true
  proxyTestResult.value = null
  try {
    const saved = await api.updateSettings({
      proxyEnabled: proxyEnabled.value,
      proxyScheme: proxyScheme.value,
      proxyHost: proxyHost.value.trim(),
      proxyPort: proxyPort.value || 0,
      proxyUsername: proxyUsername.value.trim(),
      ...(proxyPassword.value ? { proxyPassword: proxyPassword.value } : {}),
    })
    hydrate(saved)
    const result = await api.testProxy()
    proxyTestResult.value = result
    if (result.ok) {
      showToast('ok', `代理可用，耗时 ${result.elapsedMs} ms`)
    } else {
      showToast('err', result.schoolMessage || '代理测试失败')
    }
    await auth.refresh()
  } catch (e: any) {
    showToast('err', e.message || '代理测试失败')
  } finally {
    testingProxy.value = false
  }
}

async function loadProxyNodes() {
  loadingProxyNodes.value = true
  try {
    const res = await api.proxyNodes()
    proxyNodes.value = res
    selectedProxyNode.value = res.current || ''
  } catch (e: any) {
    proxyNodes.value = {
      available: false,
      group: 'Proxies',
      message: e.message || '读取节点失败',
      nodes: [],
      shared: true,
    }
  } finally {
    loadingProxyNodes.value = false
  }
}

async function selectProxyNode() {
  if (!selectedProxyNode.value || switchingProxyNode.value) return
  switchingProxyNode.value = true
  try {
    const res = await api.selectProxyNode(selectedProxyNode.value)
    proxyNodes.value = res
    selectedProxyNode.value = res.current || selectedProxyNode.value
    showToast('ok', `已切换到 ${selectedProxyNode.value}`)
  } catch (e: any) {
    showToast('err', e.message || '切换节点失败')
  } finally {
    switchingProxyNode.value = false
  }
}

async function autoSelectProxyNode() {
  if (switchingProxyNode.value) return
  switchingProxyNode.value = true
  try {
    const res = await api.autoSelectProxyNode()
    proxyNodes.value = res
    selectedProxyNode.value = res.current || ''
    showToast('ok', res.picked ? `已选择 ${res.picked}` : '已自动选择节点')
  } catch (e: any) {
    showToast('err', e.message || '自动选择失败')
  } finally {
    switchingProxyNode.value = false
  }
}

async function testServerChanPush() {
  if (testingServerChan.value) return
  testingServerChan.value = true
  try {
    await api.testServerChan()
    showToast('ok', '测试推送已发送，请查看微信')
  } catch (e: any) {
    showToast('err', e.message || '推送失败')
  } finally {
    testingServerChan.value = false
  }
}

async function toggleAutoSign() {
  form.autoSign = !form.autoSign
  try {
    await api.updateSettings({ autoSign: form.autoSign })
    showToast('ok', form.autoSign ? '自动签到已开启' : '自动签到已关闭')
    await auth.refresh()
  } catch (e: any) {
    showToast('err', e.message || '保存失败')
    form.autoSign = !form.autoSign
  }
}

async function selectDorm(id: number | null) {
  savingDorm.value = true
  try {
    await api.updateSettings({ dormId: id ?? 0 } as any)
    showToast('ok', id ? '宿舍楼已绑定' : '已解除绑定')
    await auth.refresh()
  } catch (e: any) {
    showToast('err', e.message || '保存失败')
  } finally {
    savingDorm.value = false
  }
}

const currentDorm = computed(() =>
  form.dormId ? dorms.value.find(d => d.id === form.dormId) || null : null,
)

const previewSchedule = computed(() => {
  const start = 22 * 60 + form.triggerMinute
  const list = [start]
  for (let i = 0; i < form.retryCount; i++) {
    list.push(start + form.retryGapMin * (i + 1))
  }
  return list
    .filter(m => m < 22 * 60 + 30)
    .map(m => `${Math.floor(m / 60)}:${String(m % 60).padStart(2, '0')}`)
})
</script>

<template>
  <div class="space-y-3">
    <header class="mb-2">
      <h1 class="text-2xl font-bold tracking-tight">配置</h1>
      <p class="text-sm text-zinc-500 mt-1">自动签到的行为、打卡位置和设备信息。</p>
    </header>

    <!-- Section 1: 自动签到 -->
    <section class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-4 gap-3">
        <div class="flex items-center gap-2">
          <Power class="w-4 h-4 text-zinc-500" />
          <h2 class="text-base font-semibold text-[#161b22] dark:text-zinc-200">自动签到</h2>
        </div>
        <button
          @click="toggleAutoSign"
          :class="form.autoSign ? 'bg-red-500' : 'bg-zinc-300 dark:bg-zinc-700'"
          class="relative w-11 h-6 rounded-full transition-colors"
        >
          <span
            :class="form.autoSign ? 'translate-x-5' : 'translate-x-0.5'"
            class="absolute top-0.5 left-0 w-5 h-5 bg-white rounded-full shadow-md transition-transform"
          />
        </button>
      </div>

      <p class="text-xs text-zinc-500 leading-relaxed mb-5">
        开启后服务器每天在签到窗口内自动尝试。
        <span class="text-zinc-500 dark:text-zinc-600">请确保实际在校。</span>
      </p>

      <!-- Sign-day schedule (which weekdays to auto-sign on) -->
      <div class="mb-5 p-3 rounded-lg bg-white/70 dark:bg-[#0d1117]/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04]">
        <div class="flex items-center justify-between mb-3 gap-2">
          <span class="text-[10px] text-zinc-500 tracking-wide uppercase">签到日期</span>
          <div class="flex gap-1">
            <button
              type="button"
              @click="applyPreset(PRESETS.every)"
              :class="activePreset === 'every'
                ? 'bg-red-500/15 text-red-300 ring-1 ring-red-500/30'
                : 'bg-zinc-200/60 dark:bg-zinc-800/60 text-zinc-500 dark:text-zinc-400 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-[#161b22] dark:hover:text-zinc-200'"
              class="text-[11px] px-2.5 py-1 rounded-md transition-colors"
            >每天</button>
            <button
              type="button"
              @click="applyPreset(PRESETS.weekday)"
              :class="activePreset === 'weekday'
                ? 'bg-red-500/15 text-red-300 ring-1 ring-red-500/30'
                : 'bg-zinc-200/60 dark:bg-zinc-800/60 text-zinc-500 dark:text-zinc-400 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-[#161b22] dark:hover:text-zinc-200'"
              class="text-[11px] px-2.5 py-1 rounded-md transition-colors"
            >工作日</button>
            <button
              type="button"
              @click="applyPreset(PRESETS.weekend)"
              :class="activePreset === 'weekend'
                ? 'bg-red-500/15 text-red-300 ring-1 ring-red-500/30'
                : 'bg-zinc-200/60 dark:bg-zinc-800/60 text-zinc-500 dark:text-zinc-400 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-[#161b22] dark:hover:text-zinc-200'"
              class="text-[11px] px-2.5 py-1 rounded-md transition-colors"
            >周末</button>
          </div>
        </div>

        <div class="grid grid-cols-7 gap-1.5">
          <button
            v-for="(label, i) in WEEK_LABELS"
            :key="i"
            type="button"
            @click="toggleDay(i)"
            :class="hasDay(form.signDays, i)
              ? 'bg-red-500 text-[#0d1117] ring-1 ring-red-400 shadow-[0_0_0_2px_rgba(16,185,129,0.15)]'
              : 'bg-zinc-200/60 dark:bg-zinc-800/60 text-zinc-500 dark:text-zinc-500 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-[#161b22] dark:hover:text-zinc-200'"
            class="py-1.5 rounded-md text-xs font-medium transition-all"
          >{{ label }}</button>
        </div>

        <p v-if="(form.signDays & 0x7f) === 0" class="text-[10px] text-amber-400 mt-2 inline-flex items-center gap-1">
          <AlertTriangle class="w-3 h-3" />
          一天都没选 — 自动签到不会触发
        </p>
        <p v-else class="text-[10px] text-zinc-500 dark:text-zinc-600 mt-2">
          只在选中的周几尝试签到；其他日子静默跳过
        </p>
      </div>

      <!-- Schedule FAQ — explains what these 4 inputs actually do.
           Collapsed by default; users almost never touch these so they
           don't need to read it. -->
      <div class="mb-3">
        <button
          type="button"
          @click="showScheduleFaq = !showScheduleFaq"
          class="text-[11px] text-zinc-500 dark:text-zinc-400 hover:text-red-400 transition-colors inline-flex items-center gap-1"
        >
          <span>ⓘ 这 4 个数字啥意思？</span>
          <span class="text-zinc-600">{{ showScheduleFaq ? '收起' : '展开看说明' }}</span>
        </button>
        <Transition name="expand">
          <div v-if="showScheduleFaq" class="mt-2 rounded-lg bg-sky-500/[0.05] ring-1 ring-sky-500/20 p-3 text-[11px] text-zinc-700 dark:text-zinc-300 leading-relaxed space-y-2.5 overflow-hidden">
            <p>
              系统每天 22:00 整点醒来，但不会让所有用户都在 22:00:00 这一秒同时签到 —— 那样 5 个学号同 IP 集中发请求会很显眼，且每天看你都"卡在 22:00 没结果"也会焦虑。下面 4 个参数控制具体的延迟。
            </p>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-2">
              <div>
                <p class="text-red-300 font-medium">首次触发分钟</p>
                <p class="text-zinc-500 mt-0.5">
                  从 22:00 起再等几分钟。<strong>激活账号时系统已为你随机分配过一次（0–27 分钟）</strong>，每个用户不同。你的当前值意味着每天大约 22:{{ String(form.triggerMinute).padStart(2, '0') }} 左右签。
                </p>
              </div>
              <div>
                <p class="text-red-300 font-medium">抖动秒数</p>
                <p class="text-zinc-500 mt-0.5">
                  在上面那个分钟基础上，再随机往后推 0–{{ form.jitterSec }} 秒。每天具体的"秒数"都不同，避免每天精确到秒的规律。
                </p>
              </div>
              <div>
                <p class="text-red-300 font-medium">重试次数</p>
                <p class="text-zinc-500 mt-0.5">
                  第一次签失败（网络问题 / 学校 API 抽风）后，再试几次。默认 3 次，4 次机会总共。
                </p>
              </div>
              <div>
                <p class="text-red-300 font-medium">重试间隔</p>
                <p class="text-zinc-500 mt-0.5">
                  两次重试之间等几分钟。默认 5 分钟，配合"重试 3 次" = 最多覆盖 20 分钟（接近 22:30 截止）。
                </p>
              </div>
            </div>
            <p class="text-zinc-500 mt-2">
              <strong class="text-zinc-400">大白话总结</strong>：你的预定签到时刻 ≈ <span class="font-mono-token text-red-300">22:{{ String(form.triggerMinute).padStart(2, '0') }}</span>，实际可能再往后 0–{{ form.jitterSec }} 秒。如果你没动过这些参数，<strong>什么都不用改，默认很合理</strong>。
            </p>
          </div>
        </Transition>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">
            首次触发分钟 (22:00 之后)
          </label>
          <input
            v-model.number="form.triggerMinute"
            type="number"
            min="0" max="29"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-[#161b22] dark:text-zinc-200"
          />
          <p class="text-[10px] text-zinc-500 dark:text-zinc-600 mt-1">22:00 后多少分钟开始 · 0–29</p>
        </div>
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">
            抖动秒数 (0–600)
          </label>
          <input
            v-model.number="form.jitterSec"
            type="number"
            min="0" max="600"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-[#161b22] dark:text-zinc-200"
          />
          <p class="text-[10px] text-zinc-500 dark:text-zinc-600 mt-1">在上面时刻再加 0~N 秒随机</p>
        </div>
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">
            重试次数 (0–5)
          </label>
          <input
            v-model.number="form.retryCount"
            type="number"
            min="0" max="5"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-[#161b22] dark:text-zinc-200"
          />
          <p class="text-[10px] text-zinc-500 dark:text-zinc-600 mt-1">失败后再试几次 · 默认 3</p>
        </div>
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">
            重试间隔 (1–15 分钟)
          </label>
          <input
            v-model.number="form.retryGapMin"
            type="number"
            min="1" max="15"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-[#161b22] dark:text-zinc-200"
          />
          <p class="text-[10px] text-zinc-500 dark:text-zinc-600 mt-1">两次重试之间等几分钟</p>
        </div>
      </div>

      <div class="mt-5 p-3 rounded-lg bg-white/70 dark:bg-[#0d1117]/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04]">
        <div class="flex items-center gap-2 mb-2">
          <Clock class="w-3.5 h-3.5 text-zinc-500" />
          <span class="text-[10px] text-zinc-500 tracking-wide uppercase">本配置下的尝试时刻</span>
        </div>
        <div class="flex flex-wrap gap-1.5">
          <span
            v-for="(t, i) in previewSchedule"
            :key="t"
            class="px-2 py-0.5 rounded text-xs font-mono-token tabular-nums"
            :class="i === 0 ? 'bg-red-500/15 text-red-300 ring-1 ring-red-500/30' : 'bg-zinc-200 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400'"
          >
            {{ t }}<span v-if="i === 0" class="ml-1 text-[9px] opacity-70">主</span>
          </span>
        </div>
        <p class="text-[10px] text-zinc-500 dark:text-zinc-600 mt-2">签到窗口 22:00–22:30，超出窗口的重试会被跳过</p>
      </div>
    </section>

    <!-- Section 2: 我的宿舍楼 -->
    <section class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center gap-2 mb-4">
        <Building2 class="w-4 h-4 text-zinc-500" />
        <h2 class="text-sm font-semibold text-zinc-700 dark:text-zinc-300">我的宿舍楼</h2>
      </div>

      <p class="text-xs text-zinc-500 leading-relaxed mb-4">
        从管理员预设的列表中选你的宿舍楼。每个宿舍楼的坐标都已经精确标定。
      </p>

      <div class="space-y-3">
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">
            选择宿舍楼
          </label>
          <div class="relative">
            <select
              :value="form.dormId ?? ''"
              @change="(e: any) => selectDorm(e.target.value ? Number(e.target.value) : null)"
              :disabled="savingDorm || loadingDorms || dorms.length === 0"
              class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2.5 text-sm focus-ring text-[#161b22] dark:text-zinc-200 appearance-none cursor-pointer disabled:opacity-50"
            >
              <option value="">
                {{ dorms.length === 0 ? '管理员还没有添加宿舍楼' : '— 请选择 —' }}
              </option>
              <option v-for="d in dorms" :key="d.id" :value="d.id">
                {{ d.name }}
              </option>
            </select>
          </div>
        </div>

        <!-- Current selection card -->
        <div
          v-if="currentDorm"
          class="rounded-lg bg-red-500/[0.07] ring-1 ring-red-500/25 p-4"
        >
          <div class="flex items-start gap-3">
            <MapPin class="w-4 h-4 text-red-400 shrink-0 mt-0.5" />
            <div class="min-w-0 flex-1">
              <p class="text-sm font-medium text-red-200">{{ currentDorm.name }}</p>
              <p class="text-xs text-red-400/70 mt-1 break-all">
                {{ currentDorm.address || '未配置地址' }}
              </p>
              <p class="text-[10px] text-red-400/50 mt-1.5 font-mono-token tabular-nums">
                {{ currentDorm.latitude.toFixed(6) }}, {{ currentDorm.longitude.toFixed(6) }}
                <span class="ml-1 text-red-400/30">(WGS84)</span>
              </p>
            </div>
          </div>
        </div>
        <div
          v-else-if="dorms.length > 0"
          class="flex items-center gap-2 px-3 py-2 rounded-lg bg-amber-500/[0.07] ring-1 ring-amber-500/25 text-xs text-amber-300"
        >
          <AlertTriangle class="w-3.5 h-3.5 shrink-0" />
          未选宿舍楼，自动签到将无法工作
        </div>
        <div
          v-else
          class="flex items-center gap-2 px-3 py-2 rounded-lg bg-zinc-200/30 dark:bg-zinc-800/30 ring-1 ring-black/[0.05] dark:ring-white/[0.04] text-xs text-zinc-500 dark:text-zinc-400"
        >
          <AlertTriangle class="w-3.5 h-3.5 shrink-0 text-zinc-500" />
          联系管理员添加你的宿舍楼到列表
        </div>
      </div>
    </section>

    <!-- Section 3: 邮件通知 -->
    <section class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-4 gap-3">
        <div class="flex items-center gap-2">
          <Mail class="w-4 h-4 text-zinc-500" />
          <h2 class="text-base font-semibold text-[#161b22] dark:text-zinc-200">邮件通知</h2>
        </div>
        <button
          @click="form.notifyEnabled = !form.notifyEnabled"
          :class="form.notifyEnabled ? 'bg-red-500' : 'bg-zinc-300 dark:bg-zinc-700'"
          class="relative w-11 h-6 rounded-full transition-colors"
        >
          <span
            :class="form.notifyEnabled ? 'translate-x-5' : 'translate-x-0.5'"
            class="absolute top-0.5 left-0 w-5 h-5 bg-white rounded-full shadow-md transition-transform"
          />
        </button>
      </div>
      <p class="text-xs text-zinc-500 leading-relaxed mb-3">
        签到完成（成功 / 失败）时发邮件到你下面填的邮箱。需要管理员在后台先配置 SMTP。
      </p>
      <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">接收邮箱</label>
      <input
        v-model="form.notifyEmail"
        type="email"
        placeholder="you@example.com"
        :disabled="!form.notifyEnabled"
        class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-[#161b22] dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
      />
      <p class="text-[11px] text-zinc-500 mt-2">
        只在自动签到的「最终结果」时发一封；手动「立即签到」不发邮件。
      </p>
    </section>

    <!-- Section 3.5: Server酱 微信推送 -->
    <section class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-3 gap-3">
        <div class="flex items-center gap-2">
          <Bell class="w-4 h-4 text-zinc-500" />
          <h2 class="text-base font-semibold text-[#161b22] dark:text-zinc-200">Server 酱 (微信推送)</h2>
        </div>
        <button
          @click="serverChanEnabled = !serverChanEnabled"
          :class="serverChanEnabled ? 'bg-red-500' : 'bg-zinc-300 dark:bg-zinc-700'"
          class="relative w-11 h-6 rounded-full transition-colors shrink-0"
        >
          <span
            :class="serverChanEnabled ? 'translate-x-5' : 'translate-x-0.5'"
            class="absolute top-0.5 left-0 w-5 h-5 bg-white rounded-full shadow-md transition-transform"
          />
        </button>
      </div>
      <p class="text-xs text-zinc-500 leading-relaxed mb-3">
        把签到结果和 <strong>Token 即将过期（剩 2 天时）</strong>直接推到你的微信。需要你自己在 Server 酱注册并拿到 SendKey。
      </p>

      <!-- FAQ collapsible -->
      <button
        type="button"
        @click="showServerChanFaq = !showServerChanFaq"
        class="inline-flex items-center gap-1.5 text-[11px] text-zinc-500 hover:text-[#161b22] dark:hover:text-zinc-200 transition-colors mb-2"
      >
        <HelpCircle class="w-3.5 h-3.5" />
        {{ showServerChanFaq ? '收起说明' : '什么是 Server 酱 / SendKey 在哪拿？' }}
      </button>
      <Transition name="expand">
        <div
          v-if="showServerChanFaq"
          class="rounded-lg bg-zinc-100 dark:bg-[#0d1117]/50 ring-1 ring-black/[0.06] dark:ring-white/[0.04] p-3 mb-3 text-[12px] text-zinc-600 dark:text-zinc-400 leading-relaxed space-y-2"
        >
          <p>
            <strong class="text-[#161b22] dark:text-zinc-200">Server 酱（方糖）</strong> 是一个免费的「程序→微信」推送服务。配置后，antiWG 在
            发生事件时（签到成功 / 失败 / Token 快过期）会调它的接口，你的微信就能立即收到通知。
          </p>
          <ol class="list-decimal pl-5 space-y-1">
            <li>访问 <code class="bg-white/70 dark:bg-[#161b22]/70 px-1 rounded font-mono-token">sct.ftqq.com</code>，用微信扫码登录</li>
            <li>登录后在「SendKey」页面看到形如 <code class="bg-white/70 dark:bg-[#161b22]/70 px-1 rounded font-mono-token">SCT123...AbCdEf</code> 的字符串</li>
            <li>把这串 SendKey 粘到下面输入框 → 打开开关 → 保存配置 → 点「发测试推送」</li>
            <li>微信收到「Server 酱测试推送」即成功</li>
          </ol>
          <p>
            <strong>免费版每天 5 条</strong>，正常使用足够（签到 1 条 + 可能的提醒 1 条）。SCT 前缀的 key 用免费的 sctapi.ftqq.com；
            sctp 前缀的 key 自动走 Server 酱³ 的 push.ft07.com。
          </p>
          <p>
            <strong class="text-[#161b22] dark:text-zinc-200">隐私</strong>：SendKey 等同于「允许任何人给你的微信发消息」，
            不要分享给别人。antiWG 把它<strong>加密存储</strong>，从不在网页上回显，只在发送时取出使用。
          </p>
        </div>
      </Transition>

      <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1 flex items-center justify-between">
        <span>SendKey</span>
        <span
          v-if="serverChanKeySet && !serverChanKey"
          class="text-red-600 dark:text-red-400 normal-case tracking-normal"
        >
          ✓ 已设置，留空保持不变
        </span>
      </label>
      <div class="relative">
        <input
          v-model="serverChanKey"
          :type="showServerChanKey ? 'text' : 'password'"
          :placeholder="serverChanKeySet ? '保持不变（输入新值才覆盖）' : 'SCT... 或 sctp...'"
          :disabled="!serverChanEnabled && !serverChanKeySet"
          autocomplete="off"
          class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 pr-10 text-sm focus-ring text-[#161b22] dark:text-zinc-200 font-mono-token disabled:opacity-50"
        />
        <button
          @click="showServerChanKey = !showServerChanKey"
          type="button"
          class="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 text-zinc-500 hover:text-[#161b22] dark:hover:text-zinc-100"
        >
          <component :is="showServerChanKey ? EyeOff : Eye" class="w-3.5 h-3.5" />
        </button>
      </div>
      <p class="text-[11px] text-zinc-500 mt-2">
        会推送的事件：自动签到的<strong>最终结果</strong>、<strong>Token 剩 2 天</strong>的提醒。手动「立即签到」不推送，避免刷屏。
      </p>

      <div class="mt-3 flex justify-end">
        <button
          type="button"
          @click="testServerChanPush"
          :disabled="testingServerChan || !serverChanKeySet"
          :title="serverChanKeySet ? '使用已保存的 SendKey 发一条' : '请先保存 SendKey'"
          class="inline-flex items-center gap-1.5 bg-sky-500/15 hover:bg-sky-500/25 disabled:opacity-40 disabled:cursor-not-allowed ring-1 ring-sky-500/30 text-blue-700 dark:text-blue-300 text-xs font-medium px-3 py-1.5 rounded-lg transition-colors"
        >
          <Send class="w-3 h-3" :class="testingServerChan ? 'wangui-spin' : ''" />
          {{ testingServerChan ? '推送中…' : '发测试推送' }}
        </button>
      </div>
    </section>

    <!-- Section 3.7: 代理出口 -->
    <section class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-3 gap-3">
        <div class="flex items-center gap-2">
          <Network class="w-4 h-4 text-zinc-500" />
          <h2 class="text-base font-semibold text-[#161b22] dark:text-zinc-200">请求代理</h2>
        </div>
        <button
          @click="proxyEnabled = !proxyEnabled"
          :class="proxyEnabled ? 'bg-red-500' : 'bg-zinc-300 dark:bg-zinc-700'"
          class="relative w-11 h-6 rounded-full transition-colors shrink-0"
        >
          <span
            :class="proxyEnabled ? 'translate-x-5' : 'translate-x-0.5'"
            class="absolute top-0.5 left-0 w-5 h-5 bg-white rounded-full shadow-md transition-transform"
          />
        </button>
      </div>
      <p class="text-xs text-zinc-500 leading-relaxed mb-4">
        开启后，该账号的学校接口请求会从这里配置的代理出口发出；未开启时继续使用服务器默认出口。
      </p>

      <div
        class="mb-4 rounded-lg p-3 ring-1"
        :class="proxyEnabled
          ? 'bg-red-500/[0.07] ring-red-500/25'
          : 'bg-zinc-100/80 dark:bg-[#0d1117]/60 ring-black/[0.05] dark:ring-white/[0.04]'"
      >
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <div class="min-w-0">
            <p class="text-sm font-medium" :class="proxyEnabled ? 'text-red-700 dark:text-red-200' : 'text-zinc-600 dark:text-zinc-400'">
              {{ proxyEnabled ? '代理已启用' : '代理未启用' }}
            </p>
            <p class="text-[11px] text-zinc-500 mt-1 break-all">
              {{ proxyHost && proxyPort ? `${proxyScheme}://${proxyHost}:${proxyPort}` : '尚未配置出口地址' }}
            </p>
          </div>
          <button
            type="button"
            @click="testProxy"
            :disabled="testingProxy"
            class="inline-flex items-center justify-center gap-1.5 bg-sky-500/15 hover:bg-sky-500/25 disabled:opacity-50 disabled:cursor-not-allowed ring-1 ring-sky-500/30 text-blue-700 dark:text-blue-300 text-xs font-medium px-3 py-2 rounded-lg transition-colors"
          >
            <Activity class="w-3.5 h-3.5" :class="testingProxy ? 'wangui-spin' : ''" />
            {{ testingProxy ? '测试中…' : '一键测试' }}
          </button>
        </div>

        <div
          v-if="proxyTestResult"
          class="mt-3 rounded-lg bg-white/70 dark:bg-[#0d1117]/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04] p-3"
        >
          <p
            class="text-sm font-medium mb-2"
            :class="proxyTestResult.ok ? 'text-red-700 dark:text-red-200' : 'text-amber-700 dark:text-amber-300'"
          >
            {{ proxyTestResult.ok ? '代理可用，学校接口已成功响应' : '代理测试未通过' }}
          </p>
          <dl class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1.5 text-xs">
            <div class="flex justify-between gap-3">
              <dt class="text-zinc-500">出口</dt>
              <dd class="text-zinc-700 dark:text-zinc-300 font-mono-token break-all text-right">{{ proxyTestResult.outbound }}</dd>
            </div>
            <div class="flex justify-between gap-3">
              <dt class="text-zinc-500">耗时</dt>
              <dd class="text-zinc-700 dark:text-zinc-300 font-mono-token">{{ proxyTestResult.elapsedMs }} ms</dd>
            </div>
            <div class="flex justify-between gap-3">
              <dt class="text-zinc-500">探测端点</dt>
              <dd class="text-zinc-700 dark:text-zinc-300 font-mono-token">{{ proxyTestResult.endpoint }}</dd>
            </div>
            <div class="flex justify-between gap-3">
              <dt class="text-zinc-500">学校状态</dt>
              <dd class="text-zinc-700 dark:text-zinc-300 font-mono-token">{{ proxyTestResult.schoolStatus }}</dd>
            </div>
            <div class="sm:col-span-2 flex justify-between gap-3">
              <dt class="text-zinc-500 shrink-0">学校消息</dt>
              <dd class="text-zinc-700 dark:text-zinc-300 text-right">{{ proxyTestResult.schoolMessage }}</dd>
            </div>
          </dl>
        </div>
      </div>

      <div
        v-if="proxyHost.trim() === 'mihomo'"
        class="mb-4 rounded-lg bg-white/70 dark:bg-[#0d1117]/60 ring-1 ring-black/[0.05] dark:ring-white/[0.04] p-3"
      >
        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-3">
          <div class="min-w-0">
            <p class="text-sm font-medium text-[#161b22] dark:text-zinc-200">Mihomo 节点</p>
            <p class="text-[11px] text-zinc-500 mt-1 break-all">
              当前：{{ proxyNodes?.current || (loadingProxyNodes ? '读取中…' : '未知') }}
              <span v-if="proxyNodes?.shared" class="ml-1 text-zinc-400">· 全站共享出口</span>
            </p>
          </div>
          <div class="flex gap-2">
            <button
              type="button"
              @click="loadProxyNodes"
              :disabled="loadingProxyNodes || switchingProxyNode"
              class="inline-flex items-center gap-1.5 bg-zinc-100 hover:bg-zinc-200 dark:bg-zinc-800 dark:hover:bg-zinc-700 disabled:opacity-50 ring-1 ring-black/[0.06] dark:ring-white/[0.06] text-zinc-700 dark:text-zinc-300 text-xs px-3 py-2 rounded-lg transition-colors"
            >
              <RotateCcw class="w-3.5 h-3.5" :class="loadingProxyNodes ? 'wangui-spin' : ''" />
              刷新
            </button>
            <button
              type="button"
              @click="autoSelectProxyNode"
              :disabled="switchingProxyNode || !proxyNodes?.available"
              class="inline-flex items-center gap-1.5 bg-red-500/15 hover:bg-red-500/25 disabled:opacity-50 ring-1 ring-red-500/25 text-red-700 dark:text-red-300 text-xs px-3 py-2 rounded-lg transition-colors"
            >
              <Shuffle class="w-3.5 h-3.5" :class="switchingProxyNode ? 'wangui-spin' : ''" />
              自动选最快
            </button>
          </div>
        </div>

        <div v-if="proxyNodes?.available" class="grid grid-cols-1 sm:grid-cols-[1fr_auto] gap-2">
          <select
            v-model="selectedProxyNode"
            :disabled="switchingProxyNode"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-[#161b22] dark:text-zinc-200 disabled:opacity-50"
          >
            <option v-for="n in proxyNodes.nodes" :key="n.name" :value="n.name">
              {{ n.current ? '✓ ' : '' }}{{ n.name }}{{ n.delayMs ? ` · ${n.delayMs}ms` : '' }}
            </option>
          </select>
          <button
            type="button"
            @click="selectProxyNode"
            :disabled="switchingProxyNode || !selectedProxyNode || selectedProxyNode === proxyNodes.current"
            class="inline-flex items-center justify-center gap-1.5 bg-sky-500/15 hover:bg-sky-500/25 disabled:opacity-40 disabled:cursor-not-allowed ring-1 ring-sky-500/30 text-blue-700 dark:text-blue-300 text-xs font-medium px-3 py-2 rounded-lg transition-colors"
          >
            <Network class="w-3.5 h-3.5" :class="switchingProxyNode ? 'wangui-spin' : ''" />
            切换
          </button>
        </div>
        <p v-else class="text-[11px] text-amber-700 dark:text-amber-300">
          {{ proxyNodes?.message || '未检测到 mihomo 控制接口' }}
        </p>
        <div v-if="proxyNodes?.tested?.length" class="mt-3 flex flex-wrap gap-1.5">
          <span
            v-for="n in proxyNodes.tested"
            :key="n.name"
            class="px-2 py-1 rounded-md bg-zinc-100 dark:bg-zinc-800 text-[11px] text-zinc-600 dark:text-zinc-300 ring-1 ring-black/[0.04] dark:ring-white/[0.04]"
          >
            {{ n.name }} · {{ n.delayMs }}ms
          </span>
        </div>
        <p class="text-[11px] text-zinc-500 mt-3">
          切换的是 mihomo 的共享策略组；所有使用 <span class="font-mono-token">mihomo:7893</span> 的账号都会走当前节点。
        </p>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">协议</label>
          <select
            v-model="proxyScheme"
            :disabled="!proxyEnabled"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-[#161b22] dark:text-zinc-200 disabled:opacity-50"
          >
            <option value="socks5">socks5</option>
            <option value="http">http</option>
            <option value="https">https</option>
          </select>
        </div>
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">端口</label>
          <input
            v-model.number="proxyPort"
            type="number"
            min="1"
            max="65535"
            placeholder="19037"
            :disabled="!proxyEnabled"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-[#161b22] dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
          />
        </div>
        <div class="sm:col-span-2">
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">主机地址</label>
          <input
            v-model="proxyHost"
            type="text"
            placeholder="127.0.0.1 或 proxy.example.com"
            :disabled="!proxyEnabled"
            autocomplete="off"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-[#161b22] dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
          />
        </div>
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">用户名（可选）</label>
          <input
            v-model="proxyUsername"
            type="text"
            :disabled="!proxyEnabled"
            autocomplete="off"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-[#161b22] dark:text-zinc-200 disabled:opacity-50"
          />
        </div>
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1 flex items-center justify-between">
            <span>密码（可选）</span>
            <span
              v-if="proxyPasswordSet && !proxyPassword"
              class="text-red-600 dark:text-red-400 normal-case tracking-normal"
            >
              已设置
            </span>
          </label>
          <div class="relative">
            <input
              v-model="proxyPassword"
              :type="showProxyPassword ? 'text' : 'password'"
              :placeholder="proxyPasswordSet ? '留空保持不变' : ''"
              :disabled="!proxyEnabled"
              autocomplete="off"
              class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 pr-10 text-sm font-mono-token focus-ring text-[#161b22] dark:text-zinc-200 disabled:opacity-50"
            />
            <button
              @click="showProxyPassword = !showProxyPassword"
              type="button"
              :disabled="!proxyEnabled"
              class="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 text-zinc-500 hover:text-[#161b22] dark:hover:text-zinc-100 disabled:opacity-40"
            >
              <component :is="showProxyPassword ? EyeOff : Eye" class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>

      <p class="text-[11px] text-zinc-500 mt-3">
        测试只读取学校 available-rules 接口，不会写入签到数据。配置保存后，自动签到和立即签到都会走同一个出口。
      </p>
    </section>

    <!-- Section 4: 设备信息 -->
    <section class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center gap-2 mb-4">
        <Smartphone class="w-4 h-4 text-zinc-500" />
        <h2 class="text-base font-semibold text-[#161b22] dark:text-zinc-200">设备信息</h2>
      </div>
      <p class="text-xs text-zinc-500 leading-relaxed mb-4">
        会随签到请求一起发送，让后端审计看起来像真实手机签到。
      </p>
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">deviceModel</label>
          <select v-model="form.deviceModel"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-[#161b22] dark:text-zinc-200">
            <option value="iPhone">iPhone</option>
            <option value="Android">Android</option>
          </select>
        </div>
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">deviceSystem</label>
          <select v-model="form.deviceSystem"
            class="w-full bg-white dark:bg-[#0d1117] ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-[#161b22] dark:text-zinc-200">
            <option value="iOS">iOS</option>
            <option value="Android">Android</option>
          </select>
        </div>
      </div>
    </section>

    <!-- Sticky save bar -->
    <div class="sticky bottom-24 md:bottom-4 flex justify-end gap-2 z-10">
      <button
        @click="auth.refresh().then(() => auth.state.me && hydrate(auth.state.me.settings))"
        class="bg-zinc-200/70 dark:bg-zinc-800/80 hover:bg-zinc-200 dark:hover:bg-zinc-800 backdrop-blur-md ring-1 ring-black/10 dark:ring-white/10 text-zinc-700 dark:text-zinc-300 text-sm px-4 py-2 rounded-xl transition-colors inline-flex items-center gap-1.5"
      >
        <RotateCcw class="w-3.5 h-3.5" />
        重置
      </button>
      <button
        @click="saveAll"
        :disabled="saving"
        class="bg-red-500 hover:bg-red-400 disabled:opacity-50 text-[#0d1117] text-sm font-semibold px-5 py-2 rounded-xl transition-colors inline-flex items-center gap-1.5 shadow-[0_8px_20px_-8px_rgba(16,185,129,0.5)]"
      >
        <Save class="w-3.5 h-3.5" />
        {{ saving ? '保存中…' : '保存配置' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.expand-enter-active, .expand-leave-active { transition: all 0.25s ease; max-height: 600px; }
.expand-enter-from, .expand-leave-to { max-height: 0; opacity: 0; overflow: hidden; }
</style>
