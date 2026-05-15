<script setup lang="ts">
withDefaults(
  defineProps<{
    size?: number
    text?: string
    subtitle?: string
  }>(),
  { size: 32, text: '', subtitle: '' },
)
</script>

<template>
  <div class="flex items-center gap-3">
    <div
      :style="{ width: size + 'px', height: size + 'px' }"
      class="relative shrink-0 rounded-xl bg-emerald-500/15 ring-1 ring-emerald-500/30 flex items-center justify-center"
    >
      <!-- House under crescent moon: "晚归" theme -->
      <svg
        :width="size * 0.6"
        :height="size * 0.6"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
        class="text-emerald-400"
      >
        <path
          d="M20 3 A3 3 0 1 0 20 7.4 A2.2 2.2 0 0 1 20 3 Z"
          fill="currentColor"
          stroke="none"
        />
        <path d="M3 14 L12 7 L21 14" />
        <path d="M5 13 V21 H19 V13" />
        <path d="M10 21 V16 H14 V21" />
      </svg>
    </div>
    <div v-if="text" class="min-w-0">
      <p class="font-bold tracking-tight text-zinc-900 dark:text-zinc-50 text-sm leading-tight truncate">
        {{ text }}
      </p>
      <p v-if="subtitle" class="text-[10px] text-zinc-500 tracking-wider mt-0.5 truncate">
        {{ subtitle }}
      </p>
    </div>
  </div>
</template>
