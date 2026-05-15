<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import {
  Info,
  Lock,
  Clock,
  Cog,
  ShieldCheck,
  Mail,
  Send,
  Save,
  Eye,
  EyeOff,
} from 'lucide-vue-next'
import type { AdminStats, SmtpUpdate } from '../../types'
import { adminApi } from '../../api'
import { showToast } from '../../lib/toast'

const stats = ref<AdminStats | null>(null)

// SMTP form state
const smtp = reactive<SmtpUpdate>({
  enabled: false,
  host: 'smtp.gmail.com',
  port: 587,
  username: '',
  password: '',
  from: '',
  adminBcc: '',
})
const passwordSet = ref(false)
const loadingSmtp = ref(false)
const savingSmtp = ref(false)
const testingSmtp = ref(false)
const showPassword = ref(false)

async function loadSmtp() {
  loadingSmtp.value = true
  try {
    const c = await adminApi.getSmtp()
    smtp.enabled = c.enabled
    smtp.host = c.host
    smtp.port = c.port
    smtp.username = c.username
    smtp.from = c.from
    smtp.adminBcc = c.adminBcc
    smtp.password = '' // never expose; user enters new one to overwrite
    passwordSet.value = c.passwordSet
  } catch (e: any) {
    showToast('err', e.message || 'SMTP 加载失败')
  } finally {
    loadingSmtp.value = false
  }
}

async function saveSmtp() {
  savingSmtp.value = true
  try {
    const updated = await adminApi.updateSmtp({ ...smtp })
    smtp.password = ''
    passwordSet.value = (updated as any).passwordSet ?? passwordSet.value
    showToast('ok', 'SMTP 已保存')
  } catch (e: any) {
    showToast('err', e.message || 'SMTP 保存失败')
  } finally {
    savingSmtp.value = false
  }
}

async function testSend() {
  testingSmtp.value = true
  try {
    const r = await adminApi.testSmtp()
    showToast('ok', `测试邮件已发到 ${r.sentTo}`)
  } catch (e: any) {
    showToast('err', e.message || '测试发送失败')
  } finally {
    testingSmtp.value = false
  }
}

onMounted(async () => {
  try {
    stats.value = await adminApi.stats()
  } catch {}
  await loadSmtp()
})
</script>

<template>
  <div class="space-y-3">
    <header class="mb-1">
      <h1 class="text-2xl font-bold tracking-tight">系统设置</h1>
      <p class="text-sm text-zinc-500 mt-1">运维相关的全局配置与信息。</p>
    </header>

    <!-- SMTP 邮件通知 -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-4 gap-3">
        <div class="flex items-center gap-2">
          <Mail class="w-4 h-4 text-zinc-500" />
          <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">邮件通知 (SMTP)</h2>
        </div>
        <button
          @click="smtp.enabled = !smtp.enabled"
          :class="smtp.enabled ? 'bg-emerald-500' : 'bg-zinc-300 dark:bg-zinc-700'"
          class="relative w-11 h-6 rounded-full transition-colors shrink-0"
        >
          <span
            :class="smtp.enabled ? 'translate-x-5' : 'translate-x-0.5'"
            class="absolute top-0.5 left-0 w-5 h-5 bg-white rounded-full shadow-md transition-transform"
          />
        </button>
      </div>
      <p class="text-xs text-zinc-500 leading-relaxed mb-4">
        Gmail 推荐：host <code class="bg-zinc-200/70 dark:bg-zinc-800/70 px-1 rounded text-zinc-700 dark:text-zinc-300 font-mono-token">smtp.gmail.com</code>
        port <code class="bg-zinc-200/70 dark:bg-zinc-800/70 px-1 rounded text-zinc-700 dark:text-zinc-300 font-mono-token">587</code>
        (STARTTLS)。密码请用 Gmail 「应用专用密码」(16 位)，不是登录密码。Gmail 账号需开启两步验证。
      </p>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">SMTP Host</label>
          <input
            v-model="smtp.host"
            placeholder="smtp.gmail.com"
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200 font-mono-token"
          />
        </div>
        <div>
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">Port</label>
          <input
            v-model.number="smtp.port"
            type="number"
            placeholder="587"
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200 font-mono-token"
          />
        </div>
        <div class="sm:col-span-2">
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">发件人邮箱 (SMTP Username)</label>
          <input
            v-model="smtp.username"
            type="email"
            placeholder="you@gmail.com"
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200 font-mono-token"
          />
        </div>
        <div class="sm:col-span-2">
          <label class="flex items-center justify-between text-[10px] text-zinc-500 tracking-wide uppercase mb-1">
            <span>应用专用密码 (App Password)</span>
            <span v-if="passwordSet && !smtp.password" class="text-emerald-600 dark:text-emerald-400 normal-case tracking-normal">
              ✓ 已设置，留空保持不变
            </span>
          </label>
          <div class="relative">
            <input
              v-model="smtp.password"
              :type="showPassword ? 'text' : 'password'"
              :placeholder="passwordSet ? '保持不变（输入新值才覆盖）' : '16 位 Gmail app password'"
              class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 pr-10 text-sm focus-ring text-zinc-900 dark:text-zinc-200 font-mono-token"
            />
            <button
              @click="showPassword = !showPassword"
              type="button"
              class="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"
            >
              <component :is="showPassword ? EyeOff : Eye" class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
        <div class="sm:col-span-2">
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">
            From 显示名 (可选)
          </label>
          <input
            v-model="smtp.from"
            placeholder='例如 勿外传 &lt;you@gmail.com&gt;'
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200"
          />
        </div>
        <div class="sm:col-span-2">
          <label class="block text-[10px] text-zinc-500 tracking-wide uppercase mb-1">
            管理员收件邮箱 (所有用户的通知都会抄送给这里)
          </label>
          <input
            v-model="smtp.adminBcc"
            type="email"
            placeholder="admin@example.com (留空则只发给用户自己)"
            class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2 text-sm focus-ring text-zinc-900 dark:text-zinc-200 font-mono-token"
          />
          <p class="text-[11px] text-zinc-500 mt-1.5 leading-relaxed">
            如果用户没开通知，但你配了这个邮箱 → 你仍然会收到所有签到结果（用作管理员日志）。
          </p>
        </div>
      </div>

      <div class="mt-5 flex flex-wrap gap-2 justify-end">
        <button
          @click="testSend"
          :disabled="testingSmtp || !passwordSet"
          class="inline-flex items-center gap-1.5 bg-blue-500/15 hover:bg-blue-500/25 disabled:opacity-40 disabled:cursor-not-allowed ring-1 ring-blue-500/30 text-blue-700 dark:text-blue-300 text-sm font-medium px-4 py-2 rounded-lg transition-colors"
          :title="passwordSet ? '' : '请先保存配置（含密码）'"
        >
          <Send class="w-3.5 h-3.5" :class="testingSmtp ? 'wangui-spin' : ''" />
          {{ testingSmtp ? '发送中…' : '发测试邮件' }}
        </button>
        <button
          @click="saveSmtp"
          :disabled="savingSmtp"
          class="inline-flex items-center gap-1.5 bg-emerald-500 hover:bg-emerald-400 disabled:bg-zinc-200 dark:disabled:bg-zinc-800 disabled:text-zinc-500 text-zinc-950 text-sm font-medium px-5 py-2 rounded-lg transition-colors"
        >
          <Save class="w-3.5 h-3.5" />
          {{ savingSmtp ? '保存中…' : '保存 SMTP' }}
        </button>
      </div>
    </section>

    <!-- Server info -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center gap-2 mb-4">
        <Info class="w-4 h-4 text-zinc-500" />
        <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">系统状态</h2>
      </div>
      <dl class="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">
        <div class="flex items-center justify-between">
          <dt class="text-zinc-500">签到窗口</dt>
          <dd class="font-mono-token tabular-nums">22:00 – 22:30</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-zinc-500">默认 ruleId</dt>
          <dd class="font-mono-token tabular-nums">1</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-zinc-500">总用户</dt>
          <dd class="tabular-nums">{{ stats?.users.total ?? '—' }}</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-zinc-500">已用 / 总邀请码</dt>
          <dd class="tabular-nums">{{ stats?.codes.used ?? '—' }} / {{ stats?.codes.total ?? '—' }}</dd>
        </div>
      </dl>
    </section>

    <!-- Default schedule (read-only) -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center gap-2 mb-4">
        <Clock class="w-4 h-4 text-zinc-500" />
        <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">默认调度策略</h2>
      </div>
      <p class="text-xs text-zinc-500 mb-4">新激活的用户会继承这些默认值。要修改请直接编辑后端 schema 默认值。</p>
      <dl class="grid grid-cols-2 gap-x-6 gap-y-3 text-sm font-mono-token tabular-nums">
        <div class="flex items-center justify-between">
          <dt class="text-zinc-500 font-sans">触发分钟</dt>
          <dd>2</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-zinc-500 font-sans">抖动秒</dt>
          <dd>180</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-zinc-500 font-sans">重试次数</dt>
          <dd>3</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-zinc-500 font-sans">重试间隔 (分)</dt>
          <dd>5</dd>
        </div>
      </dl>
    </section>

    <!-- Security -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center gap-2 mb-4">
        <ShieldCheck class="w-4 h-4 text-zinc-500" />
        <h2 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">安全</h2>
      </div>
      <ul class="text-sm text-zinc-700 dark:text-zinc-300 space-y-3">
        <li class="flex items-start gap-2.5">
          <Lock class="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
          <div>
            <p class="font-medium">Token / SMTP 密码 加密</p>
            <p class="text-xs text-zinc-500 mt-0.5">AES-256-GCM，主密钥从 <code class="text-zinc-700 dark:text-zinc-300 font-mono-token">WANGUI_MASTER_KEY</code> 环境变量读取。</p>
          </div>
        </li>
        <li class="flex items-start gap-2.5">
          <Cog class="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
          <div>
            <p class="font-medium">管理员密码</p>
            <p class="text-xs text-zinc-500 mt-0.5">通过 <code class="text-zinc-700 dark:text-zinc-300 font-mono-token">WANGUI_ADMIN_PASS</code> 环境变量配置。重启服务后立即生效。</p>
          </div>
        </li>
      </ul>
    </section>
  </div>
</template>
