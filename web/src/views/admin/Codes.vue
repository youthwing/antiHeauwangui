<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  Plus,
  Copy,
  Trash2,
  Pause,
  Play,
  Search,
  X,
  RefreshCw,
} from 'lucide-vue-next'
import type { InviteCode } from '../../types'
import { adminApi } from '../../api'
import { formatDateTime } from '../../lib/format'
import { showToast } from '../../lib/toast'
import { copyText } from '../../lib/clipboard'

const codes = ref<InviteCode[]>([])
const loading = ref(false)
const filter = ref<'all' | 'used' | 'unused'>('all')
const search = ref('')

const showGen = ref(false)
const genCount = ref(5)
const genNote = ref('')
const generating = ref(false)
const newlyGenerated = ref<InviteCode[]>([])

async function load() {
  loading.value = true
  try {
    codes.value = await adminApi.listCodes({
      status: filter.value === 'all' ? undefined : filter.value,
      search: search.value.trim() || undefined,
      limit: 200,
    })
  } catch (e: any) {
    showToast('err', e.message || '加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function generate() {
  generating.value = true
  try {
    const res = await adminApi.createCodes(genCount.value, genNote.value)
    newlyGenerated.value = res
    showToast('ok', `生成了 ${res.length} 张邀请码`)
    await load()
  } catch (e: any) {
    showToast('err', e.message || '生成失败')
  } finally {
    generating.value = false
  }
}

async function copy(s: string) {
  const ok = await copyText(s)
  showToast(ok ? 'ok' : 'err', ok ? '已复制' : '复制失败，请手动选取')
}

async function copyAll() {
  const ok = await copyText(newlyGenerated.value.map(c => c.code).join('\n'))
  showToast(ok ? 'ok' : 'err', ok ? `${newlyGenerated.value.length} 张已复制` : '复制失败，请手动选取')
}

async function toggleDisabled(c: InviteCode) {
  try {
    await adminApi.updateCode(c.code, { disabled: !c.disabled })
    showToast('ok', c.disabled ? '已启用' : '已禁用')
    await load()
  } catch (e: any) {
    showToast('err', e.message || '操作失败')
  }
}

async function saveNote(c: InviteCode, note: string) {
  try {
    await adminApi.updateCode(c.code, { note })
    showToast('ok', '已保存')
    await load()
  } catch (e: any) {
    showToast('err', e.message || '保存失败')
  }
}

async function remove(c: InviteCode) {
  if (!confirm(`删除邀请码 ${c.code}？此操作不可恢复。`)) return
  try {
    await adminApi.deleteCode(c.code)
    showToast('ok', '已删除')
    await load()
  } catch (e: any) {
    showToast('err', e.message || '删除失败')
  }
}

const filtered = computed(() => codes.value)

let editTimer: number | null = null
function debouncedSaveNote(c: InviteCode, v: string) {
  if (editTimer) clearTimeout(editTimer)
  editTimer = window.setTimeout(() => saveNote(c, v), 600)
}
</script>

<template>
  <div class="space-y-3">
    <header class="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">邀请码管理</h1>
        <p class="text-sm text-zinc-500 mt-1">生成、查看、禁用邀请码。</p>
      </div>
      <button
        @click="showGen = true"
        class="self-start inline-flex items-center gap-1.5 bg-emerald-500 hover:bg-emerald-400 text-zinc-950 text-sm font-medium px-4 py-2 rounded-lg transition-colors"
      >
        <Plus class="w-4 h-4" />
        生成邀请码
      </button>
    </header>

    <!-- Filter row -->
    <div class="flex flex-wrap items-center gap-3">
      <div class="relative flex-1 min-w-[200px] max-w-md">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-zinc-500" />
        <input
          v-model="search"
          @keyup.enter="load"
          placeholder="搜索邀请码 / 备注"
          class="w-full pl-9 pr-3 py-2 bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg text-sm focus-ring text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600"
        />
      </div>
      <div class="flex gap-1">
        <button
          v-for="(o, k) in { all: '全部', unused: '未用', used: '已用' }"
          :key="k"
          @click="filter = k as any; load()"
          :class="filter === k
            ? 'bg-emerald-500/20 text-emerald-300 ring-1 ring-emerald-500/30'
            : 'bg-white/85 dark:bg-zinc-900/60 text-zinc-500 dark:text-zinc-400 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:text-zinc-900 dark:hover:text-zinc-200'"
          class="text-xs px-3 py-1.5 rounded-md transition-colors"
        >{{ o }}</button>
      </div>
      <button
        @click="load"
        :disabled="loading"
        class="text-xs text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 px-2 py-1.5 rounded-md hover:bg-black/5 dark:hover:bg-white/5 transition-colors inline-flex items-center gap-1"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="loading ? 'wangui-spin' : ''" />
      </button>
    </div>

    <!-- Table -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="bg-white/50 dark:bg-zinc-950/50 border-b border-black/[0.08] dark:border-white/[0.06]">
            <tr class="text-left text-[10px] text-zinc-500 uppercase tracking-wide">
              <th class="px-4 py-3 font-medium">邀请码</th>
              <th class="px-4 py-3 font-medium">状态</th>
              <th class="px-4 py-3 font-medium">绑定用户</th>
              <th class="px-4 py-3 font-medium">备注</th>
              <th class="px-4 py-3 font-medium">创建</th>
              <th class="px-4 py-3 font-medium text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-black/[0.05] dark:divide-white/[0.04]">
            <tr v-if="loading && filtered.length === 0">
              <td colspan="6" class="px-4 py-10 text-center">
                <div class="h-5 w-5 rounded-full border-2 border-zinc-800 border-t-emerald-400 wangui-spin mx-auto" />
              </td>
            </tr>
            <tr v-else-if="filtered.length === 0">
              <td colspan="6" class="px-4 py-10 text-center text-sm text-zinc-500">
                还没有邀请码，点右上角"生成邀请码"
              </td>
            </tr>
            <tr v-for="c in filtered" :key="c.code" class="hover:bg-black/[0.02] dark:hover:bg-white/[0.02] transition-colors">
              <td class="px-4 py-3 font-mono-token text-zinc-900 dark:text-zinc-200 tracking-wider">{{ c.code }}</td>
              <td class="px-4 py-3">
                <span v-if="c.disabled"
                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs bg-zinc-300/50 dark:bg-zinc-700/50 text-zinc-500 dark:text-zinc-400">
                  已禁用
                </span>
                <span v-else-if="c.used"
                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs bg-emerald-500/15 text-emerald-400 ring-1 ring-emerald-500/30">
                  已用
                </span>
                <span v-else
                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs bg-blue-500/15 text-blue-400 ring-1 ring-blue-500/30">
                  待用
                </span>
              </td>
              <td class="px-4 py-3 text-xs">
                <template v-if="c.boundUserId">
                  <span class="text-zinc-900 dark:text-zinc-200">{{ c.boundUserName || '—' }}</span>
                  <span class="ml-1.5 text-zinc-500 dark:text-zinc-600 font-mono-token">({{ c.boundUserId }})</span>
                </template>
                <span v-else class="text-zinc-500 dark:text-zinc-600">—</span>
              </td>
              <td class="px-4 py-3">
                <input
                  :value="c.note"
                  @input="(e: any) => debouncedSaveNote(c, e.target.value)"
                  placeholder="—"
                  class="w-32 bg-transparent border-none px-1 py-0.5 text-zinc-700 dark:text-zinc-300 focus:outline-none focus:bg-zinc-950 focus:ring-1 focus:ring-emerald-500/30 rounded text-xs"
                />
              </td>
              <td class="px-4 py-3 text-xs text-zinc-500 tabular-nums">
                {{ formatDateTime(c.createdAt) }}
              </td>
              <td class="px-4 py-3 text-right">
                <div class="inline-flex gap-0.5">
                  <button @click="copy(c.code)" title="复制"
                    class="p-1.5 rounded hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 transition-colors">
                    <Copy class="w-3.5 h-3.5" />
                  </button>
                  <button @click="toggleDisabled(c)" :title="c.disabled ? '启用' : '禁用'"
                    class="p-1.5 rounded hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 dark:text-zinc-400 hover:text-amber-400 transition-colors">
                    <component :is="c.disabled ? Play : Pause" class="w-3.5 h-3.5" />
                  </button>
                  <button v-if="!c.used" @click="remove(c)" title="删除"
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

    <!-- Generate modal -->
    <Transition name="modal">
      <div v-if="showGen" class="fixed inset-0 z-50 bg-white/80 dark:bg-zinc-950/80 backdrop-blur flex items-center justify-center p-4"
        @click.self="showGen = false; newlyGenerated = []">
        <div class="w-full max-w-md bg-zinc-100 dark:bg-zinc-900 ring-1 ring-black/10 dark:ring-white/10 rounded-2xl shadow-2xl">
          <div class="p-5 border-b border-black/[0.08] dark:border-white/[0.06] flex items-center justify-between">
            <h2 class="text-base font-bold">
              {{ newlyGenerated.length ? '已生成' : '生成邀请码' }}
            </h2>
            <button @click="showGen = false; newlyGenerated = []"
              class="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              <X class="w-4 h-4" />
            </button>
          </div>

          <div v-if="newlyGenerated.length === 0" class="p-5 space-y-4">
            <div>
              <label class="block text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">数量 (1–50)</label>
              <input v-model.number="genCount" type="number" min="1" max="50"
                class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200" />
            </div>
            <div>
              <label class="block text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">备注 (可选)</label>
              <input v-model="genNote" placeholder='例如"2026春批次给XX等"'
                class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600" />
            </div>
            <button @click="generate" :disabled="generating || genCount < 1"
              class="w-full bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 text-zinc-950 font-medium py-2.5 rounded-xl transition-colors">
              {{ generating ? '生成中…' : `生成 ${genCount} 张` }}
            </button>
          </div>

          <div v-else class="p-5">
            <p class="text-xs text-zinc-500 dark:text-zinc-400 mb-3">点邀请码一键复制 · 或点底部全部复制</p>
            <ul class="space-y-1.5 max-h-72 overflow-y-auto">
              <li v-for="c in newlyGenerated" :key="c.code">
                <button @click="copy(c.code)"
                  class="w-full text-left px-3 py-2 rounded-lg bg-white/70 dark:bg-zinc-950/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04] hover:ring-emerald-500/40 font-mono-token tracking-wider text-zinc-900 dark:text-zinc-200 transition-all">
                  {{ c.code }}
                </button>
              </li>
            </ul>
            <button @click="copyAll"
              class="mt-4 w-full bg-emerald-500 hover:bg-emerald-400 text-zinc-950 font-medium py-2 rounded-xl transition-colors inline-flex items-center justify-center gap-2">
              <Copy class="w-4 h-4" />
              复制全部
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.modal-enter-active, .modal-leave-active { transition: opacity 0.2s; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
</style>
