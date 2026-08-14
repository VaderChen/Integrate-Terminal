#requires -Version 5.1

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ProjectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$OutputPath = Join-Path $ProjectRoot "build\bin\IntegTERM.exe"

if ($env:OS -ne "Windows_NT") {
    throw "此腳本只能在 Windows 上執行。"
}

foreach ($CommandName in @("go", "node", "npm")) {
    if (-not (Get-Command $CommandName -ErrorAction SilentlyContinue)) {
        throw "缺少必要指令：$CommandName"
    }
}

$PreviousAppVersion = $env:VITE_APP_VERSION
Push-Location $ProjectRoot
try {
    $WailsVersion = (& go list -m -f "{{.Version}}" github.com/wailsapp/wails/v2).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($WailsVersion) -or $WailsVersion -eq "<no value>") {
        throw "無法取得 Wails 版本。"
    }

    $AppVersion = (& node -p "require('./wails.json').info.productVersion").Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "無法取得應用程式版本。"
    }

    $env:VITE_APP_VERSION = $AppVersion
    Write-Host "建置 Windows x64 執行檔..."
    & go run "github.com/wailsapp/wails/v2/cmd/wails@$WailsVersion" build -clean -nopackage -platform windows/amd64
    if ($LASTEXITCODE -ne 0) {
        throw "Windows x64 建置失敗。"
    }

    if (-not (Test-Path -LiteralPath $OutputPath -PathType Leaf)) {
        throw "建置失敗：找不到 $OutputPath"
    }

    Write-Host "完成：$OutputPath"
}
finally {
    $env:VITE_APP_VERSION = $PreviousAppVersion
    Pop-Location
}
