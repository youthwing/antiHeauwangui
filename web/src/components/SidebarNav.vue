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
  <nav class="space-y-1.5">
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
          'group relative flex min-h-11 items-center gap-2.5 px-3 py-2 rounded-xl text-sm transition-all',
          isExactActive
            ? 'bg-white/[0.12] text-white ring-1 ring-white/[0.12]'
            : 'text-zinc-400 hover:text-white hover:bg-white/[0.07]',
        ]"
      >
        <span
          v-if="isExactActive"
          class="absolute left-0 top-2 bottom-2 w-0.5 rounded-r-full bg-[#e50914]"
        />
        <component
          :is="item.icon"
          class="w-4 h-4 transition-colors"
          :class="isExactActive ? 'text-[#ff4b55]' : 'text-zinc-500 group-hover:text-zinc-200'"
        />
        <span class="font-medium">{{ item.label }}</span>
      </a>
    </RouterLink>
  </nav>
</template>
