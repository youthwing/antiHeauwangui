<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import QRCode from 'qrcode'
import {
  Plus,
  X,
  Trash2,
  Pencil,
  RefreshCw,
  UserPlus,
  Calendar,
  Building2,
  Check,
  AlertCircle,
  QrCode as QrCodeIcon,
} from 'lucide-vue-next'
import type { AdminGuest, Dorm } from '../../types'
import { adminApi } from '../../api'
import { formatDateTime } from '../../lib/format'
import { showToast } from '../../lib/toast'
import {
  buildWechatOauthAuthorizeUrl,
  createWechatOauthState,
  detectSchoolOauthInput,
} from '../../lib/schoolOauth'

const guests = ref<AdminGuest[]>([])
const dorms = ref<Dorm[]>([])
const loading = ref(false)

// --- create modal state ---
const showCreate = ref(false)
const creating = ref(false)
const cLabel = ref('')
const cDormId = ref<number | 0>(0)
const cDates = ref<string[]>([defaultTodayPlus(1)])
const cCallback = ref('')
const cQrDataUrl = ref('')
const cQrBuilding = ref(false)
const cQrState = ref(createWechatOauthState())

// --- edit modal state ---
const editTarget = ref<AdminGuest | null>(null)
const eLabel = ref('')
const eDates = ref<string[]>([])
const updating = ref(false)

function defaultTodayPlus(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() + n)
  return d.toISOString().slice(0, 10)
}

async function load() {
  loading.value = true
  try {
    const [g, d] = await Promise.all([adminApi.listGuests(), adminApi.listDorms()])
    guests.value = g
    dorms.value = d as unknown as Dorm[]
  } catch (e: any) {
    showToast('err', e.message || '加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(load)

// --- create flow ---
function openCreate() {
  cLabel.value = ''
  cDormId.value = 0
  cDates.value = [defaultTodayPlus(1)]
  cCallback.value = ''
  cQrState.value = createWechatOauthState()
  showCreate.value = true
  refreshQr()
}

async function refreshQr() {
  cQrBuilding.value = true
  try {
    cQrState.value = createWechatOauthState()
    cQrDataUrl.value = await QRCode.toDataURL(
      buildWechatOauthAuthorizeUrl(cQrState.value),
      { width: 240, margin: 1, errorCorrectionLevel: 'M' },
    )
  } catch (e: any) {
    cQrDataUrl.value = ''
    showToast('err', e?.message || '二维码生成失败')
  } finally {
    cQrBuilding.value = false
  }
}

const cCallbackDetect = computed(() => detectSchoolOauthInput(cCallback.value))
const cCallbackOk = computed(
  () => cCallbackDetect.value.kind === 'code' || cCallbackDetect.value.kind === 'callback-url',
)

function addCreateDate() {
  cDates.value.push(defaultTodayPlus(cDates.value.length + 1))
}
function removeCreateDate(i: number) {
  cDates.value.splice(i, 1)
}

const cCanSubmit = computed(() => {
  if (creating.value) return false
  if (!cLabel.value.trim()) return false
  if (cDates.value.filter(d => d.trim()).length === 0) return false
  if (!cCallbackOk.value) return false
  return true
})

async function submitCreate() {
  if (!cCanSubmit.value) return
  creating.value = true
  try {
    const payload = {
      label: cLabel.value.trim(),
      signDates: cDates.value.filter(d => d.trim()),
      dormId: cDormId.value || undefined,
      callbackUrl: cCallback.value.trim(),
    }
    await adminApi.createGuest(payload)
    showToast('ok', '已添加临时朋友')
    showCreate.value = false
    await load()
  } catch (e: any) {
    showToast('err', e.message || '添加失败')
  } finally {
    creating.value = false
  }
}

// --- edit flow ---
function openEdit(g: AdminGuest) {
  editTarget.value = g
  eLabel.value = g.label
  eDates.value = [...g.signDates]
}

function addEditDate() {
  eDates.value.push(defaultTodayPlus(eDates.value.length + 1))
}
function removeEditDate(i: number) {
  eDates.value.splice(i, 1)
}

async function submitEdit() {
  if (!editTarget.value) return
  updating.value = true
  try {
    await adminApi.updateGuest(editTarget.value.userId, {
      label: eLabel.value.trim(),
      signDates: eDates.value.filter(d => d.trim()),
    })
    showToast('ok', '已更新')
    editTarget.value = null
    await load()
  } catch (e: any) {
    showToast('err', e.message || '保存失败')
  } finally {
    updating.value = false
  }
}

// --- delete ---
async function remove(g: AdminGuest) {
  if (!confirm(`删除临时朋友「${g.label}」(${g.userName})？\n该账号会立即销毁，签到记录一并删除。`)) return
  try {
    await adminApi.deleteGuest(g.userId)
    showToast('ok', '已删除')
    await load()
  } catch (e: any) {
    showToast('err', e.message || '删除失败')
  }
}

// --- helpers ---
function daysLeft(expiresAt: number | null): string {
  if (!expiresAt) return '永久'
  const now = Math.floor(Date.now() / 1000)
  if (expiresAt < now) return '已过期'
  const days = Math.ceil((expiresAt - now) / 86400)
  return `${days} 天剩余`
}

function daysLeftClass(expiresAt: number | null): string {
  if (!expiresAt) return 'text-zinc-500'
  const now = Math.floor(Date.now() / 1000)
  if (expiresAt < now) return 'text-red-400'
  const days = Math.ceil((expiresAt - now) / 86400)
  if (days <= 1) return 'text-amber-400'
  return 'text-emerald-400'
}

function summarizeDates(dates: string[]): string {
  if (dates.length === 0) return '—'
  if (dates.length === 1) return dates[0]
  return `${dates[0]} → ${dates[dates.length - 1]} (${dates.length} 天)`
}

watch(showCreate, v => {
  if (!v) cCallback.value = ''
})
</script>

<template>
  <div class="space-y-3">
    <header class="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">临时朋友</h1>
        <p class="text-sm text-zinc-500 mt-1">admin 代为创建 / 配置签到日期 / 用完自动删除。</p>
      </div>
      <button
        @click="openCreate"
        class="self-start inline-flex items-center gap-1.5 bg-emerald-500 hover:bg-emerald-400 text-zinc-950 text-sm font-medium px-4 py-2 rounded-lg transition-colors"
      >
        <Plus class="w-4 h-4" />
        新增临时朋友
      </button>
    </header>

    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="bg-white/50 dark:bg-zinc-950/50 border-b border-black/[0.08] dark:border-white/[0.06]">
            <tr class="text-left text-[10px] text-zinc-500 uppercase tracking-wide">
              <th class="px-4 py-3 font-medium">标签</th>
              <th class="px-4 py-3 font-medium">姓名 / 学号</th>
              <th class="px-4 py-3 font-medium">宿舍楼</th>
              <th class="px-4 py-3 font-medium">签到日期</th>
              <th class="px-4 py-3 font-medium">剩余</th>
              <th class="px-4 py-3 font-medium text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-black/[0.05] dark:divide-white/[0.04]">
            <tr v-if="loading && guests.length === 0">
              <td colspan="6" class="px-4 py-10 text-center">
                <div class="h-5 w-5 rounded-full border-2 border-zinc-800 border-t-emerald-400 wangui-spin mx-auto" />
              </td>
            </tr>
            <tr v-else-if="guests.length === 0">
              <td colspan="6" class="px-4 py-12 text-center text-sm text-zinc-500">
                还没有临时朋友，点右上角「新增」
              </td>
            </tr>
            <tr v-for="g in guests" :key="g.userId" class="hover:bg-black/[0.02] dark:hover:bg-white/[0.02] transition-colors">
              <td class="px-4 py-3 font-medium">{{ g.label || '—' }}</td>
              <td class="px-4 py-3 text-xs">
                <span class="text-zinc-900 dark:text-zinc-200">{{ g.userName }}</span>
                <span class="ml-1.5 text-zinc-500 font-mono-token">/ {{ g.userNumber }}</span>
              </td>
              <td class="px-4 py-3 text-xs">
                <span v-if="g.dormName"
                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-1 ring-emerald-500/25">
                  {{ g.dormName }}
                </span>
                <span v-else class="text-zinc-500">未绑定</span>
              </td>
              <td class="px-4 py-3 text-xs font-mono-token text-zinc-500 dark:text-zinc-400">
                {{ summarizeDates(g.signDates) }}
              </td>
              <td class="px-4 py-3 text-xs font-medium" :class="daysLeftClass(g.expiresAt)">
                {{ daysLeft(g.expiresAt) }}
              </td>
              <td class="px-4 py-3 text-right">
                <div class="inline-flex gap-0.5">
                  <button @click="openEdit(g)" title="续期/改标签"
                    class="p-1.5 rounded hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 dark:text-zinc-400 hover:text-emerald-400 transition-colors">
                    <Pencil class="w-3.5 h-3.5" />
                  </button>
                  <button @click="remove(g)" title="删除"
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

    <!-- Create modal -->
    <Transition name="modal">
      <div v-if="showCreate" class="fixed inset-0 z-50 bg-white/80 dark:bg-zinc-950/80 backdrop-blur flex items-center justify-center p-4 overflow-y-auto"
        @click.self="showCreate = false">
        <div class="w-full max-w-2xl bg-zinc-100 dark:bg-zinc-900 ring-1 ring-black/10 dark:ring-white/10 rounded-2xl shadow-2xl my-8">
          <div class="p-5 border-b border-black/[0.08] dark:border-white/[0.06] flex items-center justify-between">
            <h2 class="text-base font-bold flex items-center gap-2">
              <UserPlus class="w-4 h-4 text-emerald-400" />
              新增临时朋友
            </h2>
            <button @click="showCreate = false"
              class="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              <X class="w-4 h-4" />
            </button>
          </div>

          <div class="p-5 space-y-4">
            <!-- Label + Dorm -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label class="block text-xs text-zinc-500 mb-1.5">备注名 *</label>
                <input v-model="cLabel" placeholder="如「张三 5/20」"
                  class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600" />
              </div>
              <div>
                <label class="block text-xs text-zinc-500 mb-1.5 inline-flex items-center gap-1">
                  <Building2 class="w-3 h-3" />
                  宿舍楼
                </label>
                <select v-model="cDormId"
                  class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200">
                  <option :value="0">不绑定</option>
                  <option v-for="d in dorms" :key="d.id" :value="d.id">{{ d.name }}</option>
                </select>
              </div>
            </div>

            <!-- Dates -->
            <div>
              <label class="text-xs text-zinc-500 inline-flex items-center gap-1 mb-2">
                <Calendar class="w-3 h-3" />
                签到日期 *（选哪几天 22:00 帮他签）
              </label>
              <div class="space-y-1.5">
                <div v-for="(_, i) in cDates" :key="i" class="flex items-center gap-2">
                  <input type="date" v-model="cDates[i]"
                    class="flex-1 bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-1.5 text-sm font-mono-token focus-ring text-zinc-900 dark:text-zinc-200" />
                  <button type="button" @click="removeCreateDate(i)" :disabled="cDates.length === 1"
                    class="p-1.5 rounded hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 hover:text-red-400 disabled:opacity-30 disabled:hover:text-zinc-500 transition-colors">
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
                <button type="button" @click="addCreateDate"
                  class="text-[11px] text-emerald-400 hover:text-emerald-300 inline-flex items-center gap-1">
                  <Plus class="w-3 h-3" />
                  再加一天
                </button>
              </div>
            </div>

            <!-- QR -->
            <div class="rounded-xl bg-white/70 dark:bg-zinc-950/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04] p-3">
              <div class="flex items-center gap-2 mb-2">
                <QrCodeIcon class="w-3.5 h-3.5 text-zinc-500" />
                <span class="text-xs text-zinc-500">把这个二维码截图发给朋友</span>
              </div>
              <div class="flex flex-col sm:flex-row gap-3">
                <div class="shrink-0 self-center sm:self-start">
                  <div class="w-36 h-36 rounded-xl bg-white ring-1 ring-black/[0.06] p-2 flex items-center justify-center overflow-hidden">
                    <img v-if="cQrDataUrl" :src="cQrDataUrl" alt="二维码" class="w-full h-full object-contain" />
                    <div v-else class="text-[11px] text-zinc-400 text-center">{{ cQrBuilding ? '生成中…' : '失败' }}</div>
                  </div>
                </div>
                <div class="min-w-0 flex-1">
                  <ol class="text-[11px] text-zinc-600 dark:text-zinc-400 space-y-1 list-decimal list-inside leading-relaxed">
                    <li>截图二维码 → 微信发给朋友</li>
                    <li>朋友<strong>微信</strong>扫码 → 学校晚归页面 → 正常登录</li>
                    <li>登录成功后，朋友点页面<strong>右上角「⋯」</strong>→ <strong>「复制链接」</strong></li>
                    <li>朋友把链接发回给你，你粘到下面 ↓</li>
                  </ol>
                  <button type="button" @click="refreshQr"
                    class="mt-2 inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] text-zinc-700 dark:text-zinc-300 bg-white/80 dark:bg-zinc-900/80 ring-1 ring-black/[0.06] dark:ring-white/[0.05] hover:ring-emerald-500/40 transition-colors">
                    <RefreshCw class="w-3 h-3" />
                    刷新二维码
                  </button>
                </div>
              </div>
            </div>

            <!-- Callback paste -->
            <div>
              <label class="block text-xs text-zinc-500 mb-1.5">把朋友给的回调链接粘到这里 *</label>
              <textarea v-model="cCallback"
                placeholder="https://xhbcs.henau.edu.cn/?code=...&state=..."
                class="w-full bg-white dark:bg-zinc-950 ring-1 ring-emerald-500/30 focus:!ring-emerald-500/60 rounded-lg px-3 py-2 h-20 resize-none text-sm font-mono-token text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 focus-ring" />
              <div v-if="cCallbackDetect.kind === 'callback-url'"
                class="mt-1.5 inline-flex items-center gap-1.5 px-2 py-1 rounded text-[11px] text-emerald-400 bg-emerald-500/10">
                <Check class="w-3 h-3" />
                已识别回调链接
              </div>
              <div v-else-if="cCallbackDetect.kind === 'code'"
                class="mt-1.5 inline-flex items-center gap-1.5 px-2 py-1 rounded text-[11px] text-emerald-400 bg-emerald-500/10">
                <Check class="w-3 h-3" />
                已识别 code
              </div>
              <div v-else-if="cCallbackDetect.kind === 'invalid' && cCallback.trim()"
                class="mt-1.5 inline-flex items-center gap-1.5 px-2 py-1 rounded text-[11px] text-amber-400 bg-amber-500/10">
                <AlertCircle class="w-3 h-3" />
                没识别到 code，请检查链接
              </div>
            </div>
          </div>

          <div class="px-5 pb-5 flex justify-end gap-2">
            <button @click="showCreate = false"
              class="px-4 py-2 text-sm text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              取消
            </button>
            <button @click="submitCreate" :disabled="!cCanSubmit"
              class="bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 text-zinc-950 text-sm font-medium px-5 py-2 rounded-lg transition-colors">
              {{ creating ? '创建中…' : '创建临时朋友' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Edit modal -->
    <Transition name="modal">
      <div v-if="editTarget" class="fixed inset-0 z-50 bg-white/80 dark:bg-zinc-950/80 backdrop-blur flex items-center justify-center p-4"
        @click.self="editTarget = null">
        <div class="w-full max-w-md bg-zinc-100 dark:bg-zinc-900 ring-1 ring-black/10 dark:ring-white/10 rounded-2xl shadow-2xl">
          <div class="p-5 border-b border-black/[0.08] dark:border-white/[0.06] flex items-center justify-between">
            <h2 class="text-base font-bold">编辑临时朋友</h2>
            <button @click="editTarget = null"
              class="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              <X class="w-4 h-4" />
            </button>
          </div>
          <div class="p-5 space-y-4">
            <div>
              <p class="text-[11px] text-zinc-500">姓名 / 学号</p>
              <p class="text-sm">
                <span>{{ editTarget.userName }}</span>
                <span class="ml-1.5 text-zinc-500 font-mono-token">/ {{ editTarget.userNumber }}</span>
              </p>
            </div>
            <div>
              <label class="block text-xs text-zinc-500 mb-1.5">备注名</label>
              <input v-model="eLabel"
                class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200" />
            </div>
            <div>
              <label class="text-xs text-zinc-500 inline-flex items-center gap-1 mb-2">
                <Calendar class="w-3 h-3" />
                签到日期
              </label>
              <div class="space-y-1.5">
                <div v-for="(_, i) in eDates" :key="i" class="flex items-center gap-2">
                  <input type="date" v-model="eDates[i]"
                    class="flex-1 bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-1.5 text-sm font-mono-token focus-ring text-zinc-900 dark:text-zinc-200" />
                  <button type="button" @click="removeEditDate(i)" :disabled="eDates.length === 1"
                    class="p-1.5 rounded hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 hover:text-red-400 disabled:opacity-30 transition-colors">
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
                <button type="button" @click="addEditDate"
                  class="text-[11px] text-emerald-400 hover:text-emerald-300 inline-flex items-center gap-1">
                  <Plus class="w-3 h-3" />
                  再加一天（续期）
                </button>
              </div>
            </div>
          </div>
          <div class="px-5 pb-5 flex justify-end gap-2">
            <button @click="editTarget = null"
              class="px-4 py-2 text-sm text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              取消
            </button>
            <button @click="submitEdit" :disabled="updating"
              class="bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 text-zinc-950 text-sm font-medium px-5 py-2 rounded-lg transition-colors">
              {{ updating ? '保存中…' : '保存' }}
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
