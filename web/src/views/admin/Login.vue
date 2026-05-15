<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowRight, AlertCircle, ShieldCheck } from 'lucide-vue-next'
import Logo from '../../components/Logo.vue'
import { adminApi } from '../../api'
import { useAdminAuth } from '../../stores/auth'
import { showToast } from '../../lib/toast'

const router = useRouter()
const route = useRoute()
const admin = useAdminAuth()

const password = ref('')
const submitting = ref(false)
const error = ref<string | null>(null)

async function submit() {
  error.value = null
  if (!password.value.trim()) {
    error.value = '请输入密码'
    return
  }
  submitting.value = true
  try {
    await adminApi.login(password.value.trim())
    admin.setAdmin(true)
    showToast('ok', '欢迎，管理员')
    const redirect = (route.query.redirect as string) || '/rosekhlifa'
    router.push(redirect)
  } catch (e: any) {
    error.value = e.message || '登录失败'
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  await admin.init()
  if (admin.state.isAdmin) router.replace('/rosekhlifa')
})
</script>

<template>
  <div class="min-h-screen flex items-center justify-center px-4 py-10 relative">
    <div class="w-full max-w-sm">
      <div class="flex flex-col items-center mb-8">
        <Logo :size="56" />
        <div class="flex items-center gap-2 mt-5">
          <h1 class="text-2xl font-bold tracking-tight">晚归管理端</h1>
          <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium bg-amber-500/15 text-amber-400 ring-1 ring-amber-500/30">
            <ShieldCheck class="w-2.5 h-2.5" />
            ADMIN
          </span>
        </div>
        <p class="text-sm text-zinc-500 mt-1.5">仅供运维人员</p>
      </div>

      <div class="bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-2xl p-6 backdrop-blur-sm">
        <label class="block text-xs text-zinc-500 dark:text-zinc-400 mb-1.5">密码</label>
        <input
          v-model="password"
          type="password"
          placeholder="WANGUI_ADMIN_PASS"
          @keyup.enter="submit"
          class="w-full bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg px-3 py-2.5 focus-ring text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-300 dark:placeholder:text-zinc-700 font-mono-token"
        />

        <button
          @click="submit"
          :disabled="submitting || !password.trim()"
          class="mt-4 w-full inline-flex items-center justify-center gap-2 bg-amber-500 hover:bg-amber-400 disabled:bg-zinc-200 dark:disabled:bg-zinc-800 disabled:text-zinc-500 text-zinc-950 font-medium py-2.5 rounded-xl transition-all disabled:cursor-not-allowed"
        >
          <span>{{ submitting ? '校验中…' : '登录' }}</span>
          <ArrowRight v-if="!submitting" class="w-4 h-4" />
        </button>

        <div
          v-if="error"
          class="mt-3 flex items-start gap-2 px-3 py-2 rounded-lg bg-red-500/10 ring-1 ring-red-500/20 text-sm text-red-400"
        >
          <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
          <span>{{ error }}</span>
        </div>
      </div>

      <p class="text-center text-[11px] text-zinc-500 dark:text-zinc-600 mt-5">
        通过环境变量 <code class="text-zinc-500 font-mono-token">WANGUI_ADMIN_PASS</code> 配置
      </p>
    </div>
  </div>
</template>
