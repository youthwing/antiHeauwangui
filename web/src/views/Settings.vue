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
} from 'lucide-vue-next'
import type { Settings, Dorm } from '../types'
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
      signDays: form.signDays & 0x7f,
    }
    await api.updateSettings(payload)
    showToast('ok', '配置已保存')
    await auth.refresh()
  } catch (e: any) {
    showToast('err', e.message || '保存失败')
  } finally {
    saving.value = false
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
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-4 gap-3">
        <div class="flex items-center gap-2">
          <Power class="w-4 h-4 text-zinc-500" />
          <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">自动签到</h2>
        </div>
        <button
          @click="toggleAutoSign"
          :class="form.autoSign ? 'bg-emerald-500' : 'bg-zinc-300 dark:bg-zinc-700'"
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
      <div class="mb-5 p-3 rounded-lg bg-white/70 dark:bg-zinc-950/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04]">
        <div class="flex items-center justify-between mb-3 gap-2">
          <span class="text-[10px] text-zinc-500 tracking-wide uppercase">签到日期</span>
          <div class="flex gap-1">
            <button
              type="button"
              @click="applyPreset(PRESETS.every)"
              :class="activePreset === 'every'
                ? 'bg-emerald-500/15 text-emerald-300 ring-1 ring-emerald-500/30'
                : 'bg-zinc-200/60 dark:bg-zinc-800/60 text-zinc-500 dark:text-zinc-400 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-zinc-900 dark:hover:text-zinc-200'"
              class="text-[11px] px-2.5 py-1 rounded-md transition-colors"
            >每天</button>
            <button
              type="button"
              @click="applyPreset(PRESETS.weekday)"
              :class="activePreset === 'weekday'
                ? 'bg-emerald-500/15 text-emerald-300 ring-1 ring-emerald-500/30'
                : 'bg-zinc-200/60 dark:bg-zinc-800/60 text-zinc-500 dark:text-zinc-400 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-zinc-900 dark:hover:text-zinc-200'"
              class="text-[11px] px-2.5 py-1 rounded-md transition-colors"
            >工作日</button>
            <button
              type="button"
              @click="applyPreset(PRESETS.weekend)"
              :class="activePreset === 'weekend'
                ? 'bg-emerald-500/15 text-emerald-300 ring-1 ring-emerald-500/30'
                : 'bg-zinc-200/60 dark:bg-zinc-800/60 text-zinc-500 dark:text-zinc-400 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-zinc-900 dark:hover:text-zinc-200'"
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
              ? 'bg-emerald-500 text-zinc-950 ring-1 ring-emerald-400 shadow-[0_0_0_2px_rgba(16,185,129,0.15)]'
              : 'bg-zinc-200/60 dark:bg-zinc-800/60 text-zinc-500 dark:text-zinc-500 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-zinc-900 dark:hover:text-zinc-200'"
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
          class="text-[11px] text-zinc-500 dark:text-zinc-400 hover:text-emerald-400 transition-colors inline-flex items-center gap-1"
        >
          <span>ⓘ 这 4 个数字啥意思？</span>
          <span class="text-zinc-600">{{ showScheduleFaq ? '收起' : '展开看说明' }}</span>
        </button>
        <Transition name="expand">
          <div v-if="showScheduleFaq" class="mt-2 rounded-lg bg-blue-500/[0.05] ring-1 ring-blue-500/20 p-3 text-[11px] text-zinc-700 dark:text-zinc-300 leading-relaxed space-y-2.5 overflow-hidden">
            <p>
              系统每天 22:00 整点醒来，但不会让所有用户都在 22:00:00 这一秒同时签到 —— 那样 5 个学号同 IP 集中发请求会很显眼，且每天看你都"卡在 22:00 没结果"也会焦虑。下面 4 个参数控制具体的延迟。
            </p>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-2">
              <div>
                <p class="text-emerald-300 font-medium">首次触发分钟</p>
                <p class="text-zinc-500 mt-0.5">
                  从 22:00 起再等几分钟。<strong>激活账号时系统已为你随机分配过一次（0–27 分钟）</strong>，每个用户不同。你的当前值意味着每天大约 22:{{ String(form.triggerMinute).padStart(2, '0') }} 左右签。
                </p>
              </div>
              <div>
                <p class="text-emerald-300 font-medium">抖动秒数</p>
                <p class="text-zinc-500 mt-0.5">
                  在上面那个分钟基础上，再随机往后推 0–{{ form.jitterSec }} 秒。每天具体的"秒数"都不同，避免每天精确到秒的规律。
                </p>
              </div>
              <div>
                <p class="text-emerald-300 font-medium">重试次数</p>
                <p class="text-zinc-500 mt-0.5">
                  第一次签失败（网络问题 / 学校 API 抽风）后，再试几次。默认 3 次，4 次机会总共。
                </p>
              </div>
              <div>
                <p class="text-emerald-300 font-medium">重试间隔</p>
                <p class="text-zinc-500 mt-0.5">
                  两次重试之间等几分钟。默认 5 分钟，配合"重试 3 次" = 最多覆盖 20 分钟（接近 22:30 截止）。
                </p>
              </div>
            </div>
            <p class="text-zinc-500 mt-2">
              <strong class="text-zinc-400">大白话总结</strong>：你的预定签到时刻 ≈ <span class="font-mono-token text-emerald-300">22:{{ String(form.triggerMinute).padStart(2, '0') }}</span>，实际可能再往后 0–{{ form.jitterSec }} 秒。如果你没动过这些参数，<strong>什么都不用改，默认很合理</strong>。
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
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-zinc-900 dark:text-zinc-200"
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
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-zinc-900 dark:text-zinc-200"
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
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-zinc-900 dark:text-zinc-200"
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
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm font-mono-token focus-ring text-zinc-900 dark:text-zinc-200"
          />
          <p class="text-[10px] text-zinc-500 dark:text-zinc-600 mt-1">两次重试之间等几分钟</p>
        </div>
      </div>

      <div class="mt-5 p-3 rounded-lg bg-white/70 dark:bg-zinc-950/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04]">
        <div class="flex items-center gap-2 mb-2">
          <Clock class="w-3.5 h-3.5 text-zinc-500" />
          <span class="text-[10px] text-zinc-500 tracking-wide uppercase">本配置下的尝试时刻</span>
        </div>
        <div class="flex flex-wrap gap-1.5">
          <span
            v-for="(t, i) in previewSchedule"
            :key="t"
            class="px-2 py-0.5 rounded text-xs font-mono-token tabular-nums"
            :class="i === 0 ? 'bg-emerald-500/15 text-emerald-300 ring-1 ring-emerald-500/30' : 'bg-zinc-200 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400'"
          >
            {{ t }}<span v-if="i === 0" class="ml-1 text-[9px] opacity-70">主</span>
          </span>
        </div>
        <p class="text-[10px] text-zinc-500 dark:text-zinc-600 mt-2">签到窗口 22:00–22:30，超出窗口的重试会被跳过</p>
      </div>
    </section>

    <!-- Section 2: 我的宿舍楼 -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
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
              class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2.5 text-sm focus-ring text-zinc-900 dark:text-zinc-200 appearance-none cursor-pointer disabled:opacity-50"
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
          class="rounded-lg bg-emerald-500/[0.07] ring-1 ring-emerald-500/25 p-4"
        >
          <div class="flex items-start gap-3">
            <MapPin class="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
            <div class="min-w-0 flex-1">
              <p class="text-sm font-medium text-emerald-200">{{ currentDorm.name }}</p>
              <p class="text-xs text-emerald-400/70 mt-1 break-all">
                {{ currentDorm.address || '未配置地址' }}
              </p>
              <p class="text-[10px] text-emerald-400/50 mt-1.5 font-mono-token tabular-nums">
                {{ currentDorm.latitude.toFixed(6) }}, {{ currentDorm.longitude.toFixed(6) }}
                <span class="ml-1 text-emerald-400/30">(WGS84)</span>
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
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-4 gap-3">
        <div class="flex items-center gap-2">
          <Mail class="w-4 h-4 text-zinc-500" />
          <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">邮件通知</h2>
        </div>
        <button
          @click="form.notifyEnabled = !form.notifyEnabled"
          :class="form.notifyEnabled ? 'bg-emerald-500' : 'bg-zinc-300 dark:bg-zinc-700'"
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
        class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 disabled:opacity-50"
      />
      <p class="text-[11px] text-zinc-500 mt-2">
        只在自动签到的「最终结果」时发一封；手动「立即签到」不发邮件。
      </p>
    </section>

    <!-- Section 4: 设备信息 -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center gap-2 mb-4">
        <Smartphone class="w-4 h-4 text-zinc-500" />
        <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">设备信息</h2>
      </div>
      <p class="text-xs text-zinc-500 leading-relaxed mb-4">
        会随签到请求一起发送，让后端审计看起来像真实手机签到。
      </p>
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">deviceModel</label>
          <select v-model="form.deviceModel"
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200">
            <option value="iPhone">iPhone</option>
            <option value="Android">Android</option>
          </select>
        </div>
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">deviceSystem</label>
          <select v-model="form.deviceSystem"
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200">
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
        class="bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 text-zinc-950 text-sm font-semibold px-5 py-2 rounded-xl transition-colors inline-flex items-center gap-1.5 shadow-[0_8px_20px_-8px_rgba(16,185,129,0.5)]"
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
