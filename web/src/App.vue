<script setup lang="ts">
import { computed } from 'vue'
import { useToast } from './lib/toast'
import { useAuth } from './stores/auth'

const { state: toastState } = useToast()
const auth = useAuth()

const watermarkText = computed(() => {
  const name = auth.state.me?.userName?.trim()
  return name || 'antiWG'
})
const watermarkItems = Array.from({ length: 96 }, (_, i) => i)
</script>

<template>
  <div class="min-h-screen app-frame text-[#161b22] dark:text-[#f0f6fc] antialiased">
    <!-- User-bound watermark: switches to the logged-in user's name. -->
    <div class="pointer-events-none fixed inset-0 overflow-hidden" aria-hidden="true">
      <div class="absolute -inset-[25%] watermark-layer">
        <span
          v-for="i in watermarkItems"
          :key="i"
          class="watermark-word"
        >
          {{ watermarkText }}
        </span>
      </div>
    </div>

    <RouterView />

    <Transition name="toast">
      <div
        v-if="toastState.current"
        class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2.5 rounded-xl text-sm font-medium shadow-2xl ring-1 max-w-md text-center"
        :class="
          toastState.current.kind === 'ok'
            ? 'bg-[#171717] text-white ring-black/10 dark:bg-white dark:text-[#111111] dark:ring-white/20 backdrop-blur'
            : 'bg-[#e50914] text-white ring-[#e50914]/30 backdrop-blur'
        "
      >
        {{ toastState.current.text }}
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translate(-50%, 12px);
}
</style>
