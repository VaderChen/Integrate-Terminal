#requires -Version 5.1

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ProjectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$OutputPath = Join-Path $ProjectRoot "build\bin\IntegTERM.exe"
$LicenseOutputDirectory = Join-Path $ProjectRoot "build\bin\licenses"
$MetadataOutputPath = Join-Path $ProjectRoot "build\bin\build-metadata.json"
$BuildSourceUrl = if ([string]::IsNullOrWhiteSpace($env:BUILD_SOURCE_URL)) { "https://github.com/VaderChen/Integrate-Terminal" } else { $env:BUILD_SOURCE_URL }

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
    Write-Host "產生第三方授權清冊..."
    & node (Join-Path $ProjectRoot "scripts\generate-third-party-notices.mjs")
    if ($LASTEXITCODE -ne 0) {
        throw "第三方授權清冊產生失敗。"
    }

    $BuildCommit = $env:BUILD_COMMIT
    $BuildTag = $env:BUILD_TAG
    $BuildState = $env:BUILD_STATE
    if ([string]::IsNullOrWhiteSpace($BuildCommit) -or [string]::IsNullOrWhiteSpace($BuildTag) -or [string]::IsNullOrWhiteSpace($BuildState)) {
        $GitCommand = Get-Command git -ErrorAction SilentlyContinue
        if ($GitCommand) {
            & git -C $ProjectRoot rev-parse --is-inside-work-tree *> $null
            $IsGitWorkTree = $LASTEXITCODE -eq 0
        }
        else {
            $IsGitWorkTree = $false
        }

        if ($IsGitWorkTree) {
            if ([string]::IsNullOrWhiteSpace($BuildCommit)) {
                $BuildCommit = (& git -C $ProjectRoot rev-parse HEAD).Trim()
            }
            if ([string]::IsNullOrWhiteSpace($BuildTag)) {
                $ExactTag = (& git -C $ProjectRoot describe --tags --exact-match HEAD 2>$null)
                $BuildTag = if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($ExactTag)) { $ExactTag.Trim() } else { "untagged" }
            }
            if ([string]::IsNullOrWhiteSpace($BuildState)) {
                $GitStatus = & git -C $ProjectRoot status --porcelain=v1 --untracked-files=normal
                $BuildState = if ($GitStatus) { "dirty" } else { "clean" }
            }
        }
        else {
            if ([string]::IsNullOrWhiteSpace($BuildCommit)) { $BuildCommit = "unknown" }
            if ([string]::IsNullOrWhiteSpace($BuildTag)) { $BuildTag = "untagged" }
            if ([string]::IsNullOrWhiteSpace($BuildState)) { $BuildState = "unknown" }
        }
    }

    foreach ($MetadataValue in @($BuildCommit, $BuildTag, $BuildState, $BuildSourceUrl)) {
        if ($MetadataValue -notmatch '^[A-Za-z0-9._/:+-]+$') {
            throw "建置中繼資料包含不支援的字元：$MetadataValue"
        }
    }

    $BuildLdflags = "-X github.com/VaderChen/Integrate-Terminal/internal/version.Product=$AppVersion -X github.com/VaderChen/Integrate-Terminal/internal/version.Commit=$BuildCommit -X github.com/VaderChen/Integrate-Terminal/internal/version.Tag=$BuildTag -X github.com/VaderChen/Integrate-Terminal/internal/version.BuildState=$BuildState -X github.com/VaderChen/Integrate-Terminal/internal/version.SourceURL=$BuildSourceUrl"

    Write-Host "建置 Windows x64 執行檔..."
    & go run "github.com/wailsapp/wails/v2/cmd/wails@$WailsVersion" build -clean -nopackage -platform windows/amd64 -ldflags $BuildLdflags
    if ($LASTEXITCODE -ne 0) {
        throw "Windows x64 建置失敗。"
    }

    if (-not (Test-Path -LiteralPath $OutputPath -PathType Leaf)) {
        throw "建置失敗：找不到 $OutputPath"
    }

    if (Test-Path -LiteralPath $LicenseOutputDirectory) {
        Remove-Item -LiteralPath $LicenseOutputDirectory -Recurse -Force
    }
    New-Item -ItemType Directory -Path $LicenseOutputDirectory -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $ProjectRoot "LICENSE") -Destination (Join-Path $LicenseOutputDirectory "GPL-3.0.txt")
    Copy-Item -LiteralPath (Join-Path $ProjectRoot "THIRD-PARTY-NOTICES.md") -Destination $LicenseOutputDirectory
    Copy-Item -LiteralPath (Join-Path $ProjectRoot "THIRD-PARTY-LICENSES.txt") -Destination $LicenseOutputDirectory
    & node (Join-Path $ProjectRoot "scripts\write-build-metadata.mjs") $MetadataOutputPath $AppVersion $BuildCommit $BuildTag $BuildState $BuildSourceUrl
    if ($LASTEXITCODE -ne 0) {
        throw "建置中繼資料寫入失敗。"
    }

    Write-Host "完成：$OutputPath"
    Write-Host "授權文件：$LicenseOutputDirectory"
    Write-Host "來源版本：$BuildTag ($BuildCommit, $BuildState)"
}
finally {
    $env:VITE_APP_VERSION = $PreviousAppVersion
    Pop-Location
}
