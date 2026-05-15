<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  Search,
  RefreshCw,
  X,
  Power,
  Trash2,
  CheckCircle2,
  XCircle,
  KeyRound,
  Copy,
} from 'lucide-vue-next'
import type { AdminUser } from '../../types'
import { adminApi } from '../../api'
import { formatDateTime, formatRemaining } from '../../lib/format'
import { showToast } from '../../lib/toast'
import { copyText } from '../../lib/clipboard'

const users = ref<AdminUser[]>([])
const loading = ref(false)
const search = ref('')

const detail = ref<AdminUser | null>(null)
const loadingDetail = ref(false)

// PIN reset state
const resetting = ref(false)
const newPinResult = ref<{ user: string; pin: string } | null>(null)

async function load() {
  loading.value = true
  try {
    users.value = await adminApi.listUsers(search.value.trim())
  } catch (e: any) {
    showToast('err', e.message || '加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function open(u: AdminUser) {
  detail.value = u
  loadingDetail.value = true
  try {
    detail.value = await adminApi.getUser(u.userId)
  } finally {
    loadingDetail.value = false
  }
}

async function toggleDisabled(u: AdminUser) {
  try {
    const updated = await adminApi.updateUser(u.userId, { isDisabled: !u.isDisabled })
    Object.assign(u, updated)
    if (detail.value?.userId === u.userId) Object.assign(detail.value, updated)
    showToast('ok', u.isDisabled ? '已禁用' : '已启用')
  } catch (e: any) {
    showToast('err', e.message || '操作失败')
  }
}

async function resetPin(u: AdminUser) {
  if (!confirm(`重置 ${u.userName} (${u.userNumber}) 的登录 PIN？\n这会强制他的所有会话登出。`)) return
  resetting.value = true
  try {
    const r = await adminApi.resetUserPin(u.userId)
    newPinResult.value = { user: `${u.userName} / ${u.userNumber}`, pin: r.newPin }
  } catch (e: any) {
    showToast('err', e.message || '重置失败')
  } finally {
    resetting.value = false
  }
}

async function copyPin() {
  if (!newPinResult.value) return
  const ok = await copyText(newPinResult.value.pin)
  showToast(ok ? 'ok' : 'err', ok ? 'PIN 已复制' : '复制失败，请手动选取')
}

async function remove(u: AdminUser) {
  if (!confirm(`删除用户 ${u.userName} (${u.userNumber})？\n会同时释放他的邀请码。`)) return
  try {
    await adminApi.deleteUser(u.userId)
    showToast('ok', '已删除')
    if (detail.value?.userId === u.userId) detail.value = null
    await load()
  } catch (e: any) {
    showToast('err', e.message || '删除失败')
  }
}

const recordMeta: Record<string, { color: string; dotBg: string; label: string }> = {
  success: { color: 'text-emerald-400', dotBg: 'bg-emerald-500', label: '成功' },
  already: { color: 'text-blue-400', dotBg: 'bg-blue-500', label: '已签' },
  exempt: { color: 'text-zinc-500 dark:text-zinc-400', dotBg: 'bg-zinc-500', label: '免签' },
  failed: { color: 'text-red-400', dotBg: 'bg-red-500', label: '失败' },
  skipped: { color: 'text-amber-400', dotBg: 'bg-amber-500', label: '跳过' },
}
</script>

<template>
  <div class="space-y-3">
    <header>
      <h1 class="text-2xl font-bold tracking-tight">用户管理</h1>
      <p class="text-sm text-zinc-500 mt-1">查看用户、禁用账号、查看其签到记录。</p>
    </header>

    <div class="flex items-center gap-3">
      <div class="relative flex-1 max-w-md">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-zinc-500" />
        <input
          v-model="search"
          @keyup.enter="load"
          placeholder="搜索姓名 / 学号 / 邀请码"
          class="w-full pl-9 pr-3 py-2 bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg text-sm focus-ring text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600"
        />
      </div>
      <button @click="load" :disabled="loading"
        class="text-xs text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 px-2 py-1.5 rounded-md hover:bg-black/5 dark:hover:bg-white/5 transition-colors inline-flex items-center gap-1">
        <RefreshCw class="w-3.5 h-3.5" :class="loading ? 'wangui-spin' : ''" />
      </button>
    </div>

    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="bg-white/50 dark:bg-zinc-950/50 border-b border-black/[0.08] dark:border-white/[0.06]">
            <tr class="text-left text-[10px] text-zinc-500 uppercase tracking-wide">
              <th class="px-4 py-3 font-medium">姓名</th>
              <th class="px-4 py-3 font-medium">学号</th>
              <th class="px-4 py-3 font-medium">学院 / 班级</th>
              <th class="px-4 py-3 font-medium">邀请码</th>
              <th class="px-4 py-3 font-medium">宿舍楼</th>
              <th class="px-4 py-3 font-medium">Token</th>
              <th class="px-4 py-3 font-medium">状态</th>
              <th class="px-4 py-3 font-medium text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-black/[0.05] dark:divide-white/[0.04]">
            <tr v-if="loading && users.length === 0">
              <td colspan="8" class="px-4 py-10 text-center">
                <div class="h-5 w-5 rounded-full border-2 border-zinc-800 border-t-emerald-400 wangui-spin mx-auto" />
              </td>
            </tr>
            <tr v-else-if="users.length === 0">
              <td colspan="8" class="px-4 py-12 text-center text-sm text-zinc-500">
                还没有用户
              </td>
            </tr>
            <tr v-for="u in users" :key="u.userId"
              @click="open(u)"
              class="hover:bg-black/[0.02] dark:hover:bg-white/[0.02] transition-colors cursor-pointer">
              <td class="px-4 py-3 font-medium">{{ u.userName }}</td>
              <td class="px-4 py-3 font-mono-token text-zinc-500 dark:text-zinc-400">{{ u.userNumber }}</td>
              <td class="px-4 py-3 text-zinc-500 dark:text-zinc-400 text-xs">
                {{ u.userSection }} · {{ u.userClass }}
              </td>
              <td class="px-4 py-3 font-mono-token text-zinc-500 dark:text-zinc-400 text-xs">{{ u.inviteCode || '—' }}</td>
              <td class="px-4 py-3 text-xs">
                <span v-if="u.dormName"
                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-1 ring-emerald-500/25">
                  {{ u.dormName }}
                </span>
                <span v-else class="text-zinc-500">未绑定</span>
              </td>
              <td class="px-4 py-3">
                <CheckCircle2 v-if="u.tokenValid" class="w-3.5 h-3.5 text-emerald-400 inline" />
                <XCircle v-else class="w-3.5 h-3.5 text-red-400 inline" />
                <span class="ml-1 text-xs text-zinc-500 tabular-nums">
                  {{ formatRemaining(Math.max(0, u.tokenExp - Math.floor(Date.now() / 1000))) }}
                </span>
              </td>
              <td class="px-4 py-3">
                <span v-if="u.isDisabled"
                  class="inline-flex px-2 py-0.5 rounded-md text-xs bg-red-500/15 text-red-400 ring-1 ring-red-500/30">
                  已禁用
                </span>
                <span v-else-if="u.autoSign"
                  class="inline-flex px-2 py-0.5 rounded-md text-xs bg-emerald-500/15 text-emerald-400 ring-1 ring-emerald-500/30">
                  自动开启
                </span>
                <span v-else
                  class="inline-flex px-2 py-0.5 rounded-md text-xs bg-zinc-300/50 dark:bg-zinc-700/50 text-zinc-500 dark:text-zinc-400">
                  自动关闭
                </span>
              </td>
              <td class="px-4 py-3 text-right">
                <div class="inline-flex gap-0.5" @click.stop>
                  <button @click="toggleDisabled(u)" :title="u.isDisabled ? '启用' : '禁用'"
                    class="p-1.5 rounded hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 dark:text-zinc-400 hover:text-amber-400 transition-colors">
                    <Power class="w-3.5 h-3.5" />
                  </button>
                  <button @click="remove(u)" title="删除"
                    class="p-1.5 rounded hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 dark:text-zinc-400 hover:text-red-400 transition-colors">
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- New PIN reveal modal (after admin reset) -->
    <Transition name="modal">
      <div v-if="newPinResult" class="fixed inset-0 z-[60] bg-white/85 dark:bg-zinc-950/85 backdrop-blur flex items-center justify-center p-4"
        @click.self="newPinResult = null">
        <div class="w-full max-w-sm bg-zinc-100 dark:bg-zinc-900 ring-1 ring-emerald-500/30 rounded-2xl shadow-2xl">
          <div class="p-5 border-b border-black/[0.08] dark:border-white/[0.06] flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-emerald-500/15 ring-1 ring-emerald-500/30 flex items-center justify-center shrink-0">
              <KeyRound class="w-5 h-5 text-emerald-400" />
            </div>
            <div>
              <h2 class="text-base font-bold">PIN 已重置</h2>
              <p class="text-xs text-zinc-500 mt-0.5 truncate">{{ newPinResult.user }}</p>
            </div>
          </div>

          <div class="p-5">
            <p class="text-xs text-zinc-500 dark:text-zinc-400 mb-3">把这串 PIN 告诉用户（旧 PIN 已作废，所有会话已登出）：</p>
            <div class="bg-white dark:bg-zinc-950 ring-1 ring-emerald-500/30 rounded-xl px-4 py-5 text-center">
              <p class="text-4xl font-bold font-mono-token tabular-nums tracking-[0.5em] text-emerald-300 pl-[0.5em]">
                {{ newPinResult.pin }}
              </p>
            </div>
            <p class="text-[10px] text-zinc-500 dark:text-zinc-600 text-center mt-2">关闭后此 PIN 不再显示</p>
          </div>

          <div class="px-5 pb-5 flex gap-2">
            <button @click="copyPin"
              class="flex-1 inline-flex items-center justify-center gap-1.5 bg-emerald-500 hover:bg-emerald-400 text-zinc-950 text-sm font-medium py-2 rounded-lg transition-colors">
              <Copy class="w-3.5 h-3.5" />
              复制 PIN
            </button>
            <button @click="newPinResult = null"
              class="px-4 py-2 text-sm text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              我已记下
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Detail slide-over -->
    <Transition name="drawer">
      <div v-if="detail" class="fixed inset-0 z-50 flex">
        <div class="flex-1 bg-white/70 dark:bg-zinc-950/70 backdrop-blur-sm" @click="detail = null" />
        <aside class="w-full max-w-md bg-zinc-100 dark:bg-zinc-900 ring-1 ring-black/10 dark:ring-white/10 overflow-y-auto">
          <div class="p-5 border-b border-black/[0.08] dark:border-white/[0.06] flex items-center justify-between">
            <h2 class="text-base font-bold">{{ detail.userName }}</h2>
            <button @click="detail = null"
              class="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              <X class="w-4 h-4" />
            </button>
          </div>

          <div class="p-5 space-y-4">
            <div class="grid grid-cols-2 gap-3 text-sm">
              <div>
                <p class="text-[10px] text-zinc-500 tracking-wide uppercase mb-1">学号</p>
                <p class="font-mono-token text-zinc-700 dark:text-zinc-300">{{ detail.userNumber }}</p>
              </div>
              <div>
                <p class="text-[10px] text-zinc-500 tracking-wide uppercase mb-1">邀请码</p>
                <p class="font-mono-token text-zinc-700 dark:text-zinc-300">{{ detail.inviteCode || '—' }}</p>
              </div>
              <div>
                <p class="text-[10px] text-zinc-500 tracking-wide uppercase mb-1">学院</p>
                <p class="text-zinc-700 dark:text-zinc-300">{{ detail.userSection }}</p>
              </div>
              <div>
                <p class="text-[10px] text-zinc-500 tracking-wide uppercase mb-1">班级</p>
                <p class="text-zinc-700 dark:text-zinc-300">{{ detail.userClass }}</p>
              </div>
              <div>
                <p class="text-[10px] text-zinc-500 tracking-wide uppercase mb-1">宿舍楼</p>
                <p class="text-zinc-700 dark:text-zinc-300 text-xs">
                  {{ detail.dormName || '未绑定' }}
                </p>
              </div>
              <div>
                <p class="text-[10px] text-zinc-500 tracking-wide uppercase mb-1">坐标</p>
                <p class="font-mono-token text-zinc-700 dark:text-zinc-300 text-xs tabular-nums">
                  {{ detail.latitude && detail.longitude
                    ? `${detail.latitude.toFixed(5)}, ${detail.longitude.toFixed(5)}`
                    : '—' }}
                </p>
              </div>
              <div>
                <p class="text-[10px] text-zinc-500 tracking-wide uppercase mb-1">Token 到期</p>
                <p class="text-xs text-zinc-700 dark:text-zinc-300 tabular-nums">
                  {{ formatDateTime(detail.tokenExp) }}
                </p>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-2 pt-2 border-t border-black/[0.08] dark:border-white/[0.06]">
              <button @click="toggleDisabled(detail)"
                class="inline-flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg ring-1 transition-colors text-sm"
                :class="detail.isDisabled
                  ? 'bg-emerald-500/15 ring-emerald-500/30 text-emerald-300 hover:bg-emerald-500/20'
                  : 'bg-amber-500/15 ring-amber-500/30 text-amber-300 hover:bg-amber-500/20'">
                <Power class="w-3.5 h-3.5" />
                {{ detail.isDisabled ? '启用账号' : '禁用账号' }}
              </button>
              <button @click="resetPin(detail)" :disabled="resetting"
                class="inline-flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg bg-blue-500/15 ring-1 ring-blue-500/30 hover:bg-blue-500/20 disabled:opacity-50 text-blue-300 transition-colors text-sm">
                <KeyRound class="w-3.5 h-3.5" />
                {{ resetting ? '重置中…' : '重置 PIN' }}
              </button>
              <button @click="remove(detail)"
                class="col-span-2 inline-flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg bg-red-500/15 ring-1 ring-red-500/30 hover:bg-red-500/20 text-red-300 transition-colors text-sm">
                <Trash2 class="w-3.5 h-3.5" />
                删除账号
              </button>
            </div>

            <div class="pt-2">
              <p class="text-xs text-zinc-500 tracking-wide uppercase mb-3">最近签到</p>
              <div v-if="loadingDetail" class="flex justify-center py-6">
                <div class="h-5 w-5 rounded-full border-2 border-zinc-800 border-t-emerald-400 wangui-spin" />
              </div>
              <div v-else-if="!detail.recentRecords?.length" class="text-xs text-zinc-500 py-4 text-center">
                还没有记录
              </div>
              <ol v-else class="relative">
                <div class="absolute left-[7px] top-1.5 bottom-1.5 w-px bg-gradient-to-b from-transparent via-black/[0.06] dark:via-white/[0.06] to-transparent" />
                <li v-for="r in detail.recentRecords" :key="r.id" class="relative pl-6 pb-2.5 last:pb-0">
                  <span class="absolute left-0 top-1 w-3.5 h-3.5 rounded-full ring-4 ring-white dark:ring-zinc-900"
                    :class="(recordMeta[r.status] || recordMeta.failed).dotBg" />
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0 flex-1">
                      <span class="text-xs font-medium" :class="(recordMeta[r.status] || recordMeta.failed).color">
                        {{ (recordMeta[r.status] || recordMeta.failed).label }}
                      </span>
                      <p class="text-[11px] text-zinc-500 mt-0.5 break-all">{{ r.message || '—' }}</p>
                    </div>
                    <span class="shrink-0 text-[10px] text-zinc-500 dark:text-zinc-600 tabular-nums">{{ formatDateTime(r.occurredAt) }}</span>
                  </div>
                </li>
              </ol>
            </div>
          </div>
        </aside>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.drawer-enter-active, .drawer-leave-active { transition: opacity 0.2s ease; }
.drawer-enter-active aside, .drawer-leave-active aside { transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1); }
.drawer-enter-from, .drawer-leave-to { opacity: 0; }
.drawer-enter-from aside, .drawer-leave-to aside { transform: translateX(100%); }
.modal-enter-active, .modal-leave-active { transition: opacity 0.2s; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
</style>
