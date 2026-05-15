# syntax=docker/dockerfile:1.7

# ---------------- Stage 1: build SPA ----------------
FROM node:20-alpine AS web
WORKDIR /repo
# Use a China-friendly npm registry to avoid timeouts on registry.npmjs.org.
RUN npm config set registry https://registry.npmmirror.com
COPY web/package.json web/package-lock.json* ./web/
RUN cd web && npm ci
COPY web/ ./web/
# vite.config.ts writes to ../cmd/wangui/web-dist — make the parent exist.
RUN mkdir -p cmd/wangui && cd web && npm run build

# ---------------- Stage 2: build Go binary ----------------
FROM golang:1.25-alpine AS gobuild
WORKDIR /src
# proxy.golang.org is unreachable from many China networks; use goproxy.cn.
# `,direct` falls back to direct VCS fetch if the proxy doesn't have a module.
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Replace any stale web-dist with the freshly built one from stage 1.
RUN rm -rf cmd/wangui/web-dist
COPY --from=web /repo/cmd/wangui/web-dist ./cmd/wangui/web-dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /out/wangui ./cmd/wangui

# ---------------- Stage 3: runtime ----------------
FROM alpine:3.20
# Run as root inside the container. The container is already an isolation
# boundary; running root in here is unrelated to the host's root and is the
# simplest way to coexist with bind-mounted /data (whose ownership is set by
# the host, not the image).
RUN apk add --no-cache ca-certificates tzdata \
 && mkdir -p /data
ENV TZ=Asia/Shanghai
COPY --from=gobuild /out/wangui /usr/local/bin/wangui
WORKDIR /data
EXPOSE 5555
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/wangui"]
CMD ["serve", "-addr", "0.0.0.0:5555", "-data", "/data"]
