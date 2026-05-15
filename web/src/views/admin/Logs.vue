<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RefreshCw, ListFilter, Clock } from 'lucide-vue-next'
import type { AdminLog, SignStatus } from '../../types'
import { adminApi } from '../../api'
import { formatDateTime } from '../../lib/format'

const logs = ref<AdminLog[]>([])
const loading = ref(false)
const filter = ref<SignStatus | 'all'>('all')

async function load() {
  loading.value = true
  try {
    logs.value = await adminApi.logs(200)
  } finally {
    loading.value = false
  }
}
onMounted(load)

const filtered = computed(() => {
  if (filter.value === 'all') return logs.value
  return logs.value.filter(l => l.status === filter.value)
})

const stats = computed(() => {
  const o: Record<string, number> = { success: 0, already: 0, exempt: 0, failed: 0, skipped: 0 }
  for (const l of logs.value) o[l.status] = (o[l.status] || 0) + 1
  return o
})

const filterOptions: Array<{ key: SignStatus | 'all'; label: string }> = [
  { key: 'all', label: '全部' },
  { key: 'success', label: '成功' },
  { key: 'already', label: '已签' },
  { key: 'exempt', label: '免签' },
  { key: 'failed', label: '失败' },
  { key: 'skipped', label: '跳过' },
]

const meta: Record<string, { color: string; dotBg: string; label: string }> = {
  success: { color: 'text-emerald-400', dotBg: 'bg-emerald-500', label: '成功' },
  already: { color: 'text-blue-400', dotBg: 'bg-blue-500', label: '已签' },
  exempt: { color: 'text-zinc-500 dark:text-zinc-400', dotBg: 'bg-zinc-500', label: '免签' },
  failed: { color: 'text-red-400', dotBg: 'bg-red-500', label: '失败' },
  skipped: { color: 'text-amber-400', dotBg: 'bg-amber-500', label: '跳过' },
}
function info(s: string) { return meta[s] || meta.failed }
</script>

<template>
  <div class="space-y-3">
    <header class="flex items-end justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">日志</h1>
        <p class="text-sm text-zinc-500 mt-1">所有用户的签到流水。</p>
      </div>
      <button @click="load" :disabled="loading"
        class="shrink-0 text-xs text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 px-3 py-1.5 rounded-md hover:bg-black/5 dark:hover:bg-white/5 transition-colors inline-flex items-center gap-1.5">
        <RefreshCw class="w-3.5 h-3.5" :class="loading ? 'wangui-spin' : ''" />
        刷新
      </button>
    </header>

    <div class="flex items-center gap-2 flex-wrap">
      <ListFilter class="w-3.5 h-3.5 text-zinc-500" />
      <button v-for="opt in filterOptions" :key="opt.key" @click="filter = opt.key"
        :class="filter === opt.key
          ? 'bg-emerald-500/20 text-emerald-300 ring-1 ring-emerald-500/30'
          : 'bg-white/85 dark:bg-zinc-900/60 text-zinc-500 dark:text-zinc-400 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-zinc-900 dark:hover:text-zinc-200'"
        class="text-xs px-3 py-1 rounded-full transition-colors">
        {{ opt.label }}
        <span v-if="opt.key !== 'all'" class="ml-1 opacity-70 tabular-nums">
          ({{ stats[opt.key] || 0 }})
        </span>
      </button>
    </div>

    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div v-if="loading && logs.length === 0" class="flex items-center justify-center py-12">
        <div class="h-6 w-6 rounded-full border-2 border-zinc-800 border-t-emerald-400 wangui-spin" />
      </div>
      <div v-else-if="filtered.length === 0" class="flex flex-col items-center py-16 text-center">
        <Clock class="w-10 h-10 text-zinc-400 dark:text-zinc-700 mb-2" />
        <p class="text-sm text-zinc-500">没有符合条件的日志</p>
      </div>
      <ol v-else class="relative">
        <div class="absolute left-[7px] top-1.5 bottom-1.5 w-px bg-gradient-to-b from-transparent via-black/[0.06] dark:via-white/[0.06] to-transparent" />
        <li v-for="r in filtered" :key="r.id" class="relative pl-7 pb-4 last:pb-0">
          <span class="absolute left-0 top-1 w-3.5 h-3.5 rounded-full ring-4 ring-white dark:ring-zinc-900" :class="info(r.status).dotBg" />
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-1.5 flex-wrap">
                <span class="text-sm font-medium" :class="info(r.status).color">{{ info(r.status).label }}</span>
                <span class="text-zinc-400 dark:text-zinc-700">·</span>
                <RouterLink :to="`/rosekhlifa/users`" class="text-sm text-zinc-700 dark:text-zinc-300 hover:text-emerald-400 transition-colors">
                  {{ r.userName || r.userId }}
                </RouterLink>
                <span class="text-[10px] font-mono-token text-zinc-500 dark:text-zinc-600">({{ r.userId }})</span>
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
