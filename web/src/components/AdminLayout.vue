<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  LayoutDashboard,
  Ticket,
  Building2,
  Users,
  UserPlus,
  ScrollText,
  Cog,
  LogOut,
  ShieldCheck,
  Activity,
  Megaphone,
  KeyRound,
  Copy,
  X,
} from 'lucide-vue-next'
import Logo from './Logo.vue'
import ThemeToggle from './ThemeToggle.vue'
import SidebarNav, { type NavItem } from './SidebarNav.vue'
import { useAdminAuth } from '../stores/auth'
import { adminApi } from '../api'
import { showToast } from '../lib/toast'
import { copyText } from '../lib/clipboard'
import { formatDateTime } from '../lib/format'
import type { SiteGateCode } from '../types'

const router = useRouter()
const admin = useAdminAuth()
const gateCode = ref<SiteGateCode | null>(null)
const gateModalOpen = ref(false)
const gateBusy = ref(false)

const items: NavItem[] = [
  { to: '/airvel', label: '概览', icon: LayoutDashboard },
  { to: '/airvel/monitor', label: '监控看板', icon: Activity },
  { to: '/airvel/announcements', label: '公告', icon: Megaphone },
  { to: '/airvel/codes', label: '邀请码', icon: Ticket },
  { to: '/airvel/dorms', label: '宿舍楼', icon: Building2 },
  { to: '/airvel/users', label: '用户', icon: Users },
  { to: '/airvel/guests', label: '临时朋友', icon: UserPlus },
  { to: '/airvel/logs', label: '日志', icon: ScrollText },
  { to: '/airvel/settings', label: '设置', icon: Cog },
]

async function logout() {
  try {
    await adminApi.logout()
  } finally {
    admin.setAdmin(false)
    showToast('ok', '已退出管理员')
    router.push('/airvel/login')
  }
}

async function createGateCode() {
  if (gateBusy.value) return
  gateBusy.value = true
  try {
    const r = await adminApi.createSiteGateCode()
    gateCode.value = r
    gateModalOpen.value = true
    const ok = await copyText(r.code)
    showToast('ok', ok ? '入口码已生成并复制' : '入口码已生成')
  } catch (e: any) {
    showToast('err', e.message || '入口码生成失败')
  } finally {
    gateBusy.value = false
  }
}

async function copyGateCode() {
  if (!gateCode.value) return
  const ok = await copyText(gateCode.value.code)
  showToast(ok ? 'ok' : 'err', ok ? '入口码已复制' : '复制失败，请手动复制')
}

onMounted(() => {
  if (!admin.state.initialized) admin.init()
})
</script>

<template>
  <div class="relative flex flex-col md:flex-row mx-auto max-w-[1700px] bg-white dark:bg-[#0d1117] min-h-screen">
    <aside class="hidden md:flex flex-col w-64 shrink-0 sticky top-0 h-screen border-r border-amber-500/10 bg-white/60 dark:bg-[#0d1117]/60 backdrop-blur-xl overflow-y-auto">
      <div class="px-5 py-5 border-b border-black/[0.05] dark:border-white/[0.04] flex items-center justify-between gap-2">
        <Logo :size="34" text="antiWG 管理端" subtitle="仅供运维" />
        <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium bg-amber-500/15 text-amber-400 ring-1 ring-amber-500/30 shrink-0">
          <ShieldCheck class="w-2.5 h-2.5" />
          ADMIN
        </span>
      </div>

      <div class="flex-1 px-3 py-5">
        <p class="px-3 mb-2 text-[10px] uppercase tracking-wider text-zinc-500 dark:text-zinc-600 font-medium">管理</p>
        <SidebarNav :items="items" />
      </div>

      <div class="px-3 py-3 border-t border-black/[0.05] dark:border-white/[0.04] flex items-center gap-1">
        <button
          @click="createGateCode"
          :disabled="gateBusy"
          title="生成入口码"
          aria-label="生成入口码"
          class="h-9 w-9 shrink-0 flex items-center justify-center rounded-lg text-zinc-500 dark:text-zinc-400 hover:text-[#161b22] dark:hover:text-zinc-100 hover:bg-black/5 dark:hover:bg-white/5 transition-colors disabled:opacity-50"
        >
          <KeyRound class="w-4 h-4" :class="gateBusy ? 'wangui-spin' : ''" />
        </button>
        <button
          @click="logout"
          class="flex-1 flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-zinc-500 dark:text-zinc-400 hover:text-[#161b22] dark:hover:text-zinc-100 hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
        >
          <LogOut class="w-4 h-4 text-zinc-500" />
          <span>退出管理员</span>
        </button>
        <ThemeToggle />
      </div>
    </aside>

    <header class="md:hidden sticky top-0 z-30 bg-white/85 dark:bg-[#0d1117]/85 backdrop-blur-xl border-b border-black/[0.08] dark:border-white/[0.06] h-12 flex items-center justify-between px-4">
      <Logo :size="26" text="antiWG 管理端" />
      <div class="flex items-center gap-2">
        <button
          @click="createGateCode"
          :disabled="gateBusy"
          title="生成入口码"
          aria-label="生成入口码"
          class="h-8 w-8 flex items-center justify-center rounded-lg text-zinc-500 hover:text-red-400 transition-colors disabled:opacity-50"
        >
          <KeyRound class="w-4 h-4" :class="gateBusy ? 'wangui-spin' : ''" />
        </button>
        <ThemeToggle />
        <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium bg-amber-500/15 text-amber-400 ring-1 ring-amber-500/30">
          <ShieldCheck class="w-2.5 h-2.5" />
          ADMIN
        </span>
      </div>
    </header>

    <main class="relative flex-1 min-w-0">
      <div class="px-3 sm:px-6 md:px-10 lg:px-14 py-4 sm:py-6 md:py-8 pb-24 md:pb-16">
        <RouterView v-slot="{ Component }">
          <Transition name="fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </div>

      <nav class="md:hidden fixed bottom-0 inset-x-0 z-30 bg-white/90 dark:bg-[#0d1117]/90 backdrop-blur-xl border-t border-black/[0.08] dark:border-white/[0.06] flex justify-around py-2">
        <RouterLink
          v-for="item in items"
          :key="item.to"
          :to="item.to"
          v-slot="{ isExactActive }"
          custom
        >
          <a
            :href="item.to"
            @click.prevent="$router.push(item.to)"
            class="flex flex-col items-center gap-0.5 px-2 py-1.5 rounded-lg"
            :class="isExactActive ? 'text-amber-400' : 'text-zinc-500'"
          >
            <component :is="item.icon" class="w-5 h-5" />
            <span class="text-[10px]">{{ item.label }}</span>
          </a>
        </RouterLink>
      </nav>
    </main>

    <Transition name="fade">
      <div
        v-if="gateModalOpen && gateCode"
        class="fixed inset-0 z-50 flex items-center justify-center px-4 py-6 bg-[#0d1117]/55 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        aria-labelledby="gate-code-title"
      >
        <div class="w-full max-w-md rounded-2xl bg-white dark:bg-[#161b22] ring-1 ring-black/[0.08] dark:ring-white/[0.08] shadow-2xl overflow-hidden">
          <div class="flex items-center justify-between px-5 py-4 border-b border-black/[0.06] dark:border-white/[0.06]">
            <div>
              <h2 id="gate-code-title" class="text-base font-bold text-[#161b22] dark:text-zinc-100">一次性入口码</h2>
              <p class="text-xs text-zinc-500 mt-0.5">10 分钟内有效，用过即废</p>
            </div>
            <button
              @click="gateModalOpen = false"
              aria-label="关闭"
              class="h-9 w-9 flex items-center justify-center rounded-lg text-zinc-500 hover:text-red-400 hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
            >
              <X class="w-4 h-4" />
            </button>
          </div>

          <div class="p-5">
            <div class="rounded-xl bg-[#f6f8fa] dark:bg-[#0d1117] ring-1 ring-black/[0.06] dark:ring-white/[0.06] p-4">
              <p class="text-[11px] text-zinc-500 mb-2">访问码</p>
              <p class="font-mono-token text-base sm:text-lg text-[#161b22] dark:text-zinc-100 break-all">
                {{ gateCode.code }}
              </p>
            </div>

            <p class="mt-3 text-xs text-zinc-500 dark:text-zinc-400">
              截止 {{ formatDateTime(gateCode.expiresAt) }}。此码只打开站点入口，不登录任何用户。
            </p>

            <div class="mt-5 grid grid-cols-2 gap-3">
              <button
                @click="copyGateCode"
                class="inline-flex items-center justify-center gap-2 rounded-xl bg-[#161b22] dark:bg-white text-white dark:text-[#0d1117] px-3 py-2.5 text-sm font-medium hover:opacity-90 transition-opacity"
              >
                <Copy class="w-4 h-4" />
                复制
              </button>
              <button
                @click="createGateCode"
                :disabled="gateBusy"
                class="inline-flex items-center justify-center gap-2 rounded-xl bg-[#e50914] text-white px-3 py-2.5 text-sm font-medium hover:bg-red-500 transition-colors disabled:opacity-50"
              >
                <KeyRound class="w-4 h-4" :class="gateBusy ? 'wangui-spin' : ''" />
                再生成
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.15s ease, transform 0.15s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; transform: translateY(4px); }
</style>
