# Expose local Vite :5173 so 5G phones can open /join.
# Run from repo root: powershell -ExecutionPolicy Bypass -File scripts/expose-5173.ps1
$ErrorActionPreference = "Stop"
$Repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Tools = Join-Path $Repo "tools"
$Runtime = Join-Path $Repo ".runtime"
$UrlFile = Join-Path $Runtime "public-base.url"
$Cloudflared = Join-Path $Tools "cloudflared.exe"
New-Item -ItemType Directory -Force -Path $Tools, $Runtime | Out-Null

function Write-PublicBase([string]$Url) {
    $base = $Url.Trim().TrimEnd("/")
    Set-Content -Path $UrlFile -Value $base -Encoding ascii
    Write-Host "public join: $base/join"
    Write-Host "wrote $UrlFile"
}

function Test-Port5173 {
    try {
        $c = New-Object System.Net.Sockets.TcpClient
        $iar = $c.BeginConnect("127.0.0.1", 5173, $null, $null)
        $ok = $iar.AsyncWaitHandle.WaitOne(800)
        if ($ok -and $c.Connected) { $c.Close(); return $true }
        $c.Close()
    } catch {}
    return $false
}

if (-not (Test-Port5173)) {
    Write-Host "port 5173 is not listening. start npm run dev in web/console first."
    exit 1
}

function Find-Cmd([string]$Name) {
    $c = Get-Command $Name -ErrorAction SilentlyContinue
    if ($c) { return $c.Source }
    $guess = @(
        (Join-Path $env:USERPROFILE "cpolar\cpolar.exe"),
        "C:\cpolar\cpolar.exe",
        (Join-Path $Tools "cpolar.exe"),
        (Join-Path $env:LOCALAPPDATA "ngrok\ngrok.exe")
    )
    foreach ($g in $guess) {
        if (Test-Path $g) { return $g }
    }
    return $null
}

function Save-Cloudflared {
    if (Test-Path $Cloudflared) { return $Cloudflared }
    $urls = @(
        "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe",
        "https://ghproxy.net/https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"
    )
    foreach ($u in $urls) {
        Write-Host "download cloudflared: $u"
        try {
            Invoke-WebRequest -Uri $u -OutFile $Cloudflared -UseBasicParsing -TimeoutSec 120
            if ((Get-Item $Cloudflared).Length -gt 1000000) { return $Cloudflared }
        } catch {
            Write-Host "download failed, try next mirror"
        }
    }
    return $null
}

$logOut = Join-Path $Runtime "tunnel.out.log"
$logErr = Join-Path $Runtime "tunnel.err.log"
Remove-Item $logOut, $logErr -ErrorAction SilentlyContinue

function Read-UrlFromLog([int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    $rx = [regex]"https://[a-zA-Z0-9._-]+\.(?:trycloudflare\.com|cpolar\.(?:cn|io|top|vip)|ngrok-free\.app|ngrok\.(?:io|app|dev)|loca\.lt|pinggy\.link|pinggy\.io)"
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 400
        $text = ""
        foreach ($f in @($logOut, $logErr)) {
            if (Test-Path $f) {
                $text += (Get-Content -Raw -ErrorAction SilentlyContinue $f)
            }
        }
        if (-not $text) { continue }
        $m = $rx.Match($text)
        if ($m.Success) { return $m.Value }
    }
    return $null
}

$cpolar = Find-Cmd "cpolar"
$ngrok = Find-Cmd "ngrok"

if ($cpolar) {
    Write-Host "using cpolar: $cpolar"
    if ($env:CPOLAR_AUTHTOKEN) {
        & $cpolar authtoken $env:CPOLAR_AUTHTOKEN | Out-Null
    }
    $p = Start-Process -FilePath $cpolar -ArgumentList @("http","5173") -RedirectStandardOutput $logOut -RedirectStandardError $logErr -WorkingDirectory $Repo -PassThru -WindowStyle Hidden
    $url = Read-UrlFromLog 25
    if ($url) { Write-PublicBase $url; Write-Host "cpolar pid=$($p.Id)"; exit 0 }
    Write-Host "cpolar did not print a public URL (need authtoken). fallback to cloudflared."
    Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
}

if ($ngrok) {
    Write-Host "using ngrok: $ngrok"
    $ngrokArgs = @("http","5173","--log=stdout")
    $p = Start-Process -FilePath $ngrok -ArgumentList $ngrokArgs -RedirectStandardOutput $logOut -RedirectStandardError $logErr -WorkingDirectory $Repo -PassThru -WindowStyle Hidden
    $url = Read-UrlFromLog 25
    if ($url) { Write-PublicBase $url; Write-Host "ngrok pid=$($p.Id)"; exit 0 }
    Write-Host "ngrok did not print a public URL (need NGROK_AUTHTOKEN). fallback to cloudflared."
    Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
}

$cf = Save-Cloudflared
if (-not $cf) {
    Write-Host "no cpolar/ngrok, and cloudflared download failed."
    Write-Host "install cpolar, run: cpolar http 5173, then write the https URL to $UrlFile"
    exit 1
}

Write-Host "using cloudflared quick tunnel (no account)..."
$p = Start-Process -FilePath $cf -ArgumentList @("tunnel","--url","http://127.0.0.1:5173","--no-autoupdate") -RedirectStandardOutput $logOut -RedirectStandardError $logErr -WorkingDirectory $Repo -PassThru -WindowStyle Hidden
$url = Read-UrlFromLog 45
if (-not $url) {
    Write-Host "cloudflared did not print a trycloudflare URL. logs:"
    Get-Content $logOut, $logErr -ErrorAction SilentlyContinue | Select-Object -Last 50
    exit 1
}
Write-PublicBase $url
Write-Host "cloudflared pid=$($p.Id)  (kill this process to stop the tunnel)"
exit 0
