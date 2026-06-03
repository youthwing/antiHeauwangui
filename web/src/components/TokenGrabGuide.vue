<script setup lang="ts">
import { Clipboard, Download } from 'lucide-vue-next'

withDefaults(defineProps<{
  handoffText?: string
}>(), {
  handoffText: '回到这里粘贴；如果工具自动打开本页，会直接填好。',
})

const TOKEN_GRAB_DOWNLOAD_URL = '/downloads/wangui-token-grab.exe'
</script>

<template>
  <div class="rounded-xl bg-white/70 dark:bg-[#0d1117]/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04] p-3">
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="inline-flex items-center gap-1.5 text-sm font-semibold text-[#161b22] dark:text-zinc-200">
          <Clipboard class="w-4 h-4 text-zinc-500" />
          抓包工具获取 JWT
        </p>
        <p class="text-[11px] text-zinc-500 mt-1 leading-relaxed">
          仅 Windows 电脑使用。抓到后会自动复制 JWT，并可自动打开本页填入；下载不可用时，说明服务端还没放置工具。
        </p>
      </div>
      <a
        :href="TOKEN_GRAB_DOWNLOAD_URL"
        download
        class="shrink-0 inline-flex items-center gap-1.5 bg-zinc-100 hover:bg-zinc-200 dark:bg-zinc-800 dark:hover:bg-zinc-700 ring-1 ring-black/[0.06] dark:ring-white/[0.06] text-zinc-700 dark:text-zinc-300 text-xs px-3 py-2 rounded-lg transition-colors"
      >
        <Download class="w-3.5 h-3.5" />
        下载
      </a>
    </div>
    <ol class="mt-3 space-y-1.5 text-[12px] text-zinc-600 dark:text-zinc-400 leading-relaxed list-decimal list-inside">
      <li>在 Windows 电脑打开微信 PC，进入学校晚归页面。</li>
      <li>运行 <span class="font-mono-token">wangui-token-grab.exe</span>，点“开始抓取”。</li>
      <li>回到微信 PC，按 <span class="font-mono-token">Ctrl+R</span> 刷新晚归页面。</li>
      <li>工具显示“已抓到”后，JWT 会自动复制到剪贴板。</li>
      <li>{{ handoffText }}</li>
    </ol>
    <p class="mt-2 text-[11px] text-amber-600 dark:text-amber-300 leading-relaxed">
      首次运行会提示安装本机 CA / 设置系统代理，这是抓微信 HTTPS 请求所必需；抓完会自动恢复代理。
    </p>
  </div>
</template>
