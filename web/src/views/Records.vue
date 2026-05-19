<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  Check,
  X,
  CircleDot,
  Clock,
  RefreshCw,
  AlertCircle,
  ListFilter,
} from 'lucide-vue-next'
import type { SignRecord, SignStatus } from '../types'
import { api } from '../api'
import { formatDateTime } from '../lib/format'

const records = ref<SignRecord[]>([])
const loading = ref(false)
const filter = ref<SignStatus | 'all'>('all')

async function load() {
  loading.value = true
  try {
    records.value = await api.records()
  } catch {
    records.value = []
  } finally {
    loading.value = false
  }
}
onMounted(load)

const filtered = computed(() => {
  if (filter.value === 'all') return records.value
  return records.value.filter(r => r.status === filter.value)
})

const stats = computed(() => {
  const out: Record<string, number> = {
    success: 0, already: 0, exempt: 0, failed: 0, skipped: 0,
  }
  for (const r of records.value) out[r.status] = (out[r.status] || 0) + 1
  return out
})

const successRate = computed(() => {
  const n = records.value.length
  if (n === 0) return 0
  const ok = (stats.value.success || 0) + (stats.value.already || 0) + (stats.value.exempt || 0)
  return Math.round((ok / n) * 100)
})

const meta: Record<
  SignStatus,
  { label: string; icon: any; color: string; dotBg: string; bg: string }
> = {
  success: { label: '签到成功', icon: Check, color: 'text-red-400', dotBg: 'bg-red-500', bg: 'bg-red-500/10 ring-red-500/25' },
  already: { label: '今日已签', icon: CircleDot, color: 'text-blue-400', dotBg: 'bg-sky-500', bg: 'bg-sky-500/10 ring-sky-500/25' },
  exempt: { label: '免签', icon: CircleDot, color: 'text-zinc-500 dark:text-zinc-400', dotBg: 'bg-zinc-500', bg: 'bg-zinc-500/10 ring-zinc-500/25' },
  failed: { label: '签到失败', icon: X, color: 'text-red-400', dotBg: 'bg-red-500', bg: 'bg-red-500/10 ring-red-500/25' },
  skipped: { label: '跳过', icon: AlertCircle, color: 'text-amber-400', dotBg: 'bg-amber-500', bg: 'bg-amber-500/10 ring-amber-500/25' },
}

function info(s: string) { return meta[s as SignStatus] || meta.failed }

const filterOptions: Array<{ key: SignStatus | 'all'; label: string }> = [
  { key: 'all', label: '全部' },
  { key: 'success', label: '成功' },
  { key: 'already', label: '已签' },
  { key: 'exempt', label: '免签' },
  { key: 'failed', label: '失败' },
  { key: 'skipped', label: '跳过' },
]

// Calendar grid for the last 30 days.
const calendarCells = computed(() => {
  const days: Array<{ date: Date; status: SignStatus | null }> = []
  const today = new Date()
  for (let i = 29; i >= 0; i--) {
    const d = new Date(today)
    d.setDate(today.getDate() - i)
    d.setHours(0, 0, 0, 0)
    const next = new Date(d)
    next.setDate(d.getDate() + 1)

    // find first success/already record in that day
    let status: SignStatus | null = null
    for (const r of records.value) {
      const t = new Date(r.occurredAt * 1000)
      if (t >= d && t < next) {
        if (r.status === 'success' || r.status === 'already' || r.status === 'exempt') {
          status = r.status as SignStatus
          break
        }
        if (status === null) status = r.status as SignStatus
      }
    }
    days.push({ date: d, status })
  }
  return days
})

function cellColor(s: SignStatus | null): string {
  if (!s) return 'bg-zinc-200 dark:bg-zinc-800/50'
  return info(s).dotBg
}
</script>

<template>
  <div class="space-y-3">
    <header class="flex items-end justify-between gap-3 mb-1">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">签到记录</h1>
        <p class="text-sm text-zinc-500 mt-1">所有签到尝试的历史流水。</p>
      </div>
      <button
        @click="load"
        :disabled="loading"
        class="shrink-0 text-xs text-zinc-500 dark:text-zinc-400 hover:text-[#161b22] dark:hover:text-zinc-200 px-3 py-1.5 rounded-md hover:bg-black/5 dark:hover:bg-white/5 transition-colors inline-flex items-center gap-1.5"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="loading ? 'wangui-spin' : ''" />
        刷新
      </button>
    </header>

    <!-- Stats row -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-3.5">
        <p class="text-[10px] text-zinc-500 tracking-wide uppercase">总计</p>
        <p class="text-2xl font-bold tabular-nums mt-1">{{ records.length }}</p>
      </div>
      <div class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-3.5">
        <p class="text-[10px] text-zinc-500 tracking-wide uppercase">成功率</p>
        <p class="text-2xl font-bold tabular-nums mt-1 text-red-400">{{ successRate }}<span class="text-sm text-zinc-500">%</span></p>
      </div>
      <div class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-3.5">
        <p class="text-[10px] text-zinc-500 tracking-wide uppercase">成功 / 已签</p>
        <p class="text-2xl font-bold tabular-nums mt-1 text-blue-300">{{ (stats.success || 0) + (stats.already || 0) }}</p>
      </div>
      <div class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-3.5">
        <p class="text-[10px] text-zinc-500 tracking-wide uppercase">失败</p>
        <p class="text-2xl font-bold tabular-nums mt-1 text-red-300">{{ stats.failed || 0 }}</p>
      </div>
    </div>

    <!-- Last 30 days calendar -->
    <section class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-3">
        <h2 class="text-base font-semibold text-[#161b22] dark:text-zinc-200">最近 30 天</h2>
        <div class="flex items-center gap-2 text-[10px] text-zinc-500">
          <span class="inline-flex items-center gap-1"><span class="w-2 h-2 rounded-sm bg-red-500"></span>成功</span>
          <span class="inline-flex items-center gap-1"><span class="w-2 h-2 rounded-sm bg-red-500"></span>失败</span>
          <span class="inline-flex items-center gap-1"><span class="w-2 h-2 rounded-sm bg-zinc-200 dark:bg-zinc-800/50 ring-1 ring-zinc-300 dark:ring-zinc-700/50"></span>无</span>
        </div>
      </div>
      <div class="grid grid-cols-10 sm:grid-cols-15 gap-1.5">
        <div
          v-for="(cell, i) in calendarCells"
          :key="i"
          class="aspect-square rounded-sm group relative"
          :class="cellColor(cell.status)"
          :title="`${cell.date.toLocaleDateString('zh-CN')} ${cell.status || '无记录'}`"
        >
          <span class="absolute -top-1 -translate-y-full left-1/2 -translate-x-1/2 hidden group-hover:block z-10 px-2 py-1 rounded bg-zinc-200 dark:bg-zinc-800 text-[#161b22] dark:text-zinc-200 text-[10px] whitespace-nowrap pointer-events-none">
            {{ `${cell.date.getMonth() + 1}/${cell.date.getDate()}` }} · {{ cell.status || '无' }}
          </span>
        </div>
      </div>
    </section>

    <!-- Filter chips -->
    <div class="flex items-center gap-2 flex-wrap">
      <ListFilter class="w-3.5 h-3.5 text-zinc-500" />
      <button
        v-for="opt in filterOptions"
        :key="opt.key"
        @click="filter = opt.key"
        :class="filter === opt.key
          ? 'bg-red-500/20 text-red-300 ring-1 ring-red-500/30'
          : 'bg-white/85 dark:bg-[#161b22]/60 text-zinc-500 dark:text-zinc-400 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-[#161b22] dark:hover:text-zinc-200'"
        class="text-xs px-3 py-1 rounded-full transition-colors"
      >
        {{ opt.label }}
        <span v-if="opt.key !== 'all'" class="ml-1 opacity-70 tabular-nums">
          ({{ stats[opt.key] || 0 }})
        </span>
      </button>
    </div>

    <!-- Full list -->
    <section class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div v-if="loading" class="flex items-center justify-center py-10">
        <div class="h-5 w-5 rounded-full border-2 border-zinc-800 border-t-red-400 wangui-spin" />
      </div>
      <div v-else-if="filtered.length === 0" class="flex flex-col items-center py-12 text-center">
        <div class="w-12 h-12 rounded-2xl bg-zinc-200/60 dark:bg-zinc-800/60 ring-1 ring-black/[0.06] dark:ring-white/[0.05] flex items-center justify-center mb-3">
          <Clock class="w-5 h-5 text-zinc-500" />
        </div>
        <p class="text-sm text-zinc-500 dark:text-zinc-400">没有符合条件的记录</p>
      </div>

      <ol v-else class="relative">
        <div class="absolute left-[7px] top-1.5 bottom-1.5 w-px bg-gradient-to-b from-transparent via-black/[0.06] dark:via-white/[0.06] to-transparent" />
        <li v-for="r in filtered" :key="r.id" class="relative pl-7 pb-4 last:pb-0">
          <span
            class="absolute left-0 top-1 w-3.5 h-3.5 rounded-full ring-4 ring-white dark:ring-[#161b22]"
            :class="info(r.status).dotBg"
          />
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-1.5">
                <component :is="info(r.status).icon" class="w-3.5 h-3.5" :class="info(r.status).color" />
                <span class="text-sm font-medium" :class="info(r.status).color">
                  {{ info(r.status).label }}
                </span>
              </div>
              <p class="text-xs text-zinc-500 dark:text-zinc-400 mt-1 leading-relaxed break-all">{{ r.message || '—' }}</p>
            </div>
            <span class="shrink-0 text-[10px] text-zinc-500 tabular-nums whitespace-nowrap">
              {{ formatDateTime(r.occurredAt) }}
            </span>
          </div>
        </li>
      </ol>
    </section>
  </div>
</template>

<style scoped>
.grid-cols-15 {
  grid-template-columns: repeat(15, minmax(0, 1fr));
}
</style>
