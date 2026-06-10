# 前端构建阶段
FROM node:20-slim AS frontend-builder

# 安装pnpm
WORKDIR /app/web
COPY web/ ./
RUN npm install --frozen-lockfile
ARG NUXT_PUBLIC_API_SERVER=http://backend:8080/api
ARG NUXT_PUBLIC_API_CLIENT=/api
RUN npm run build

# 前端运行阶段
FROM node:20-alpine AS frontend

# RUN npm install -g pnpm
ENV NODE_ENV=production

WORKDIR /app
COPY --from=frontend-builder /app/web/.output ./.output
COPY --from=frontend-builder /app/web/package*.json ./
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]

# 后端构建阶段
FROM golang:1.24.5-alpine AS backend-builder

WORKDIR /app
COPY go.mod go.sum ./

# 去除 UTF-8 BOM 及首行乱码字符（GitHub zip 源可能带 BOM）
RUN for f in go.mod go.sum; do \
        sed -i '1s/^\xEF\xBB\xBF//' "$f" 2>/dev/null; \
        awk 'NR==1{sub(/^\?+/,"")}1' "$f" > "$f.tmp" && mv "$f.tmp" "$f"; \
    done

# 国内 Go 代理（通过构建参数传入）
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

RUN go mod download

# 复制所有源代码
COPY . .

# 去除所有源码文件的 UTF-8 BOM 和首行乱码
RUN find . -type f \( -name "*.go" -o -name "*.yaml" -o -name "*.yml" -o -name "*.json" \) \
    -exec sh -c 'sed -i "1s/^\xEF\xBB\xBF//" "$1" 2>/dev/null; awk "NR==1{sub(/^\?+/,\"\")}1" "$1" > "$1.tmp" && mv "$1.tmp" "$1"' _ {} \;

# 定义构建参数
ARG VERSION
ARG GIT_COMMIT
ARG GIT_BRANCH
ARG BUILD_TIME

# 获取版本信息并编译
RUN VERSION=${VERSION:-$(cat VERSION)} && \
    GIT_COMMIT=${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")} && \
    GIT_BRANCH=${GIT_BRANCH:-$(git branch --show-current 2>/dev/null || echo "unknown")} && \
    BUILD_TIME=${BUILD_TIME:-$(date '+%Y-%m-%d %H:%M:%S')} && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags "-X 'github.com/zhiyungezhu/urldb-novel-upload/utils.Version=${VERSION}' \
              -X 'github.com/zhiyungezhu/urldb-novel-upload/utils.BuildTime=${BUILD_TIME}' \
              -X 'github.com/zhiyungezhu/urldb-novel-upload/utils.GitCommit=${GIT_COMMIT}' \
              -X 'github.com/zhiyungezhu/urldb-novel-upload/utils.GitBranch=${GIT_BRANCH}'" \
    -o main .

# 后端运行阶段
FROM alpine:latest AS backend

# 安装时区数据
RUN apk add --no-cache tzdata

WORKDIR /root/

# 复制后端二进制文件
COPY --from=backend-builder /app/main .

# 创建uploads目录
RUN mkdir -p uploads

# 设置环境变量
ENV GIN_MODE=release
ENV TZ=Asia/Shanghai

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["./main"] 