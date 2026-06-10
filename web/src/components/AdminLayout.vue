<script setup lang="ts">
import { onMounted } from 'vue'
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
} from 'lucide-vue-next'
import Logo from './Logo.vue'
import ThemeToggle from './ThemeToggle.vue'
import SidebarNav, { type NavItem } from './SidebarNav.vue'
import { useAdminAuth } from '../stores/auth'
import { adminApi } from '../api'
import { showToast } from '../lib/toast'

const router = useRouter()
const admin = useAdminAuth()

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

onMounted(() => {
  if (!admin.state.initialized) admin.init()
})
</script>

<template>
  <div class="relative flex flex-col md:flex-row gap-4 mx-auto max-w-[1760px] min-h-screen md:p-4">
    <aside class="hidden md:flex flex-col w-64 shrink-0 sticky top-4 h-[calc(100vh-2rem)] side-rail rounded-2xl overflow-y-auto">
      <div class="px-5 py-5 border-b border-white/[0.08] flex items-center justify-between gap-2">
        <Logo :size="34" text="antiWG 管理端" subtitle="仅供运维" />
        <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium bg-amber-500/15 text-amber-400 ring-1 ring-amber-500/30 shrink-0">
          <ShieldCheck class="w-2.5 h-2.5" />
          ADMIN
        </span>
      </div>

      <div class="flex-1 px-3 py-5">
        <p class="px-3 mb-2 text-[10px] text-zinc-500 font-medium">管理</p>
        <SidebarNav :items="items" />
      </div>

      <div class="px-3 py-3 border-t border-white/[0.08] flex items-center gap-1">
        <button
          @click="logout"
          class="flex-1 flex min-h-11 items-center gap-2.5 px-3 py-2 rounded-xl text-sm text-zinc-400 hover:text-white hover:bg-white/[0.07] transition-colors"
        >
          <LogOut class="w-4 h-4 text-zinc-500" />
          <span>退出管理员</span>
        </button>
        <ThemeToggle />
      </div>
    </aside>

    <header class="md:hidden sticky top-0 z-30 bg-white/92 dark:bg-[#0d1117]/92 backdrop-blur-xl border-b border-black/[0.08] dark:border-white/[0.06] h-14 flex items-center justify-between px-4">
      <Logo :size="26" text="antiWG 管理端" />
      <div class="flex items-center gap-2">
        <ThemeToggle />
        <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium bg-amber-500/15 text-amber-400 ring-1 ring-amber-500/30">
          <ShieldCheck class="w-2.5 h-2.5" />
          ADMIN
        </span>
      </div>
    </header>

    <main class="relative flex-1 min-w-0">
      <div class="px-3 sm:px-6 md:px-4 lg:px-8 py-4 sm:py-6 md:py-2 pb-24 md:pb-10">
        <RouterView v-slot="{ Component }">
          <Transition name="fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </div>

      <nav class="md:hidden fixed bottom-0 inset-x-0 z-30 bg-white/92 dark:bg-[#0d1117]/92 backdrop-blur-xl border-t border-black/[0.08] dark:border-white/[0.06] flex justify-around py-2">
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
            class="flex min-w-12 flex-col items-center gap-0.5 px-2 py-1.5 rounded-xl transition-colors"
            :class="isExactActive ? 'text-[#b0000b] dark:text-[#ff6b72] bg-[#fff1f2] dark:bg-[#2a1114]' : 'text-zinc-500'"
          >
            <component :is="item.icon" class="w-5 h-5" />
            <span class="text-[10px]">{{ item.label }}</span>
          </a>
        </RouterLink>
      </nav>
    </main>
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.15s ease, transform 0.15s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; transform: translateY(4px); }
</style>
