param([string]$Version = '0.2.0-dev')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Push-Location $projectRoot
$savedEnvironment = @{}
foreach ($name in @('GOPATH','GOCACHE','GOTOOLCHAIN','CGO_ENABLED','GOOS','GOARCH')) {
    $savedEnvironment[$name] = [Environment]::GetEnvironmentVariable($name)
}
try {
    $goBinary = Join-Path $projectRoot '.tools/go/bin/go.exe'
    if (!(Test-Path -LiteralPath $goBinary)) { $goBinary = (Get-Command go -ErrorAction Stop).Source }
    $env:GOPATH = Join-Path $projectRoot '.tools/gopath'
    $env:GOCACHE = Join-Path $projectRoot '.tools/gocache'
    $env:GOTOOLCHAIN = 'local'
    $env:CGO_ENABLED = '0'
    $releaseDir = Join-Path $projectRoot 'artifacts/release'
    New-Item -ItemType Directory -Force -Path $releaseDir | Out-Null
    $manifest = @()
    foreach ($target in @(@('windows','amd64','.exe'), @('linux','amd64',''), @('linux','arm64',''))) {
        $env:GOOS = $target[0]
        $env:GOARCH = $target[1]
        $filename = "mvm-$Version-$($target[0])-$($target[1])$($target[2])"
        $outputPath = Join-Path $releaseDir $filename
        & $goBinary build -tags nomsgpack -trimpath -buildvcs=false "-ldflags=-s -w -X main.version=$Version" -o $outputPath ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw "Build failed for $filename" }
        $manifest += [pscustomobject]@{file=$filename; bytes=(Get-Item -LiteralPath $outputPath).Length; sha256=(Get-FileHash -LiteralPath $outputPath -Algorithm SHA256).Hash.ToLowerInvariant()}
    }
    $manifest | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $releaseDir 'manifest.json') -Encoding utf8
    $manifest | Format-Table file,bytes,sha256
} finally {
    foreach ($name in $savedEnvironment.Keys) { [Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name]) }
    Pop-Location
}

