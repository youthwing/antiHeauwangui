<script setup lang="ts">
import type { FunctionalComponent } from 'vue'

export interface NavItem {
  to: string
  label: string
  icon: FunctionalComponent
}

defineProps<{ items: NavItem[] }>()
</script>

<template>
  <nav class="space-y-1">
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
        :class="[
          'group flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-all',
          isExactActive
            ? 'bg-red-500/15 text-red-300 ring-1 ring-red-500/25'
            : 'text-zinc-500 dark:text-zinc-400 hover:text-[#161b22] dark:hover:text-zinc-100 hover:bg-black/5 dark:hover:bg-white/5',
        ]"
      >
        <component
          :is="item.icon"
          class="w-4 h-4 transition-colors"
          :class="isExactActive ? 'text-red-400' : 'text-zinc-500 group-hover:text-zinc-400 dark:text-zinc-300'"
        />
        <span class="font-medium">{{ item.label }}</span>
      </a>
    </RouterLink>
  </nav>
</template>
