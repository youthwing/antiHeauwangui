<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  LayoutDashboard,
  Settings as SettingsIcon,
  ScrollText,
  User as UserIcon,
  LogOut,
} from 'lucide-vue-next'
import Logo from './Logo.vue'
import Avatar from './Avatar.vue'
import ThemeToggle from './ThemeToggle.vue'
import SidebarNav, { type NavItem } from './SidebarNav.vue'
import { useAuth } from '../stores/auth'
import { api } from '../api'
import { showToast } from '../lib/toast'

const router = useRouter()
const auth = useAuth()

const items: NavItem[] = [
  { to: '/', label: '仪表盘', icon: LayoutDashboard },
  { to: '/settings', label: '配置', icon: SettingsIcon },
  { to: '/records', label: '签到记录', icon: ScrollText },
  { to: '/account', label: '账号', icon: UserIcon },
]

async function logout() {
  try {
    await api.logout()
  } finally {
    auth.clear()
    showToast('ok', '已退出')
    router.push('/login')
  }
}

onMounted(() => {
  if (!auth.state.initialized) auth.init()
})
</script>

<template>
  <div class="relative flex flex-col md:flex-row mx-auto max-w-[1700px] bg-white dark:bg-zinc-950 min-h-screen">
    <!-- Sidebar (desktop, sticky below the global banner) -->
    <aside
      class="hidden md:flex flex-col w-64 shrink-0 sticky top-0 h-screen border-r border-black/[0.06] dark:border-white/[0.05] bg-white/60 dark:bg-zinc-950/60 backdrop-blur-xl overflow-y-auto"
    >
      <div class="px-5 py-5 border-b border-black/[0.05] dark:border-white/[0.04]">
        <Logo :size="34" text="勿外传" />
      </div>

      <div class="flex-1 px-3 py-5">
        <p class="px-3 mb-2 text-[10px] uppercase tracking-wider text-zinc-500 dark:text-zinc-600 font-medium">导航</p>
        <SidebarNav :items="items" />
      </div>

      <div class="px-3 py-3 border-t border-black/[0.05] dark:border-white/[0.04] space-y-1">
        <div
          v-if="auth.state.me"
          class="flex items-center gap-3 px-3 py-2 rounded-lg"
        >
          <Avatar
            :src="auth.state.me.userAvatarUrl"
            :name="auth.state.me.userName"
            :size="36"
            rounded="lg"
          />
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-zinc-900 dark:text-zinc-200 truncate">{{ auth.state.me.userName }}</p>
            <p class="text-[11px] text-zinc-500 font-mono-token truncate">{{ auth.state.me.userNumber }}</p>
          </div>
          <ThemeToggle />
        </div>
        <button
          @click="logout"
          class="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
        >
          <LogOut class="w-4 h-4 text-zinc-500" />
          <span>退出</span>
        </button>
      </div>
    </aside>

    <!-- Mobile top bar (sits below the global banner) -->
    <header class="md:hidden sticky top-0 z-30 bg-white/85 dark:bg-zinc-950/85 backdrop-blur-xl border-b border-black/[0.08] dark:border-white/[0.06] h-12 flex items-center justify-between px-4">
      <Logo :size="26" text="勿外传" />
      <div class="flex items-center gap-1">
        <ThemeToggle />
        <button
          @click="logout"
          class="text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-100 p-2 rounded-lg hover:bg-black/5 dark:hover:bg-white/5"
        >
          <LogOut class="w-4 h-4" />
        </button>
      </div>
    </header>

    <!-- Main — fills the space right of the sidebar, with inner padding -->
    <main class="relative flex-1 min-w-0">
      <div class="px-3 sm:px-6 md:px-10 lg:px-14 py-4 sm:py-6 md:py-8 pb-24 md:pb-16">
        <RouterView v-slot="{ Component }">
          <Transition name="fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </div>

      <!-- Mobile bottom nav -->
      <nav class="md:hidden fixed bottom-0 inset-x-0 z-30 bg-white/90 dark:bg-zinc-950/90 backdrop-blur-xl border-t border-black/[0.08] dark:border-white/[0.06] flex justify-around py-2">
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
            class="flex flex-col items-center gap-0.5 px-3 py-1.5 rounded-lg"
            :class="isExactActive ? 'text-emerald-400' : 'text-zinc-500'"
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
