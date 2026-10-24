$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw "未找到 npm"
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "未找到 go"
}

Push-Location (Join-Path $root "web")
if (Test-Path "package-lock.json") {
    npm ci
} else {
    npm install
}
if ($LASTEXITCODE -ne 0) { throw "前端依赖安装失败" }
npm run build
if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
Pop-Location

$version = if ($env:APP_VERSION) { $env:APP_VERSION } else { "dev" }
$commit = "none"
try { $commit = (git rev-parse --short HEAD).Trim() } catch { }
$built = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-X main.version=$version -X main.commit=$commit -X main.buildTime=$built"

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Name = "chatbi.exe" },
    @{ GOOS = "linux"; GOARCH = "amd64"; Name = "chatbi" },
    @{ GOOS = "linux"; GOARCH = "arm64"; Name = "chatbi" }
)

$release = Join-Path $root "release"
if (Test-Path $release) { Remove-Item $release -Recurse -Force }
New-Item -ItemType Directory -Path $release | Out-Null

foreach ($target in $targets) {
    $dir = Join-Path $release ("chatbi-" + $target.GOOS + "-" + $target.GOARCH)
    New-Item -ItemType Directory -Path $dir | Out-Null
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $env:CGO_ENABLED = "0"
    & go build -trimpath -ldflags $ldflags -o (Join-Path $dir $target.Name) ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "编译失败 $($target.GOOS)/$($target.GOARCH)" }
    Copy-Item (Join-Path $root "configs") (Join-Path $dir "configs") -Recurse
    Copy-Item (Join-Path $root "migrations") (Join-Path $dir "migrations") -Recurse
    Copy-Item (Join-Path $root "scripts/install-linux.sh") (Join-Path $dir "install-linux.sh")
    @"
chatbi $version
commit: $commit
built: $built
os/arch: $($target.GOOS)/$($target.GOARCH)
"@ | Set-Content -Encoding utf8 (Join-Path $dir "VERSION.txt")
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Write-Host "release -> $release"
