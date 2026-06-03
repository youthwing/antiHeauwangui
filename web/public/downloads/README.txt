这个目录用于放“抓包获取 JWT”的 Windows 小工具。

线上登录页的下载按钮固定指向：

  /downloads/wangui-token-grab.exe

因此最终文件名必须是：

  web/public/downloads/wangui-token-grab.exe

构建方式（建议在 Windows 电脑上执行）：

  cd tokengrab
  wails build -clean -trimpath

构建完成后复制：

  tokengrab/build/bin/wangui.exe -> web/public/downloads/wangui-token-grab.exe

注意：

1. 这个 exe 需要在 docker compose build wangui 之前放进去，因为主站静态文件会被打进 Go 二进制。
2. 不建议把本地临时构建产物、node_modules 或 dist 提交到仓库。
3. 如果线上点下载是 404，就是镜像构建时还没有这个 exe，放好后重新构建并重启 wangui。
