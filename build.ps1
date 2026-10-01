# 构建灵感管理器
#
#   .\build.ps1              # 构建 GUI 版（无控制台窗口，托盘 + 自动打开浏览器）
#   .\build.ps1 -Console     # 额外构建一个带控制台的调试版 inspirationer-console.exe
#   .\build.ps1 -Icon        # 重新生成图标与 Windows 资源（需要 python + Pillow + rsrc）
#
param(
  [switch]$Console,
  [switch]$Icon
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if ($Icon) {
  Write-Host "==> 生成图标与资源（需要 python + Pillow，以及 rsrc）" -ForegroundColor Cyan
  $py = (Get-Command python -ErrorAction SilentlyContinue).Source
  if (-not $py) { $py = (Get-Command py -ErrorAction SilentlyContinue).Source }
  if (-not $py) {
    throw "未找到 python。仓库里已包含生成好的 assets\app.ico 与 rsrc_windows_amd64.syso，可跳过 -Icon。"
  }
  & $py scripts\make-icon.py

  $rsrc = (Get-Command rsrc -ErrorAction SilentlyContinue).Source
  if (-not $rsrc) {
    $candidate = Join-Path $env:USERPROFILE "go\bin\rsrc.exe"
    if (Test-Path $candidate) { $rsrc = $candidate }
  }
  if (-not $rsrc) {
    throw "未找到 rsrc，可先安装：go install github.com/akavel/rsrc@latest"
  }
  & $rsrc -manifest assets\app.manifest -ico assets\app.ico -o rsrc_windows_amd64.syso -arch amd64
}

Write-Host "==> gofmt" -ForegroundColor Cyan
gofmt -w .
Write-Host "==> go vet" -ForegroundColor Cyan
go vet ./...
Write-Host "==> go test" -ForegroundColor Cyan
go test ./...

Write-Host "==> go build (GUI / 无控制台窗口)" -ForegroundColor Cyan
go build -trimpath -ldflags "-s -w -H=windowsgui" -o inspirationer.exe .

if ($Console) {
  Write-Host "==> go build (控制台调试版)" -ForegroundColor Cyan
  go build -trimpath -ldflags "-s -w" -o inspirationer-console.exe .
}

Write-Host "`n构建完成：" -ForegroundColor Green
Get-ChildItem *.exe | ForEach-Object { "  {0,-32} {1,8:N0} KB" -f $_.Name, ($_.Length / 1KB) }
Write-Host "`n运行：.\inspirationer.exe          （自动打开浏览器，托盘图标常驻，无黑窗口）"
Write-Host "调试：.\inspirationer-console.exe  （带控制台窗口，可看实时日志）"
Write-Host "      .\inspirationer.exe -console （无黑窗口版临时开一个控制台）"
