#!/usr/bin/env bash

# Antigravity-Go Core Update Utility
# Perfected by Antigravity Agent for the Boss.

set -e

# --- Configuration ---
APP_PATH="/Applications/Antigravity.app"
BIN_DIR="./bin"
TARGET_CORE="./antigravity_core"

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# --- ASCII Logo ---
echo -e "${CYAN}"
echo "█████╗ ███╗   ██╗████████╗██╗ ██████╗ ██████╗  █████╗ ██╗   ██╗██╗████████╗██╗   ██╗"
echo "██╔══██╗████╗  ██║╚══██╔══╝██║██╔════╝ ██╔══██╗██╔══██╗██║   ██║██║╚══██╔══╝╚██╗ ██╔╝"
echo "███████║██╔██╗ ██║   ██║   ██║██║  ███╗██████╔╝███████║██║   ██║██║   ██║    ╚████╔╝ "
echo "██╔══██║██║╚██╗██║   ██║   ██║██║   ██║██╔══██╗██╔══██║╚██╗ ██╔╝██║   ██║     ╚██╔╝  "
echo "██║  ██║██║ ╚████║   ██║   ██║╚██████╔╝██║  ██║██║  ██║ ╚████╔╝ ██║   ██║      ██║   "
echo "╚═╝  ╚═╝╚═╝  ╚═══╝   ╚═╝   ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝  ╚═══╝  ╚═╝   ╚═╝      ╚═╝   "
echo -e "${YELLOW}                 >>> CORE UPDATE UTILITY v2.0 <<<${NC}"
echo ""

# --- Functions ---
log_info() { echo -e "${CYAN}[INFO]${NC} $1"; }
log_done() { echo -e "${GREEN}[DONE]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

find_first_file() {
    for candidate in "$@"; do
        if [ -n "$candidate" ] && [ -f "$candidate" ]; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

find_first_cmd() {
    for candidate in "$@"; do
        if command -v "$candidate" >/dev/null 2>&1; then
            command -v "$candidate"
            return 0
        fi
    done
    return 1
}

read_tool_version() {
    local tool_path="$1"
    if [ -x "$tool_path" ]; then
        "$tool_path" --version 2>/dev/null | head -n 1 | awk '{print $2}'
    else
        echo "Unavailable"
    fi
}

get_arch() {
    if [ -e "$1" ]; then
        file -b "$1" 2>/dev/null | awk -F' ' '{print $NF}'
    else
        echo "Unavailable"
    fi
}

# --- Execution ---

log_info "检查系统环境..."
if [ ! -d "$APP_PATH" ]; then
    log_error "未在 /Applications 中找到 Antigravity.app。请优先安装官方软件。"
fi

mkdir -p "$BIN_DIR"

# 1. 自动检测架构与核心二进制
log_info "正在探索核心引擎 (antigravity_core)..."
SOURCE_CORE=$(find_first_file \
    "$APP_PATH/Contents/Resources/app/extensions/antigravity/bin/language_server_macos_x64" \
    "$APP_PATH/Contents/Resources/app/extensions/antigravity/bin/language_server_macos_arm64" || true)

if [ -n "$SOURCE_CORE" ]; then
    log_info "发现核心二进制: $SOURCE_CORE"
    
    # 备份旧版本
    if [ -f "$TARGET_CORE" ]; then
        log_warn "备份旧的核心文件至 ${TARGET_CORE}.bak"
        cp "$TARGET_CORE" "${TARGET_CORE}.bak"
    fi
    
    cp "$SOURCE_CORE" "$TARGET_CORE"
    chmod +x "$TARGET_CORE"
    
    # Ad-hoc sign to prevent macOS Dyld hang
    log_info "签署核心二进制 (Ad-hoc Signing)..."
    codesign --force --deep -s - "$TARGET_CORE"
    
    log_done "核心引擎更新完毕。"
else
    log_error "无法定位核心引擎。请检查软件结构是否发生变化。"
fi

# 2. 提取辅助工具 (Ripgrep & FD)
log_info "正在压榨辅助工具 (Take-Itism Mode)..."

# Ripgrep
SOURCE_RG="$APP_PATH/Contents/Resources/app/node_modules/@vscode/ripgrep/bin/rg"
if [ -f "$SOURCE_RG" ]; then
    cp "$SOURCE_RG" "$BIN_DIR/rg"
    chmod +x "$BIN_DIR/rg"
    log_done "Ripgrep (rg) 已同步。"
else
    log_warn "跳过 Ripgrep: 未找到源文件。"
fi

# FD
SOURCE_FD=$(find_first_file \
    "$APP_PATH/Contents/Resources/app/extensions/antigravity/bin/fd" \
    "$APP_PATH/Contents/Resources/app/node_modules/.bin/fd" || true)

if [ -n "$SOURCE_FD" ]; then
    cp "$SOURCE_FD" "$BIN_DIR/fd"
    chmod +x "$BIN_DIR/fd"
    log_done "FD 已同步。"
else
    SYSTEM_FD=$(find_first_cmd fd fdfind || true)
    if [ -n "$SYSTEM_FD" ]; then
        cp "$SYSTEM_FD" "$BIN_DIR/fd"
        chmod +x "$BIN_DIR/fd"
        log_done "FD 已从系统命令同步: $SYSTEM_FD"
    else
        if [ -e "$BIN_DIR/fd" ]; then
            rm -f "$BIN_DIR/fd"
            log_warn "新版 App 未提供 FD，已移除失效的本地副本。"
        fi
        log_warn "跳过 FD: 官方包和系统环境都未提供可用版本。"
    fi
fi

# 3. 记录版本信息
VERSION_FILE="./CORE_VERSION.json"
CORE_VER=$(/usr/libexec/PlistBuddy -c "Print CFBundleShortVersionString" "$APP_PATH/Contents/Info.plist" 2>/dev/null || echo "Unknown")
RG_VER=$(read_tool_version "$BIN_DIR/rg")
FD_VER=$(read_tool_version "$BIN_DIR/fd")
UPDATED_DATE=$(date +%Y-%m-%d)

CORE_ARCH=$(get_arch "$TARGET_CORE")
RG_ARCH=$(get_arch "$BIN_DIR/rg")
FD_ARCH=$(get_arch "$BIN_DIR/fd")
OS_TYPE=$(uname -s)
HOST_ARCH=$(uname -m)

cat > "$VERSION_FILE" <<EOF
{
  "core": {
    "version": "$CORE_VER",
    "binary": "$TARGET_CORE",
    "arch": "$CORE_ARCH",
    "last_updated": "$UPDATED_DATE"
  },
  "tools": {
    "ripgrep": {
      "version": "$RG_VER",
      "arch": "$RG_ARCH"
    },
    "fd": {
      "version": "$FD_VER",
      "arch": "$FD_ARCH"
    }
  },
  "system": {
    "os": "$OS_TYPE",
    "host_arch": "$HOST_ARCH"
  },
  "source_app": {
    "path": "$APP_PATH",
    "version": "$CORE_VER"
  }
}
EOF

# 4. 结果汇总
echo ""
log_done "✨ 所有内核组件更新成功！"
echo "------------------------------------------------"
ls -lh "$TARGET_CORE" "$BIN_DIR/rg" "$BIN_DIR/fd" 2>/dev/null || true
echo "------------------------------------------------"
log_info "版本信息已更新至: $VERSION_FILE"
log_info "现在您可以继续使用 'make build' 或 'make run' 了。"
