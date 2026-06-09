#!/bin/bash
# ============================================
# urldb-novel-upload 一键部署脚本
# 用法: bash deploy.sh
# ============================================

set -e

echo "========================================"
echo "  urldb-novel-upload 服务部署"
echo "========================================"

# 1. 检测 Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装，请先安装 Docker"
    echo "   curl -fsSL https://get.docker.com | sh"
    exit 1
fi

# 2. 检测 Docker Compose
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE="docker compose"
elif docker-compose --version &> /dev/null; then
    DOCKER_COMPOSE="docker-compose"
else
    echo "❌ Docker Compose 未安装"
    exit 1
fi
echo "✅ Docker: $(docker --version)"
echo "✅ $($DOCKER_COMPOSE version)"

# 3. 创建必要的目录
mkdir -p uploads
mkdir -p pending_upload
echo "✅ 目录已创建"

# 4. 复制环境变量（如果不存在）
if [ ! -f .env ]; then
    cp env.example .env
    echo "✅ .env 文件已创建，请修改配置"
else
    echo "✅ .env 文件已存在"
fi

# 5. 构建并启动
echo ""
echo "🔄 构建并启动服务..."
$DOCKER_COMPOSE up --build -d

# 6. 等待服务启动
echo ""
echo "⏳ 等待服务就绪..."
sleep 15

# 7. 检查服务状态
echo ""
echo "📊 服务状态："
$DOCKER_COMPOSE ps

echo ""
echo "========================================"
echo "  ✅ 部署完成！"
echo "========================================"
echo ""
echo "  🌐 访问地址: http://服务器IP:3030"
echo ""
echo "  首次使用："
echo "    1. 打开 http://服务器IP:3030 注册管理员账号"
echo "    2. 在管理后台添加夸克网盘 Cookie"
echo "    3. 在管理后台添加网盘账号"
echo "    4. 配置 NovelUploadWatcher 环境变量"
echo ""
echo "  常用命令："
echo "    查看日志:  $DOCKER_COMPOSE logs -f"
echo "    停止服务:  $DOCKER_COMPOSE down"
echo "    重启服务:  $DOCKER_COMPOSE restart"
echo "    重新构建:  $DOCKER_COMPOSE up --build -d"
echo ""
echo "  环境变量配置（修改 .env 后需重启）："
echo "    NOVEL_CK_ID=1           # 夸克账号ID"
echo "    NOVEL_PARENT_FID=xxx    # 小说目录FID"
echo "    NOVEL_WATCH_DIR=xxx     # 小说下载监控目录"
echo ""
