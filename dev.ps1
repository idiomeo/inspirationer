# 开发模式启动：前端资源直接从 web/ 目录读取，改完 CSS/JS 刷新浏览器即可生效（无需重新编译）
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not (Test-Path ".\inspirationer.exe")) {
  Write-Host "==> 首次运行，先编译" -ForegroundColor Cyan
  go build -o inspirationer.exe .
}

Write-Host "==> 开发模式启动（磁盘前端 + 自动打开浏览器）" -ForegroundColor Cyan
.\inspirationer.exe -dev-web .\web -open=true
