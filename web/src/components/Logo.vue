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
      class="relative shrink-0 rounded-xl bg-[#e50914] ring-1 ring-[#e50914]/30 shadow-sm shadow-[#e50914]/20 flex items-center justify-center"
    >
      <svg
        :width="size * 0.6"
        :height="size * 0.6"
        viewBox="0 0 24 24"
        fill="currentColor"
        class="text-white"
        aria-hidden="true"
      >
        <text
          x="12"
          y="15"
          text-anchor="middle"
          font-family="JetBrains Mono, ui-monospace, monospace"
          font-size="8.5"
          font-weight="900"
        >aW</text>
        <rect x="6" y="17" width="12" height="1.6" rx="0.8" />
      </svg>
    </div>
    <div v-if="text" class="min-w-0">
      <p class="font-bold tracking-tight text-[#161b22] dark:text-zinc-50 text-sm leading-tight truncate">
        {{ text }}
      </p>
      <p v-if="subtitle" class="text-[10px] text-zinc-500 tracking-wider mt-0.5 truncate">
        {{ subtitle }}
      </p>
    </div>
  </div>
</template>
