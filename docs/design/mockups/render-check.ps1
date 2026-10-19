#requires -Version 5.1
<#
.SYNOPSIS
  无头 Chrome 渲染检查：量 .table-wrap 横向溢出 + 截图。

.DESCRIPTION
  1) 把目标 HTML 复制到 %TEMP%，注入一段在 load 时执行的内联脚本；
     脚本遍历 document.querySelectorAll('.table-wrap')，把
     scrollWidth / clientWidth / over 写进一个隐藏 div 的 data-payload。
  2) 用 chrome --headless=new --dump-dom 取回渲染后的 DOM，解析出上面的数据。
  3) 用 chrome --headless=new --screenshot 出一张 PNG。
  必须带 --window-size，否则量出来的溢出不真实（会恒为 0）。

.PARAMETER Html
  目标 HTML 的绝对路径。

.PARAMETER Width
  视口宽度，默认 1600。

.PARAMETER OutPng
  截图输出路径；不传则不出图。

.EXAMPLE
  pwsh -File render-check.ps1 -Html E:\path\d3-console.html -Width 1600 -OutPng $env:TEMP\d3.png
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$Html,
  [int]$Width = 1600,
  [string]$OutPng
)

$ErrorActionPreference = 'Stop'

$chrome = 'C:\Program Files\Google\Chrome\Application\chrome.exe'
if (-not (Test-Path -LiteralPath $chrome)) {
  Write-Error "找不到 Chrome：$chrome"
  exit 1
}
if (-not (Test-Path -LiteralPath $Html)) {
  Write-Error "找不到 HTML：$Html"
  exit 1
}
if ($Width -lt 320) {
  Write-Error "Width 至少 320，收到：$Width"
  exit 1
}

function ConvertTo-FileUrl([string]$Path) {
  $full = [System.IO.Path]::GetFullPath($Path)
  return 'file:///' + ($full -replace '\\', '/')
}

$stamp    = [guid]::NewGuid().ToString('N')
$tempHtml = Join-Path $env:TEMP "sf-render-check-$stamp.html"
$userData = Join-Path $env:TEMP "sf-chrome-$stamp"

$measureJs = @'
<script>
window.addEventListener('load', function () {
  try {
    var out = [];
    var wraps = document.querySelectorAll('.table-wrap');
    for (var i = 0; i < wraps.length; i++) {
      var w = wraps[i];
      var t = w.querySelector('table');
      var cap = t ? (t.querySelector('caption') ? t.querySelector('caption').textContent : '') : '';
      out.push('T' + i + '|' + w.scrollWidth + '|' + w.clientWidth + '|' + (w.scrollWidth - w.clientWidth) + '|' + Math.round(t ? t.getBoundingClientRect().width : 0) + '|' + cap);
    }
    out.push('VIEWPORT|' + window.innerWidth + '|' + window.innerHeight + '|0|0|');
    var d = document.createElement('div');
    d.id = 'SF_MEASURE';
    d.style.display = 'none';
    d.setAttribute('data-payload', out.join(' @@ '));
    document.body.appendChild(d);
  } catch (err) {
    var e = document.createElement('div');
    e.id = 'SF_ERR';
    e.style.display = 'none';
    e.setAttribute('data-payload', 'THROW:' + err.message);
    document.body.appendChild(e);
  }
});
</script>
'@

try {
  $raw = [System.IO.File]::ReadAllText($Html, [System.Text.Encoding]::UTF8)
  $injected = $raw.Replace('</body>', $measureJs + '</body>')
  [System.IO.File]::WriteAllText($tempHtml, $injected, (New-Object System.Text.UTF8Encoding($false)))

  $fileUrl = ConvertTo-FileUrl $tempHtml

  $psi = New-Object System.Diagnostics.ProcessStartInfo
  $psi.FileName = $chrome
  $psi.Arguments = '--headless=new --disable-gpu --no-sandbox --hide-scrollbars --virtual-time-budget=3000 --window-size=' + $Width + ',1000 --user-data-dir="' + $userData + '" --dump-dom "' + $fileUrl + '"'
  $psi.UseShellExecute = $false
  $psi.RedirectStandardOutput = $true
  $psi.RedirectStandardError = $true
  $psi.CreateNoWindow = $true
  $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
  $proc = [System.Diagnostics.Process]::Start($psi)
  $dom = $proc.StandardOutput.ReadToEnd()
  $null = $proc.StandardError.ReadToEnd()
  $proc.WaitForExit()

  $errMatch = [regex]::Match($dom, 'id="SF_ERR" data-payload="([^"]*)"')
  if ($errMatch.Success) {
    Write-Host ("页面脚本报错：" + [System.Net.WebUtility]::HtmlDecode($errMatch.Groups[1].Value)) -ForegroundColor Red
  }

  $match = [regex]::Match($dom, 'id="SF_MEASURE" data-payload="([^"]*)"')
  if (-not $match.Success) {
    Write-Host "未取到测量标记 SF_MEASURE（页面可能没加载完）" -ForegroundColor Red
    exit 1
  }
  $payload = [System.Net.WebUtility]::HtmlDecode($match.Groups[1].Value)

  Write-Host ""
  Write-Host ("HTML   : " + $Html)
  Write-Host ("Width  : " + $Width + " (--window-size=" + $Width + ",1000)")
  Write-Host ""

  $bad = 0
  $count = 0
  foreach ($item in ($payload -split ' @@ ')) {
    if ($item -eq '') { continue }
    $f = $item -split '\|'
    if ($f[0] -eq 'VIEWPORT') {
      Write-Host ("viewport: innerWidth=" + $f[1] + " innerHeight=" + $f[2])
      continue
    }
    $count++
    $over = [int]$f[3]
    $color = if ($over -eq 0) { 'Green' } else { 'Red' }
    Write-Host ("{0}: scrollWidth={1} clientWidth={2} over={3} tableW={4} caption={5}" -f $f[0], $f[1], $f[2], $over, $f[4], $f[5]) -ForegroundColor $color
    if ($over -ne 0) { $bad++ }
  }

  if ($OutPng) {
    $outDir = Split-Path -Parent $OutPng
    if ($outDir -and -not (Test-Path -LiteralPath $outDir)) { $null = New-Item -ItemType Directory -Path $outDir -Force }
    $shotUrl = ConvertTo-FileUrl $Html
    $psi2 = New-Object System.Diagnostics.ProcessStartInfo
    $psi2.FileName = $chrome
    $psi2.Arguments = '--headless=new --disable-gpu --no-sandbox --hide-scrollbars --force-device-scale-factor=1 --window-size=' + $Width + ',2400 --user-data-dir="' + $userData + '" --screenshot="' + $OutPng + '" "' + $shotUrl + '"'
    $psi2.UseShellExecute = $false
    $psi2.RedirectStandardOutput = $true
    $psi2.RedirectStandardError = $true
    $psi2.CreateNoWindow = $true
    $p2 = [System.Diagnostics.Process]::Start($psi2)
    $null = $p2.StandardOutput.ReadToEnd()
    $null = $p2.StandardError.ReadToEnd()
    $p2.WaitForExit()
    if (Test-Path -LiteralPath $OutPng) {
      $len = (Get-Item -LiteralPath $OutPng).Length
      Write-Host ("screenshot: " + $OutPng + " (" + $len + " bytes)") -ForegroundColor Green
    } else {
      Write-Host ("screenshot 未生成：" + $OutPng) -ForegroundColor Red
      $bad++
    }
  }

  Write-Host ""
  if ($count -eq 0) {
    Write-Host "未找到任何 .table-wrap" -ForegroundColor Red
    exit 1
  }
  if ($bad -eq 0) {
    Write-Host ("PASS: " + $count + " 个 .table-wrap 全部 over=0") -ForegroundColor Green
    exit 0
  } else {
    Write-Host ("FAIL: " + $bad + " 个 .table-wrap 存在横向溢出") -ForegroundColor Red
    exit 1
  }
}
finally {
  if (Test-Path -LiteralPath $tempHtml) { Remove-Item -LiteralPath $tempHtml -Force -ErrorAction SilentlyContinue }
  if (Test-Path -LiteralPath $userData) { Remove-Item -LiteralPath $userData -Recurse -Force -ErrorAction SilentlyContinue }
}
