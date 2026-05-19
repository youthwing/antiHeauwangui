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
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
        class="text-white"
      >
        <path d="M6 17 V7 L12 11 L18 7 V17" />
        <path d="M8.5 17 V11.5" />
        <path d="M15.5 17 V11.5" />
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
