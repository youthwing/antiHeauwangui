<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { LogOut, ChevronDown } from 'lucide-vue-next'
import Logo from './Logo.vue'

defineProps<{ name: string }>()
defineEmits<{ (e: 'logout'): void }>()

const menuOpen = ref(false)
const menuRef = ref<HTMLElement | null>(null)

function onDocClick(e: MouseEvent) {
  if (menuRef.value && !menuRef.value.contains(e.target as Node)) {
    menuOpen.value = false
  }
}

onMounted(() => document.addEventListener('mousedown', onDocClick))
onUnmounted(() => document.removeEventListener('mousedown', onDocClick))
</script>

<template>
  <header class="sticky top-0 z-40 bg-white/70 dark:bg-zinc-950/70 backdrop-blur-xl border-b border-black/[0.08] dark:border-white/[0.06]">
    <div class="max-w-3xl mx-auto px-4 h-14 flex justify-between items-center">
      <Logo :size="28" />

      <div ref="menuRef" class="relative">
        <button
          @click="menuOpen = !menuOpen"
          class="flex items-center gap-2 pl-1 pr-2 py-1 rounded-lg hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
        >
          <span class="w-7 h-7 rounded-md bg-emerald-500/15 ring-1 ring-emerald-500/30 text-emerald-400 text-xs font-semibold flex items-center justify-center">
            {{ name?.[0] || '?' }}
          </span>
          <span class="text-sm text-zinc-900 dark:text-zinc-200">{{ name }}</span>
          <ChevronDown
            class="w-3.5 h-3.5 text-zinc-500 transition-transform"
            :class="menuOpen ? 'rotate-180' : ''"
          />
        </button>

        <Transition name="menu">
          <div
            v-if="menuOpen"
            class="absolute right-0 top-full mt-2 w-44 rounded-xl bg-zinc-100 dark:bg-zinc-900 ring-1 ring-black/10 dark:ring-white/10 shadow-2xl overflow-hidden"
          >
            <button
              @click="$emit('logout'); menuOpen = false"
              class="w-full flex items-center gap-2 px-3 py-2.5 text-sm text-zinc-700 dark:text-zinc-300 hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
            >
              <LogOut class="w-4 h-4 text-zinc-500" />
              <span>退出登录</span>
            </button>
          </div>
        </Transition>
      </div>
    </div>
  </header>
</template>

<style scoped>
.menu-enter-active,
.menu-leave-active {
  transition: all 0.15s ease;
}
.menu-enter-from,
.menu-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
