<script setup lang="ts">
import { Sun, Moon } from 'lucide-vue-next'
import { useTheme } from '../stores/theme'

const { state, toggle } = useTheme()
</script>

<template>
  <button
    @click="toggle"
    :title="state.theme === 'dark' ? '切换浅色' : '切换深色'"
    class="inline-flex items-center justify-center w-10 h-10 rounded-xl text-zinc-500 hover:text-[#161b22] hover:bg-white ring-1 ring-transparent hover:ring-black/[0.08] dark:hover:text-zinc-100 dark:hover:bg-white/[0.06] dark:hover:ring-white/[0.08] transition-colors"
  >
    <Sun v-if="state.theme === 'dark'" class="w-4 h-4" />
    <Moon v-else class="w-4 h-4" />
  </button>
</template>
