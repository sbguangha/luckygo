# Load-test POST /api/lottery/join with wrk (via WSL) and random Chinese names.
# Usage (repo root):
#   powershell -ExecutionPolicy Bypass -File scripts/wrk-join.ps1
#   powershell -ExecutionPolicy Bypass -File scripts/wrk-join.ps1 -Watch
#   powershell -ExecutionPolicy Bypass -File scripts/wrk-join.ps1 -Stress
param(
    [switch]$Watch,
    [switch]$Stress,
    [string]$Url = "http://127.0.0.1:8888/api/lottery/join"
)

$ErrorActionPreference = "Stop"
$Repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$LuaWin = Join-Path $Repo "scripts\lottery-join.lua"

function Test-JoinPort {
    try {
        $c = New-Object System.Net.Sockets.TcpClient
        $ok = $c.BeginConnect("127.0.0.1", 8888, $null, $null).AsyncWaitHandle.WaitOne(600)
        if ($ok -and $c.Connected) { $c.Close(); return $true }
        $c.Close()
    } catch {}
    return $false
}

if (-not (Test-JoinPort)) {
    Write-Host "gateway is not listening on 8888. start: go run ./app/gateway -f app/gateway/etc/luckygo-api.yaml"
    exit 1
}

$xing = @("赵","钱","孙","李","周","吴","郑","王","冯","陈","蒋","沈","韩","杨","朱","秦","许","何","吕","张","孔","曹","严","华","金","魏","陶","姜","谢","邹","苏","潘","葛","范","彭","鲁","马","苗","方","俞","任","袁","柳","史","唐","薛","雷","贺","罗","郝","顾","孟","黄","萧","尹","姚","汪","毛","戴","宋","庞","熊","董","梁","杜","贾")
$ming = @("伟","芳","娜","敏","静","丽","强","磊","洋","勇","军","杰","娟","艳","涛","明","超","霞","平","刚","华","辉","鹏","飞","浩","婷","雪","琳","倩","晨","阳","峰","波","斌","健","丹","萍","红","玲","悦","欣","睿","轩","涵","宇","宁","彤","瑶","琪","萱","秀英","桂英","建华","志强","俊杰","佳琪","雨桐","浩然","子轩","诗涵")

function New-RealName {
    $n = $xing[(Get-Random -Maximum $xing.Count)] + $ming[(Get-Random -Maximum $ming.Count)]
    if ((Get-Random -Maximum 100) -lt 35) {
        $n = $n + $ming[(Get-Random -Maximum $ming.Count)]
    }
    if ($n.Length -lt 2 -or $n.Length -gt 16) {
        $n = "李" + $ming[(Get-Random -Maximum $ming.Count)]
    }
    return $n
}

if ($Watch) {
    $n = 80
    $delayMs = 200
    Write-Host "watch mode: join $n people, 1 every ${delayMs}ms. open http://localhost:5173/live"
    $ok = 0
    $fail = 0
    for ($i = 1; $i -le $n; $i++) {
        $name = New-RealName
        $id = "watch-$i-$(Get-Random -Minimum 100000 -Maximum 999999)"
        $body = (@{ user_id = $id; user_name = $name } | ConvertTo-Json -Compress)
        try {
            $r = Invoke-RestMethod -Uri $Url -Method POST -ContentType "application/json; charset=utf-8" -Body ([System.Text.Encoding]::UTF8.GetBytes($body)) -TimeoutSec 5
            if ($r.code -eq 0) {
                $ok++
                Write-Host ("  [{0,3}/{1}] {2}" -f $i, $n, $name)
            } else {
                $fail++
                Write-Host ("  [{0,3}/{1}] skip {2}" -f $i, $n, $r.msg)
            }
        } catch {
            $fail++
            Write-Host ("  [{0,3}/{1}] failed" -f $i, $n)
        }
        Start-Sleep -Milliseconds $delayMs
    }
    Write-Host "done ok=$ok fail=$fail. names should show on /live"
    exit 0
}

$wrk = Get-Command wrk -ErrorAction SilentlyContinue
$useWsl = $false
if (-not $wrk) {
    $wslWrk = wsl -d Ubuntu -- bash -lc "command -v wrk" 2>$null
    if ($LASTEXITCODE -eq 0 -and $wslWrk) {
        $useWsl = $true
    } else {
        Write-Host "wrk is not installed. from an elevated-free shell run:"
        Write-Host '  wsl -d Ubuntu -u root -- bash -lc "apt-get update && apt-get install -y wrk"'
        exit 1
    }
}

$threads = 2
$conns = 16
$dur = "8s"
if ($Stress) {
    $threads = 4
    $conns = 200
    $dur = "10s"
}

Write-Host "open http://localhost:5173/live first, then this run will flood random Chinese names."
Write-Host "wrk -t$threads -c$conns -d$dur  (join is rate-limited at 100/s; many 503s are expected in -Stress)"

if ($useWsl) {
    $lua = $LuaWin
    if ($LuaWin -match '^([A-Za-z]):\\(.+)$') {
        $lua = "/mnt/$($Matches[1].ToLower())/$($Matches[2].Replace('\','/'))"
    }
    # WSL2 cannot use Windows 127.0.0.1. Prefer the Hyper-V WSL nic, then LAN.
    $hostIp = $null
    foreach ($cand in @("172.28.32.1", "192.168.1.3")) {
        $code = wsl -d Ubuntu -- bash -lc "curl -sS -m 2 -o /dev/null -w '%{http_code}' http://${cand}:8888/api/lottery/session || true"
        if ("$code" -match "200") { $hostIp = $cand; break }
    }
    if (-not $hostIp) {
        $hostIp = (Get-NetIPAddress -AddressFamily IPv4 |
            Where-Object { $_.InterfaceAlias -like "*WSL*" -and $_.IPAddress -notlike "127.*" } |
            Select-Object -First 1 -ExpandProperty IPAddress)
    }
    if (-not $hostIp) { $hostIp = "172.28.32.1" }
    $target = $Url.Replace("127.0.0.1", $hostIp).Replace("localhost", $hostIp)
    Write-Host "via WSL wrk -> $target  script=$lua"
    wsl -d Ubuntu -- wrk -t$threads -c$conns -d$dur -s $lua $target
} else {
    wrk -t$threads -c$conns -d$dur -s $LuaWin $Url
}
