<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  Megaphone,
  Plus,
  X,
  Pencil,
  Trash2,
  RefreshCw,
  Save,
  Eye,
  Info,
  CheckCircle2,
  AlertTriangle,
  AlertOctagon,
} from 'lucide-vue-next'
import type { Announcement, AnnouncementLevel } from '../../types'
import { adminApi } from '../../api'
import { showToast } from '../../lib/toast'
import { formatDateTime } from '../../lib/format'
import AnnouncementCard from '../../components/AnnouncementCard.vue'

const list = ref<Announcement[]>([])
const loading = ref(false)

// Editor state. `editId` null means a fresh "新建" form; otherwise we're
// editing that id. The same panel is reused so the layout doesn't jump.
const editId = ref<number | null>(null)
const title = ref('')
const content = ref('')
const level = ref<AnnouncementLevel>('info')
const expiresDate = ref('') // YYYY-MM-DD; empty = never expires
const saving = ref(false)
const showHelp = ref(false)

const LEVEL_OPTIONS: Array<{ key: AnnouncementLevel; label: string; icon: any; toneClass: string }> = [
  { key: 'info', label: '通知', icon: Info, toneClass: 'text-blue-500' },
  { key: 'success', label: '喜报', icon: CheckCircle2, toneClass: 'text-emerald-500' },
  { key: 'warning', label: '注意', icon: AlertTriangle, toneClass: 'text-amber-500' },
  { key: 'critical', label: '紧急', icon: AlertOctagon, toneClass: 'text-red-500' },
]

async function load() {
  loading.value = true
  try {
    list.value = await adminApi.listAnnouncements()
  } catch (e: any) {
    showToast('err', e.message || '加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(load)

function resetForm() {
  editId.value = null
  title.value = ''
  content.value = ''
  level.value = 'info'
  expiresDate.value = ''
}

function startEdit(a: Announcement) {
  editId.value = a.id
  title.value = a.title
  content.value = a.content
  level.value = a.level
  if (a.expiresAt) {
    const d = new Date(a.expiresAt * 1000)
    expiresDate.value =
      d.getFullYear() +
      '-' +
      String(d.getMonth() + 1).padStart(2, '0') +
      '-' +
      String(d.getDate()).padStart(2, '0')
  } else {
    expiresDate.value = ''
  }
  // Scroll the editor into view on small screens since it lives at the top.
  if (typeof window !== 'undefined') {
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }
}

const canSubmit = computed(() => {
  return !saving.value && title.value.trim().length > 0 && content.value.trim().length > 0
})

function expiresUnix(): number | null {
  if (!expiresDate.value) return null
  // Use end-of-day so the date the admin picked is fully included.
  const d = new Date(expiresDate.value + 'T23:59:59')
  if (isNaN(d.getTime())) return null
  return Math.floor(d.getTime() / 1000)
}

async function submit() {
  if (!canSubmit.value) return
  saving.value = true
  try {
    const body = {
      title: title.value.trim(),
      content: content.value.trim(),
      level: level.value,
      expiresAt: expiresUnix(),
    }
    if (editId.value == null) {
      await adminApi.createAnnouncement(body)
      showToast('ok', '公告已发布')
    } else {
      await adminApi.updateAnnouncement(editId.value, body)
      showToast('ok', '公告已更新')
    }
    resetForm()
    await load()
  } catch (e: any) {
    showToast('err', e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove(a: Announcement) {
  if (!confirm(`删除公告「${a.title}」？\n用户端立即不再可见。`)) return
  try {
    await adminApi.deleteAnnouncement(a.id)
    if (editId.value === a.id) resetForm()
    showToast('ok', '已删除')
    await load()
  } catch (e: any) {
    showToast('err', e.message || '删除失败')
  }
}

// Live preview uses the same card component as the Dashboard, so what
// admin sees in the editor === what users see after publish.
const previewAnnouncement = computed<Announcement>(() => ({
  id: editId.value ?? -1,
  title: title.value || '(还没填标题)',
  content: content.value || '(还没填内容)',
  level: level.value,
  expiresAt: expiresUnix(),
  createdAt: Math.floor(Date.now() / 1000),
  updatedAt: Math.floor(Date.now() / 1000),
}))

function fmtDate(t: number | null): string {
  if (!t) return '—'
  return formatDateTime(t)
}
</script>

<template>
  <div class="space-y-4">
    <header class="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold tracking-tight flex items-center gap-2">
          <Megaphone class="w-5 h-5 text-amber-400" />
          公告
        </h1>
        <p class="text-sm text-zinc-500 mt-1">
          只需填<strong>标题</strong> + <strong>内容</strong>，模板自动美化；用户 Dashboard 顶部立即可见。
        </p>
      </div>
      <button @click="load" :disabled="loading"
        class="self-start text-xs text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200 px-2 py-2 rounded-md hover:bg-black/5 dark:hover:bg-white/5 transition-colors inline-flex items-center gap-1">
        <RefreshCw class="w-3.5 h-3.5" :class="loading ? 'wangui-spin' : ''" />
      </button>
    </header>

    <!-- Editor: form on the left, live preview on the right. Stacks on small screens. -->
    <section class="grid grid-cols-1 lg:grid-cols-2 gap-3">
      <!-- Form panel -->
      <div class="rounded-2xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5 space-y-4">
        <div class="flex items-center justify-between gap-2">
          <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">
            {{ editId == null ? '新建公告' : `编辑 #${editId}` }}
          </h2>
          <button
            v-if="editId != null"
            @click="resetForm"
            class="text-[11px] text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200 inline-flex items-center gap-1"
          >
            <Plus class="w-3 h-3" />
            改新建
          </button>
        </div>

        <!-- Level selector -->
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1.5">级别</label>
          <div class="grid grid-cols-4 gap-1.5">
            <button
              v-for="opt in LEVEL_OPTIONS"
              :key="opt.key"
              type="button"
              @click="level = opt.key"
              :class="level === opt.key
                ? 'bg-zinc-900 dark:bg-zinc-100 text-zinc-100 dark:text-zinc-900'
                : 'bg-white dark:bg-zinc-950 text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200'"
              class="ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg py-2 text-xs font-medium inline-flex items-center justify-center gap-1.5 transition-colors"
            >
              <component :is="opt.icon" class="w-3.5 h-3.5" :class="level === opt.key ? '' : opt.toneClass" />
              {{ opt.label }}
            </button>
          </div>
        </div>

        <!-- Title -->
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1.5">
            标题 <span class="text-red-400 normal-case">*</span>
          </label>
          <input
            v-model="title"
            placeholder="例如：本周六 (05/22) 系统维护"
            maxlength="200"
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600"
          />
          <p class="text-[10px] text-zinc-500 mt-1 tabular-nums">{{ title.length }} / 200</p>
        </div>

        <!-- Content -->
        <div>
          <label class="flex items-center justify-between text-[10px] text-zinc-500 tracking-wide uppercase mb-1.5">
            <span>内容 <span class="text-red-400 normal-case">*</span></span>
            <button
              type="button"
              @click="showHelp = !showHelp"
              class="normal-case text-emerald-500 hover:text-emerald-400 inline-flex items-center gap-1 tracking-normal"
            >
              {{ showHelp ? '收起说明' : '支持哪些格式？' }}
            </button>
          </label>
          <Transition name="expand">
            <div
              v-if="showHelp"
              class="rounded-md bg-zinc-100 dark:bg-zinc-950/50 ring-1 ring-black/[0.06] dark:ring-white/[0.04] p-2.5 mb-2 text-[11px] text-zinc-600 dark:text-zinc-400 leading-relaxed space-y-1"
            >
              <p>支持极少量 markdown，方便排版：</p>
              <ul class="list-disc pl-5 space-y-0.5">
                <li><code class="bg-white/70 dark:bg-zinc-900/70 px-1 rounded font-mono-token">**粗体**</code> → <strong>粗体</strong></li>
                <li><code class="bg-white/70 dark:bg-zinc-900/70 px-1 rounded font-mono-token">*斜体*</code> → <em>斜体</em></li>
                <li><code class="bg-white/70 dark:bg-zinc-900/70 px-1 rounded font-mono-token">[链接文字](https://...)</code> → 蓝色链接</li>
                <li>普通换行 = 换行；<strong>空一行 = 分段</strong></li>
                <li>不支持 HTML、图片、列表 —— 模板保持统一</li>
              </ul>
            </div>
          </Transition>
          <textarea
            v-model="content"
            placeholder="可以多段。空一行分段。&#10;&#10;支持 **粗体** 和 [链接](https://example.com)。"
            maxlength="10000"
            rows="8"
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 leading-relaxed resize-y"
          />
          <p class="text-[10px] text-zinc-500 mt-1 tabular-nums">{{ content.length }} / 10000</p>
        </div>

        <!-- Expiry -->
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1.5">
            到期日期（可选）
          </label>
          <div class="flex items-center gap-2">
            <input
              type="date"
              v-model="expiresDate"
              class="bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-1.5 text-sm font-mono-token focus-ring text-zinc-900 dark:text-zinc-200"
            />
            <button
              v-if="expiresDate"
              @click="expiresDate = ''"
              type="button"
              class="text-xs text-zinc-500 hover:text-red-400 transition-colors"
            >
              清除
            </button>
            <span v-else class="text-[11px] text-zinc-500">留空 = 永不到期（需手动删）</span>
          </div>
        </div>

        <!-- Submit -->
        <div class="flex justify-end gap-2 pt-1">
          <button
            v-if="editId != null"
            @click="resetForm"
            type="button"
            class="px-4 py-2 text-sm text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors"
          >
            取消
          </button>
          <button
            @click="submit"
            :disabled="!canSubmit"
            class="inline-flex items-center gap-1.5 bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 text-zinc-950 text-sm font-medium px-5 py-2 rounded-lg transition-colors"
          >
            <Save class="w-3.5 h-3.5" />
            {{ saving ? '保存中…' : editId == null ? '发布公告' : '保存修改' }}
          </button>
        </div>
      </div>

      <!-- Live preview -->
      <div class="space-y-2">
        <div class="flex items-center gap-2 text-[11px] text-zinc-500 px-1">
          <Eye class="w-3.5 h-3.5" />
          <span>用户看到的样子（实时预览）</span>
        </div>
        <AnnouncementCard :a="previewAnnouncement" />
      </div>
    </section>

    <!-- Existing announcements list -->
    <section class="rounded-2xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] overflow-hidden">
      <header class="px-5 py-3 border-b border-black/[0.05] dark:border-white/[0.04]">
        <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">
          所有公告 ({{ list.length }})
        </h2>
      </header>
      <div v-if="loading && list.length === 0" class="py-12 flex justify-center">
        <div class="h-5 w-5 rounded-full border-2 border-zinc-800 border-t-emerald-400 wangui-spin" />
      </div>
      <div v-else-if="list.length === 0" class="py-12 text-center text-sm text-zinc-500">
        还没有公告。在上面发布一条吧。
      </div>
      <ul v-else class="divide-y divide-black/[0.05] dark:divide-white/[0.04]">
        <li
          v-for="a in list"
          :key="a.id"
          class="px-5 py-3 flex items-start gap-3 hover:bg-black/[0.02] dark:hover:bg-white/[0.02] transition-colors"
        >
          <span
            class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium shrink-0 mt-1"
            :class="a.level === 'info'
              ? 'bg-blue-500/15 text-blue-700 dark:text-blue-300 ring-1 ring-blue-500/30'
              : a.level === 'success'
                ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 ring-1 ring-emerald-500/30'
                : a.level === 'warning'
                  ? 'bg-amber-500/15 text-amber-700 dark:text-amber-300 ring-1 ring-amber-500/30'
                  : 'bg-red-500/15 text-red-700 dark:text-red-300 ring-1 ring-red-500/30'"
          >
            {{ a.level === 'info' ? '通知' : a.level === 'success' ? '喜报' : a.level === 'warning' ? '注意' : '紧急' }}
          </span>
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-zinc-900 dark:text-zinc-200 truncate">{{ a.title }}</p>
            <p class="text-[10px] text-zinc-500 font-mono-token tabular-nums">
              发布 {{ fmtDate(a.createdAt) }}
              <span v-if="a.expiresAt"> · 截止 {{ fmtDate(a.expiresAt) }}</span>
              <span v-else> · 永不到期</span>
            </p>
          </div>
          <div class="flex items-center gap-0.5">
            <button
              @click="startEdit(a)"
              title="编辑"
              class="p-1.5 rounded hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 hover:text-emerald-400 transition-colors"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="remove(a)"
              title="删除"
              class="p-1.5 rounded hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 hover:text-red-400 transition-colors"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.expand-enter-active, .expand-leave-active { transition: all 0.2s ease; max-height: 200px; opacity: 1; }
.expand-enter-from, .expand-leave-to { max-height: 0; opacity: 0; overflow: hidden; }
</style>
