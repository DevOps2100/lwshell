#!/bin/bash
# 编译并打包为 macOS 应用 (.app)，双击即可使用
# 用法: ./build-app.sh          # 当前架构
#       ./build-app.sh universal # 通用二进制 (arm64+amd64)

set -e
cd "$(dirname "$0")"

APP_NAME="SSH-Manager"
APP_DIR="$APP_NAME.app"
CONTENTS="$APP_DIR/Contents"
MACOS="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"
BINARY_NAME="ssh-manager"
LDFLAGS="-s -w"

echo "[1/4] 编译 Go 程序..."
if [[ "${1:-}" == "universal" ]]; then
  echo "  → 构建 universal (darwin/arm64 + darwin/amd64)"
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o "${BINARY_NAME}_arm64" ./cmd/ssh-manager/
  CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$LDFLAGS" -o "${BINARY_NAME}_amd64" ./cmd/ssh-manager/
  lipo -create -output "$BINARY_NAME" "${BINARY_NAME}_arm64" "${BINARY_NAME}_amd64"
  rm -f "${BINARY_NAME}_arm64" "${BINARY_NAME}_amd64"
else
  go build -ldflags="$LDFLAGS" -o "$BINARY_NAME" ./cmd/ssh-manager/
fi

echo "[2/4] 创建 .app 包结构..."
rm -rf "$APP_DIR"
mkdir -p "$MACOS"
mkdir -p "$RESOURCES"

mv "$BINARY_NAME" "$MACOS/"

echo "[3/4] 写入启动脚本..."

# 启动：先启 Web 服务，再打开浏览器（会跳转到 设置密码 / 登录 / 主界面）
RUN_CMD="$RESOURCES/run.command"
cat > "$RUN_CMD" << 'ENDRUN'
#!/bin/bash
BINDIR="$(cd "$(dirname "$0")/../MacOS" && pwd)"
EXEC="$BINDIR/ssh-manager"

"$EXEC" --http=:21008 &
PID=$!
sleep 1.5
open "http://127.0.0.1:21008"
wait $PID
ENDRUN
chmod +x "$RUN_CMD"

# 双击 .app 时由系统执行 launcher，再通过 Terminal 打开 run.command
LAUNCHER="$MACOS/launcher"
cat > "$LAUNCHER" << 'ENDLAUNCHER'
#!/bin/bash
CONTENTS="$(cd "$(dirname "$0")/.." && pwd)"
open "$CONTENTS/Resources/run.command"
ENDLAUNCHER
chmod +x "$LAUNCHER"

echo "[4/4] 写入 Info.plist..."
plist="$CONTENTS/Info.plist"
cat > "$plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>launcher</string>
  <key>CFBundleIdentifier</key>
  <string>com.ssh-manager.app</string>
  <key>CFBundleName</key>
  <string>SSH-Manager</string>
  <key>CFBundleDisplayName</key>
  <string>SSH-Manager</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>1.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST

echo ""
echo "已完成: $APP_DIR"
echo ""
echo "使用："
echo "  · 双击 $APP_NAME.app，或拖到「应用程序」后从启动台/Spotlight 打开"
echo "  · 会弹出终端并打开浏览器：首次为「设置主密码」页，之后为「登录」页，登录后进入主机管理"
echo "  · 关闭终端窗口即停止 Web 服务"
echo ""
open -R "$APP_DIR"
