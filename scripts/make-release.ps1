# 打包发布目录：生成 release/ 与可直接上传到 GitHub Releases 的 zip
#
#   .\scripts\make-release.ps1              # 先构建再打包
#   .\scripts\make-release.ps1 -SkipBuild   # 直接用现有 exe 打包
#
# 产物：
#   release/inspirationer.exe                        主程序（GUI，无控制台窗口）
#   release/inspirationer-console.exe                调试版（带控制台）
#   release/start-inspirationer.bat                  双击启动
#   release/README.md  release/LICENSE               文档与许可
#   release/HOW-TO-RUN.txt                           三语快速上手
#   release/inspirationer-v<版本>-windows-amd64.zip  发布包（上传用）
#   release/SHA256SUMS.txt                           校验和
param(
  [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
Set-Location $root

# ---- 版本号：从 main.go 读取
$mainSrc = [IO.File]::ReadAllText((Join-Path $root "main.go"), [Text.Encoding]::UTF8)
if ($mainSrc -notmatch 'version\s*=\s*"([^"]+)"') { throw "无法从 main.go 解析出版本号" }
$version = $Matches[1]
Write-Host "==> 打包 Inspirationer v$version" -ForegroundColor Cyan

# ---- 构建
if (-not $SkipBuild) {
  & (Join-Path $root "build.ps1") -Console
  if ($LASTEXITCODE -ne 0) { throw "构建失败" }
}

foreach ($f in @("inspirationer.exe", "inspirationer-console.exe")) {
  if (-not (Test-Path $f)) { throw "缺少 $f，请先执行 .\build.ps1 -Console" }
}

# ---- 组装 release 目录
$release = Join-Path $root "release"
Remove-Item $release -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $release | Out-Null

Copy-Item "inspirationer.exe", "inspirationer-console.exe" $release -Force
Copy-Item "README.md", "LICENSE" $release -Force

$launcher = @'
@echo off
rem Inspirationer launcher - starts the app, which opens your browser and
rem shows a tray icon. Non-ASCII text is kept out of this file on purpose:
rem cmd.exe can misparse UTF-8 batch files (see HOW-TO-RUN.txt for details).
cd /d "%~dp0"
start "" "%~dp0inspirationer.exe" -open=true
exit /b 0
'@
[IO.File]::WriteAllText((Join-Path $release "start-inspirationer.bat"), $launcher, (New-Object Text.UTF8Encoding($false)))

$howto = @"
Inspirationer v$version — Windows 10/11 (x64)
灵感管理器 / インスピレーションマネージャー

QUICK START
  1. Double-click  inspirationer.exe   (or start-inspirationer.bat)
  2. Your browser opens http://127.0.0.1:8420/ automatically
  3. A light-bulb icon appears in the system tray: left-click to open,
     right-click for the menu (backup now / open data folder / view log / quit)

  数据默认保存在程序同级的 data\ 目录；日志在 data\logs\inspirationer.log
  データは exe と同じフォルダの data\ に保存されます

FILES
  inspirationer.exe          主程序（无控制台窗口，托盘常驻）
  inspirationer-console.exe  调试版（带控制台窗口，可看实时日志）
  start-inspirationer.bat    双击启动（等价于直接双击 exe）
  README.md                  完整文档（English；另有 README.zh-CN.md / README.ja.md）
  LICENSE                    Apache License 2.0

NOTES
  * 端口默认 8420，被占用会自动顺延，实际地址见日志
  * 全部数据都在本地，程序不联网（除非你启用 AI 或 WebDAV 备份）
  * 语言可在 设置 → 外观 → 语言 里切换（English / 简体中文 / 日本語）
  * 没有登录鉴权，请不要直接暴露到公网
"@
[IO.File]::WriteAllText((Join-Path $release "HOW-TO-RUN.txt"), $howto, (New-Object Text.UTF8Encoding($false)))

# ---- 压缩包（发布用）
$zipName = "inspirationer-v$version-windows-amd64.zip"
$zipPath = Join-Path $release $zipName
$zipItems = @(
  (Join-Path $release "inspirationer.exe"),
  (Join-Path $release "inspirationer-console.exe"),
  (Join-Path $release "start-inspirationer.bat"),
  (Join-Path $release "README.md"),
  (Join-Path $release "LICENSE"),
  (Join-Path $release "HOW-TO-RUN.txt")
)
Compress-Archive -Path $zipItems -DestinationPath $zipPath -CompressionLevel Optimal -Force

# ---- 校验和
$sums = foreach ($f in @($zipName, "inspirationer.exe", "inspirationer-console.exe")) {
  $p = Join-Path $release $f
  "{0}  {1}" -f (Get-FileHash $p -Algorithm SHA256).Hash.ToLower(), $f
}
[IO.File]::WriteAllText((Join-Path $release "SHA256SUMS.txt"), (($sums -join "`n") + "`n"), (New-Object Text.UTF8Encoding($false)))

Write-Host "`n✅ 发布包已就绪：$release" -ForegroundColor Green
Get-ChildItem $release -File | Sort-Object Name | ForEach-Object {
  "   {0,-46} {1,8:N0} KB" -f $_.Name, ($_.Length / 1KB)
}
Write-Host "`n上传到 GitHub Release 时选这个文件：release\$zipName"
Write-Host "校验和见 release\SHA256SUMS.txt"
