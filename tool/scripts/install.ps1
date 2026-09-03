<#
.SYNOPSIS
  Install the deal-kit CLI on Windows.

.DESCRIPTION
  Downloads a prebuilt binary for this machine, verifies its SHA-256 checksum
  against the published checksums.txt, and installs it under LOCALAPPDATA.
  No Go toolchain is required.

.EXAMPLE
  irm https://raw.githubusercontent.com/oliviosubelza/deal-dev-kit/main/tool/scripts/install.ps1 | iex

.NOTES
  Environment overrides:
    DEAL_KIT_VERSION  release tag to install (default: latest)
    DEAL_KIT_BIN_DIR  install directory
    DEAL_KIT_REPO     kit repository to verify access to
#>

$ErrorActionPreference = 'Stop'

$Repo = 'oliviosubelza/deal-dev-kit'
$Version = if ($env:DEAL_KIT_VERSION) { $env:DEAL_KIT_VERSION } else { 'latest' }
$BinDir = if ($env:DEAL_KIT_BIN_DIR) { $env:DEAL_KIT_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'deal-kit\bin' }
# Must match kit.DefaultRepo in the CLI: checking a URL the CLI never uses
# reports a problem that does not exist, and misses one that does.
$KitRepo = if ($env:DEAL_KIT_REPO) { $env:DEAL_KIT_REPO } else { 'https://github.com/oliviosubelza/deal-dev-kit.git' }

# Windows PowerShell 5.1 still negotiates TLS 1.0 by default, which GitHub rejects.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Get-Target {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { return 'windows_amd64' }
        'ARM64' { return 'windows_arm64' }
        default { throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
    }
}

function Resolve-Version {
    if ($Version -ne 'latest') { return $Version }
    try {
        $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    } catch {
        throw @"
no published release found for $Repo.
  Someone needs to cut one first: push a v* tag and let CI build the binaries.
  To install a specific version once it exists: `$env:DEAL_KIT_VERSION = 'v0.1.0'
"@
    }
    return $release.tag_name
}

$target = Get-Target
$version = Resolve-Version
$asset = "deal-kit_$target.exe"
$base = "https://github.com/$Repo/releases/download/$version"

$tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Write-Host "==> downloading deal-kit $version ($target)"
    $exe = Join-Path $tmp 'deal-kit.exe'
    Invoke-WebRequest "$base/$asset" -OutFile $exe -UseBasicParsing

    Write-Host '==> verifying checksum'
    $sums = Join-Path $tmp 'checksums.txt'
    Invoke-WebRequest "$base/checksums.txt" -OutFile $sums -UseBasicParsing

    $line = Select-String -Path $sums -Pattern "\s$([regex]::Escape($asset))$" | Select-Object -First 1
    if (-not $line) { throw "no checksum published for $target" }
    $expected = ($line.Line -split '\s+')[0]
    $actual = (Get-FileHash $exe -Algorithm SHA256).Hash.ToLower()
    if ($expected.ToLower() -ne $actual) {
        throw "checksum mismatch (expected $expected, got $actual)"
    }

    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    Copy-Item $exe (Join-Path $BinDir 'deal-kit.exe') -Force
    Write-Host "==> installed to $(Join-Path $BinDir 'deal-kit.exe')"
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# Add to the user's PATH, not the machine's: no elevation, and reversible from
# the same Environment Variables dialog the user already knows.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$BinDir*") {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$BinDir", 'User')
    Write-Host "==> added $BinDir to your user PATH (open a new terminal to pick it up)"
}
$env:Path = "$env:Path;$BinDir"

Write-Host '==> checking access to the kit repository'
if (Get-Command git -ErrorAction SilentlyContinue) {
    # Never prompt: this may run piped into iex, with no terminal to answer.
    $env:GIT_TERMINAL_PROMPT = '0'
    git ls-remote --exit-code $KitRepo HEAD *> $null
    if ($LASTEXITCODE -eq 0) {
        Write-Host '    ok'
    } else {
        Write-Warning "cannot reach $KitRepo"
        Write-Warning 'deal-kit is installed but cannot fetch the kit yet.'
        Write-Warning 'If the repository is private, make sure your git credentials have access.'
    }
} else {
    Write-Warning 'git is not installed; deal-kit needs it to fetch the kit.'
}
