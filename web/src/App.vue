<script setup lang="ts">
import { useToast } from './lib/toast'

const { state: toastState } = useToast()
</script>

<template>
  <div class="min-h-screen bg-white dark:bg-zinc-950 text-zinc-900 dark:text-zinc-100 antialiased">
    <!-- "勿外传" watermark drifting diagonally (fills behind centered layout, visible in side margins) -->
    <div class="pointer-events-none fixed inset-0 overflow-hidden">
      <div class="absolute -inset-[25%] watermark-bg" />
    </div>

    <!-- Ambient background blobs (on top of cross) -->
    <div class="pointer-events-none fixed inset-0 overflow-hidden">
      <div class="absolute top-[-8%] left-[18%] w-[460px] h-[460px] rounded-full bg-emerald-500/[0.04] blur-3xl" />
      <div class="absolute top-[55%] right-[-8%] w-[400px] h-[400px] rounded-full bg-blue-500/[0.03] blur-3xl" />
    </div>

    <RouterView />

    <Transition name="toast">
      <div
        v-if="toastState.current"
        class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2.5 rounded-xl text-sm font-medium shadow-2xl ring-1 max-w-md text-center"
        :class="
          toastState.current.kind === 'ok'
            ? 'bg-emerald-500/15 text-emerald-300 ring-emerald-500/30 backdrop-blur'
            : 'bg-red-500/15 text-red-300 ring-red-500/30 backdrop-blur'
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
