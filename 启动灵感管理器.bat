@echo off
chcp 65001 >nul
cd /d "%~dp0"
title 灵感管理器 Inspirationer

if not exist inspirationer.exe (
  echo [1/2] 未找到 inspirationer.exe，尝试用 Go 编译...
  where go >nul 2>nul
  if errorlevel 1 (
    echo     没有检测到 Go 环境。请先安装 Go 1.20+ ^(https://go.dev/dl/^)，
    echo     或直接使用已编译好的 inspirationer.exe。
    pause
    exit /b 1
  )
  go build -trimpath -ldflags "-s -w -H=windowsgui" -o inspirationer.exe .
  if errorlevel 1 (
    echo     编译失败，请检查上面的错误信息。
    pause
    exit /b 1
  )
)

echo [2/2] 正在启动灵感管理器...
echo     服务地址：http://127.0.0.1:8420   （启动完成后会自动打开浏览器）
echo     数据目录：%~dp0data
echo     常驻系统托盘（右下角 💡 图标）：左键打开界面，右键菜单，菜单里可退出。
echo.
rem 用 start 拉起后本窗口立即关闭；服务本体是 GUI 子系统程序，本身也没有控制台窗口
start "" "%~dp0inspirationer.exe" -open=true
exit /b 0
