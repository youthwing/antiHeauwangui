<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import {
  Users,
  Ticket,
  Activity,
  AlertTriangle,
  TrendingUp,
  ArrowRight,
  ScrollText,
} from 'lucide-vue-next'
import type { AdminStats, AdminLog } from '../../types'
import { adminApi } from '../../api'
import { formatDateTime } from '../../lib/format'

const stats = ref<AdminStats | null>(null)
const recentLogs = ref<AdminLog[]>([])
const loading = ref(true)

// School-rules snapshot: filled by daily 18:00 sweep.
interface SchoolRule {
  ruleId: number
  ruleName: string
  startTime: string
  endTime: string
  description: string
}
const schoolRules = ref<SchoolRule[]>([])
const schoolRulesUpdatedAt = ref(0)

async function loadAll() {
  loading.value = true
  try {
    const [s, l, sr] = await Promise.all([
      adminApi.stats(),
      adminApi.logs(10),
      adminApi.schoolRules(),
    ])
    stats.value = s
    recentLogs.value = l
    schoolRules.value = Array.isArray(sr.rules) ? (sr.rules as SchoolRule[]) : []
    schoolRulesUpdatedAt.value = sr.updatedAt
  } finally {
    loading.value = false
  }
}
onMounted(loadAll)

const todayBreakdown = computed(() => {
  const t = stats.value?.today || {}
  return [
    { key: 'success', label: '成功', value: t.success || 0, color: 'text-emerald-400', dot: 'bg-emerald-500' },
    { key: 'already', label: '已签', value: t.already || 0, color: 'text-blue-400', dot: 'bg-blue-500' },
    { key: 'exempt', label: '免签', value: t.exempt || 0, color: 'text-zinc-500 dark:text-zinc-400', dot: 'bg-zinc-500' },
    { key: 'failed', label: '失败', value: t.failed || 0, color: 'text-red-400', dot: 'bg-red-500' },
  ]
})

const logMeta: Record<string, { label: string; color: string; dotBg: string }> = {
  success: { label: '成功', color: 'text-emerald-400', dotBg: 'bg-emerald-500' },
  already: { label: '已签', color: 'text-blue-400', dotBg: 'bg-blue-500' },
  exempt: { label: '免签', color: 'text-zinc-500 dark:text-zinc-400', dotBg: 'bg-zinc-500' },
  failed: { label: '失败', color: 'text-red-400', dotBg: 'bg-red-500' },
  skipped: { label: '跳过', color: 'text-amber-400', dotBg: 'bg-amber-500' },
}
function info(s: string) { return logMeta[s] || logMeta.failed }
</script>

<template>
  <div class="space-y-3">
    <header class="mb-1">
      <h1 class="text-2xl font-bold tracking-tight">概览</h1>
      <p class="text-sm text-zinc-500 mt-1">系统当前状态。</p>
    </header>

    <!-- KPI cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <RouterLink to="/rosekhlifa/users" class="block">
        <div class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] hover:ring-black/[0.12] dark:hover:ring-white/[0.12] p-3.5 transition-all">
          <div class="flex items-center gap-2 mb-3">
            <Users class="w-4 h-4 text-zinc-500" />
            <span class="text-[10px] text-zinc-500 tracking-wide uppercase">用户</span>
          </div>
          <p class="text-3xl font-bold tabular-nums">
            {{ stats?.users.total ?? '—' }}
            <span v-if="stats?.users.guests" class="text-base font-medium text-zinc-500 dark:text-zinc-400 ml-1">
              + <span class="text-emerald-400">{{ stats.users.guests }}</span> 临时
            </span>
          </p>
          <p class="text-xs text-zinc-500 mt-2 tabular-nums">
            <span class="text-red-400">{{ stats?.users.disabled ?? 0 }}</span> 禁用 ·
            <span class="text-amber-400">{{ stats?.users.expiring ?? 0 }}</span> 24h 内过期
          </p>
        </div>
      </RouterLink>

      <RouterLink to="/rosekhlifa/codes" class="block">
        <div class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] hover:ring-black/[0.12] dark:hover:ring-white/[0.12] p-3.5 transition-all">
          <div class="flex items-center gap-2 mb-3">
            <Ticket class="w-4 h-4 text-zinc-500" />
            <span class="text-[10px] text-zinc-500 tracking-wide uppercase">邀请码</span>
          </div>
          <p class="text-3xl font-bold tabular-nums">{{ stats?.codes.total ?? '—' }}</p>
          <p class="text-xs text-zinc-500 mt-2 tabular-nums">
            <span class="text-emerald-400">{{ stats?.codes.used ?? 0 }}</span> 已用 ·
            <span class="text-zinc-700 dark:text-zinc-300">{{ stats?.codes.unused ?? 0 }}</span> 未用
          </p>
        </div>
      </RouterLink>

      <div class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-3.5">
        <div class="flex items-center gap-2 mb-3">
          <Activity class="w-4 h-4 text-zinc-500" />
          <span class="text-[10px] text-zinc-500 tracking-wide uppercase">今日签到</span>
        </div>
        <p class="text-3xl font-bold tabular-nums">
          {{ (stats?.today.success ?? 0) + (stats?.today.already ?? 0) }}
        </p>
        <p class="text-xs text-zinc-500 mt-2 tabular-nums">
          失败 <span class="text-red-400">{{ stats?.today.failed ?? 0 }}</span>
        </p>
      </div>

      <div class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-3.5">
        <div class="flex items-center gap-2 mb-3">
          <AlertTriangle class="w-4 h-4 text-zinc-500" />
          <span class="text-[10px] text-zinc-500 tracking-wide uppercase">告警</span>
        </div>
        <p class="text-3xl font-bold tabular-nums text-amber-400">
          {{ (stats?.users.expiring ?? 0) + (stats?.today.failed ?? 0) }}
        </p>
        <p class="text-xs text-zinc-500 mt-2">过期 + 今日失败</p>
      </div>
    </div>

    <!-- Today breakdown -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center gap-2 mb-4">
        <TrendingUp class="w-4 h-4 text-zinc-500" />
        <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">今日活动</h2>
      </div>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div v-for="b in todayBreakdown" :key="b.key">
          <div class="flex items-center gap-1.5 mb-1.5">
            <span class="w-2 h-2 rounded-full" :class="b.dot" />
            <span class="text-xs text-zinc-500">{{ b.label }}</span>
          </div>
          <p class="text-2xl font-bold tabular-nums" :class="b.color">{{ b.value }}</p>
        </div>
      </div>
    </section>

    <!-- School rules snapshot — refreshed daily at 18:00 by the scheduler;
         admin gets emailed if it changes between two probes. -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-3 gap-2">
        <div class="flex items-center gap-2 min-w-0">
          <ScrollText class="w-4 h-4 text-zinc-500" />
          <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">学校签到规则</h2>
        </div>
        <span v-if="schoolRulesUpdatedAt > 0" class="text-[11px] text-zinc-500 font-mono-token shrink-0">
          {{ formatDateTime(schoolRulesUpdatedAt) }}
        </span>
      </div>
      <div v-if="schoolRules.length === 0" class="text-xs text-zinc-500">
        尚未抓取规则快照（每天 18:00 自动抓一次）。规则变化会邮件 + Server酱 推送。
      </div>
      <ul v-else class="space-y-2">
        <li
          v-for="r in schoolRules"
          :key="r.ruleId"
          class="rounded-lg bg-white/70 dark:bg-zinc-950/40 ring-1 ring-black/[0.05] dark:ring-white/[0.04] p-3"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <p class="text-sm font-medium text-zinc-900 dark:text-zinc-200">
                <span class="text-[10px] font-mono-token text-zinc-500 mr-1.5">#{{ r.ruleId }}</span>
                {{ r.ruleName || '(无名)' }}
              </p>
              <p v-if="r.description" class="text-[11px] text-zinc-500 mt-1 leading-relaxed">
                {{ r.description }}
              </p>
            </div>
            <span class="shrink-0 text-xs text-emerald-400 font-mono-token tabular-nums whitespace-nowrap">
              {{ r.startTime }} – {{ r.endTime }}
            </span>
          </div>
        </li>
      </ul>
    </section>

    <!-- Recent activity -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">最近活动</h2>
        <RouterLink to="/rosekhlifa/logs" class="text-xs text-zinc-500 hover:text-emerald-400 inline-flex items-center gap-1">
          查看全部
          <ArrowRight class="w-3 h-3" />
        </RouterLink>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-8">
        <div class="h-5 w-5 rounded-full border-2 border-zinc-800 border-t-emerald-400 wangui-spin" />
      </div>
      <div v-else-if="recentLogs.length === 0" class="text-center py-8 text-sm text-zinc-500">
        无活动
      </div>
      <ol v-else class="relative">
        <div class="absolute left-[7px] top-1.5 bottom-1.5 w-px bg-gradient-to-b from-transparent via-black/[0.06] dark:via-white/[0.06] to-transparent" />
        <li v-for="r in recentLogs" :key="r.id" class="relative pl-7 pb-3 last:pb-0">
          <span class="absolute left-0 top-1 w-3.5 h-3.5 rounded-full ring-4 ring-white dark:ring-zinc-900" :class="info(r.status).dotBg" />
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-1.5 flex-wrap">
                <span class="text-sm font-medium" :class="info(r.status).color">{{ info(r.status).label }}</span>
                <span class="text-zinc-400 dark:text-zinc-700">·</span>
                <span class="text-sm text-zinc-700 dark:text-zinc-300">{{ r.userName || r.userId }}</span>
              </div>
              <p class="text-xs text-zinc-500 mt-0.5 leading-relaxed break-all">{{ r.message || '—' }}</p>
            </div>
            <span class="shrink-0 text-[10px] text-zinc-500 dark:text-zinc-600 tabular-nums">{{ formatDateTime(r.occurredAt) }}</span>
          </div>
        </li>
      </ol>
    </section>
  </div>
</template>
