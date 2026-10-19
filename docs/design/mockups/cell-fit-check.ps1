#Requires -Version 5.1
<#
.SYNOPSIS
  Headless-Chrome per-cell fit check: three-criteria gate (SPILL / NOTITLE / STRICT).

.DESCRIPTION
  Upgraded from the original scrollWidth>clientWidth single-criterion check.
  For every th/td in .table-wrap table (fallback: all tables) it now applies
  three criteria; any one match makes the cell BAD:
    1. SPILL   : a descendant getBoundingClientRect().right exceeds the cell
                 rect.right + 0.5px (or left < rect.left - 0.5px) AND none of
                 the ancestors between the descendant and the cell have
                 computed overflow-x in {hidden, clip}.
    2. NOTITLE : td.scrollWidth > td.clientWidth but neither the cell nor any
                 descendant carries a title attribute with full text.
    3. STRICT  : columns whose header text contains one of
                 名称/节点数/流量/到期时间/更新时间 must satisfy
                 scrollWidth <= clientWidth.
  Clean ellipsis (URL / status / actions columns) passes via criteria 1+2.
  Exit code 0 when all cells pass, 1 otherwise.

  -SelfTest writes four tiny fixtures under %TEMP% and runs the same measure
  path to prove the gate was not loosened.

.PARAMETER Html
  Absolute path of the target HTML file. Required unless -SelfTest is given.

.PARAMETER Width
  Viewport width in px, default 1600.

.PARAMETER OutPng
  Optional screenshot output path.

.PARAMETER SelfTest
  Run built-in self-test fixtures instead of a single HTML.

.EXAMPLE
  powershell -File cell-fit-check.ps1 -SelfTest
  powershell -File cell-fit-check.ps1 -Html E:\path\d3-console.html -Width 1600
#>
[CmdletBinding()]
param(
  [string]$Html,
  [int]$Width = 1600,
  [string]$OutPng,
  [switch]$SelfTest
)

$ErrorActionPreference = 'Stop'

$chrome = 'C:\Program Files\Google\Chrome\Application\chrome.exe'
if (-not (Test-Path -LiteralPath $chrome)) {
  Write-Error "Chrome not found: $chrome"
  exit 1
}
if ($Width -lt 320) {
  Write-Error "Width must be >= 320, got: $Width"
  exit 1
}

function ConvertTo-FileUrl([string]$Path) {
  $full = [System.IO.Path]::GetFullPath($Path)
  return 'file:///' + ($full -replace '\\', '/')
}

function Invoke-ChromeDump([string]$FileUrl, [int]$W) {
  $ud = Join-Path $env:TEMP ("sf-chrome-" + ([guid]::NewGuid().ToString('N')))
  try {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $chrome
    $psi.Arguments = '--headless=new --disable-gpu --no-sandbox --hide-scrollbars --virtual-time-budget=3000 --window-size=' + $W + ',1000 --user-data-dir="' + $ud + '" --dump-dom "' + $FileUrl + '"'
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $proc = [System.Diagnostics.Process]::Start($psi)
    $dom = $proc.StandardOutput.ReadToEnd()
    $null = $proc.StandardError.ReadToEnd()
    $proc.WaitForExit()
    return $dom
  } finally {
    if (Test-Path -LiteralPath $ud) { Remove-Item -LiteralPath $ud -Recurse -Force -ErrorAction SilentlyContinue }
  }
}

$measureJs = @'
<script>
window.addEventListener('load', function () {
  try {
    var out = [];
    var tables = document.querySelectorAll('.table-wrap table');
    if (!tables.length) { tables = document.querySelectorAll('table'); }
    for (var t = 0; t < tables.length; t++) {
      var tbl = tables[t];
      var capEl = tbl.querySelector('caption');
      var name = (capEl && capEl.textContent) ? capEl.textContent.replace(/[\r\n]+/g, ' ').trim() : '';
      if (!name) { name = 'table#' + t; }
      var strictHeaders = ['名称','节点数','流量','到期时间','更新时间'];
      var strictCols = {};
      var ths = tbl.querySelectorAll('thead th, thead td');
      var hasThead = ths.length > 0;
      if (hasThead) {
        for (var h = 0; h < ths.length; h++) {
          var ht = (ths[h].textContent || '').replace(/[\r\n]+/g, ' ').trim();
          for (var si = 0; si < strictHeaders.length; si++) {
            if (ht.indexOf(strictHeaders[si]) !== -1) { strictCols[h] = strictHeaders[si]; break; }
          }
        }
      }
      var rows = tbl.querySelectorAll('tr');
      for (var r = 0; r < rows.length; r++) {
        var cells = rows[r].querySelectorAll('th, td');
        for (var c = 0; c < cells.length; c++) {
          var cell = cells[c];
          var crect = cell.getBoundingClientRect();
          var cRight = crect.right;
          var cLeft = crect.left;
          var spill = false;
          var spillInfo = '';
          var desc = cell.querySelectorAll('*');
          for (var di = 0; di < desc.length; di++) {
            var el = desc[di];
            var r2 = el.getBoundingClientRect();
            if (r2.width === 0 && r2.height === 0) { continue; }
            var beyond = (r2.right > cRight + 0.5) || (r2.left < cLeft - 0.5);
            if (!beyond) { continue; }
            var guarded = false;
            var node = el.parentElement;
            while (node && node !== cell) {
              var ox = getComputedStyle(node).overflowX;
              if (ox === 'hidden' || ox === 'clip') { guarded = true; break; }
              node = node.parentElement;
            }
            if (!guarded) {
              spill = true;
              spillInfo = 'desc=[' + (el.tagName||'') + '] right=' + r2.right.toFixed(1) + ' cellRight=' + cRight.toFixed(1);
              break;
            }
          }
          var sw = cell.scrollWidth;
          var cw = cell.clientWidth;
          var truncated = sw > cw;
          var hasTitle = false;
          if (truncated) {
            if (cell.hasAttribute && cell.hasAttribute('title') && cell.getAttribute('title')) {
              hasTitle = true;
            } else {
              var tdesc = cell.querySelectorAll('[title]');
              for (var ti = 0; ti < tdesc.length; ti++) {
                var tv = tdesc[ti].getAttribute('title');
                if (tv && tv.length) { hasTitle = true; break; }
              }
            }
          }
          var notitle = truncated && !hasTitle;
          var strict = false;
          var strictReason = '';
          if (hasThead && strictCols.hasOwnProperty(c)) {
            if (sw > cw) {
              strict = true;
              strictReason = 'header=[' + strictCols[c] + '] td sw=' + sw + ' cw=' + cw;
            } else {
              for (var di2 = 0; di2 < desc.length; di2++) {
                var el2 = desc[di2];
                var r2b = el2.getBoundingClientRect();
                if (r2b.width === 0 && r2b.height === 0) { continue; }
                if (el2.scrollWidth > el2.clientWidth) {
                  var clipped = false;
                  var node2 = el2;
                  while (node2 && node2 !== cell) {
                    var cs = getComputedStyle(node2);
                    if (cs.overflowX === 'hidden' || cs.overflowX === 'clip') { clipped = true; break; }
                    node2 = node2.parentElement;
                  }
                  if (clipped) {
                    strict = true;
                    strictReason = 'header=[' + strictCols[c] + '] desc ' + (el2.tagName||'') + ' sw=' + el2.scrollWidth + ' cw=' + el2.clientWidth + ' (ellipsis clip)';
                    break;
                  }
                }
              }
            }
          }
          var txt = (cell.textContent || '').replace(/[\r\n]+/g, ' ').replace(/\|/g, '/').trim();
          if (txt.length > 24) { txt = txt.slice(0, 24); }
          out.push('CELL|' + t + '|' + r + '|' + c + '|' + name + '|' +
            'spill=' + (spill?1:0) + '|notitle=' + (notitle?1:0) + '|strict=' + (strict?1:0) + '|' +
            sw + '|' + cw + '|' + (spill?spillInfo:(strict?strictReason:'')) + '|' + txt);
        }
      }
    }
    out.push('VIEWPORT|' + window.innerWidth + '|' + window.innerHeight);
    var d = document.createElement('div');
    d.id = 'SF_CELLFIT';
    d.style.display = 'none';
    d.setAttribute('data-payload', out.join(' @@ '));
    document.body.appendChild(d);
  } catch (err) {
    var e = document.createElement('div');
    e.id = 'SF_CELLERR';
    e.style.display = 'none';
    e.setAttribute('data-payload', 'THROW:' + err.message);
    document.body.appendChild(e);
  }
});
</script>
'@

function Measure-Html([string]$SourceHtml, [int]$W, [string]$Label) {
  $stamp    = [guid]::NewGuid().ToString('N')
  $tempHtml = Join-Path $env:TEMP ("sf-cell-fit-" + $stamp + ".html")
  try {
    $raw = [System.IO.File]::ReadAllText($SourceHtml, [System.Text.Encoding]::UTF8)
    $injected = $raw.Replace('</body>', $measureJs + '</body>')
    [System.IO.File]::WriteAllText($tempHtml, $injected, (New-Object System.Text.UTF8Encoding($false)))

    $fileUrl = ConvertTo-FileUrl $tempHtml
    $dom = Invoke-ChromeDump -FileUrl $fileUrl -W $W

    $errMatch = [regex]::Match($dom, 'id="SF_CELLERR" data-payload="([^"]*)"')
    if ($errMatch.Success) {
      Write-Host ("page script error: " + [System.Net.WebUtility]::HtmlDecode($errMatch.Groups[1].Value)) -ForegroundColor Red
      return @{ ExitCode = 1; Lines = @(); Count = 0; Bad = 0; Dom = $dom }
    }

    $match = [regex]::Match($dom, 'id="SF_CELLFIT" data-payload="([^"]*)"')
    if (-not $match.Success) {
      Write-Host "measurement marker SF_CELLFIT not found (page may not have finished loading)" -ForegroundColor Red
      return @{ ExitCode = 1; Lines = @(); Count = 0; Bad = 0; Dom = $dom }
    }
    $payload = [System.Net.WebUtility]::HtmlDecode($match.Groups[1].Value)

    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add("")
    $lines.Add("HTML   : " + $SourceHtml)
    $lines.Add("Label  : " + $Label)
    $lines.Add("Width  : " + $W + " (--window-size=" + $W + ",1000)")
    $lines.Add("")

    $bad = 0
    $count = 0
    $items = $payload -split ' @@ '
    foreach ($item in $items) {
      if ($item -eq '') { continue }
      $f = $item -split '\|'
      if ($f[0] -eq 'VIEWPORT') {
        $lines.Add("viewport: innerWidth=" + $f[1] + " innerHeight=" + $f[2])
        continue
      }
      $count++
      $t=$f[1]; $r=$f[2]; $c=$f[3]; $tname=$f[4]
      $spill=[int]$f[5].Substring($f[5].IndexOf('=')+1)
      $notitle=[int]$f[6].Substring($f[6].IndexOf('=')+1)
      $strict=[int]$f[7].Substring($f[7].IndexOf('=')+1)
      $sw=$f[8]; $cw=$f[9]; $info=$f[10]; $txt=$f[11]
      if ($spill -or $notitle -or $strict) {
        $bad++
        $tags = @()
        if ($spill)  { $tags += 'SPILL' }
        if ($notitle){ $tags += 'NOTITLE' }
        if ($strict) { $tags += 'STRICT' }
        $lines.Add("BAD table=" + $tname + " row=" + $r + " col=" + $c + " [" + ($tags -join '/') + "] sw=" + $sw + " cw=" + $cw + " " + $info + " text=[" + $txt + "]")
      }
    }

    $lines.Add("")
    if ($count -eq 0) {
      $lines.Add("FAIL: no table cells found")
      return @{ ExitCode = 1; Lines = $lines; Count = 0; Bad = 0; Dom = $dom }
    }
    if ($bad -eq 0) {
      $lines.Add("PASS: " + $count + " cells fit (all SPILL/NOTITLE/STRICT clean)")
    } else {
      $lines.Add("FAIL: " + $bad + "/" + $count + " cells bad")
    }
    return @{ ExitCode = ($(if ($bad -eq 0) {0} else {1})); Lines = $lines; Count = $count; Bad = $bad; Dom = $dom }
  } finally {
    if (Test-Path -LiteralPath $tempHtml) { Remove-Item -LiteralPath $tempHtml -Force -ErrorAction SilentlyContinue }
  }
}

function New-SelfTestFixtures([string]$Dir) {
  $fxA = @'
<table>
  <caption>A 全装得下</caption>
  <thead><tr><th>名称</th><th>节点数</th></tr></thead>
  <tbody>
    <tr><td>甲机场</td><td>186</td></tr>
    <tr><td>乙机场</td><td>92</td></tr>
  </tbody>
</table>
'@
  $fxB = @'
<table>
  <caption>B 溢出无 title</caption>
  <thead><tr><th>操作</th></tr></thead>
  <tbody>
    <tr><td><span style="white-space:nowrap">这是一段非常非常非常非常非常非常非常非常长的文本没有任何裁剪也没有title属性</span></td></tr>
  </tbody>
</table>
'@
  $fxC = @'
<table>
  <caption>C 合法省略号带 title</caption>
  <thead><tr><th>订阅地址</th></tr></thead>
  <tbody>
    <tr><td><span title="https://sub.example.com/api/v1/client/subscribe?token=verylongtokenvalue" style="display:inline-block;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">https://sub.example.com/api/v1/client/subscribe?token=verylongtokenvalue</span></td></tr>
  </tbody>
</table>
'@
  $fxD = @'
<table>
  <caption>D 严格列被省略号</caption>
  <thead><tr><th>名称</th></tr></thead>
  <tbody>
    <tr><td><span style="display:inline-block;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">这是一个特别特别特别特别特别特别长到必须省略号截断的机场名称</span></td></tr>
  </tbody>
</table>
'@
  $set = [ordered]@{ A = $fxA; B = $fxB; C = $fxC; D = $fxD }
  $paths = @{}
  foreach ($k in 'A','B','C','D') {
    $p = Join-Path $Dir ("sf-selftest-" + $k + ".html")
    $full = '<!doctype html><html><head><meta charset="utf-8"><style>' +
            'table{border-collapse:collapse}td,th{border:1px solid #ccc;padding:4px 8px;max-width:160px}' +
            '.table-wrap{width:480px}</style></head><body><div class="table-wrap">' +
            $set[$k] + '</div></body></html>'
    [System.IO.File]::WriteAllText($p, $full, (New-Object System.Text.UTF8Encoding($true)))
    $paths[$k] = $p
  }
  return $paths
}

if ($SelfTest) {
  $expected = [ordered]@{
    A = @{ Exit = 0; Want = 'PASS'; Neg = '' }
    B = @{ Exit = 1; Want = ''; Neg = 'SPILL' }
    C = @{ Exit = 0; Want = 'PASS'; Neg = '' }
    D = @{ Exit = 1; Want = ''; Neg = 'STRICT' }
  }
  $allOk = $true
  $report = New-Object System.Collections.Generic.List[string]
  $report.Add("SELFTEST fixtures: A=fit B=spill/notitle C=clean-ellipsis+title D=strict-overflow")
  $paths = New-SelfTestFixtures $env:TEMP
  foreach ($k in 'A','B','C','D') {
    $res = Measure-Html -SourceHtml $paths[$k] -W 1600 -Label ("fixture " + $k)
    $joined = ($res.Lines -join "`n")
    $exp = $expected[$k]
    $ok = $true
    $detail = ""
    if ($res.ExitCode -ne $exp.Exit) { $ok = $false; $detail = "exit=" + $res.ExitCode + " expected=" + $exp.Exit }
    if ($exp.Want -and ($joined -notlike "*$($exp.Want)*")) { $ok = $false; $detail = $detail + " missing[" + $exp.Want + "]" }
    if ($exp.Neg -and ($joined -notlike "*$($exp.Neg)*")) { $ok = $false; $detail = $detail + " missing[" + $exp.Neg + "]" }
    if (-not $ok) { $allOk = $false }
    $report.Add("[" + $k + "] exit=" + $res.ExitCode + " want=" + $exp.Exit + " " + $(if($ok){"OK"}else{"MISMATCH "+$detail}))
  }
  foreach ($k in 'A','B','C','D') {
    if (Test-Path -LiteralPath $paths[$k]) { Remove-Item -LiteralPath $paths[$k] -Force -ErrorAction SilentlyContinue }
  }
  if ($allOk) {
    Write-Host "SELFTEST PASS" -ForegroundColor Green
    foreach ($l in $report) { Write-Host $l }
    exit 0
  } else {
    Write-Host "SELFTEST FAIL" -ForegroundColor Red
    foreach ($l in $report) { Write-Host $l }
    exit 1
  }
}

if (-not $Html) {
  Write-Error "-Html is required unless -SelfTest is given."
  exit 1
}
if (-not (Test-Path -LiteralPath $Html)) {
  Write-Error "HTML not found: $Html"
  exit 1
}

$res = Measure-Html -SourceHtml $Html -W $Width -Label "single"
foreach ($l in $res.Lines) {
  if ($l -like 'BAD *') { Write-Host $l -ForegroundColor Red }
  elseif ($l -like 'PASS *') { Write-Host $l -ForegroundColor Green }
  elseif ($l -like 'FAIL *') { Write-Host $l -ForegroundColor Red }
  else { Write-Host $l }
}

if ($OutPng) {
  $outDir = Split-Path -Parent $OutPng
  if ($outDir -and -not (Test-Path -LiteralPath $outDir)) { $null = New-Item -ItemType Directory -Path $outDir -Force }
  $shotUrl = ConvertTo-FileUrl $Html
  $ud = Join-Path $env:TEMP ("sf-chrome-" + ([guid]::NewGuid().ToString('N')))
  try {
    $psi2 = New-Object System.Diagnostics.ProcessStartInfo
    $psi2.FileName = $chrome
    $psi2.Arguments = '--headless=new --disable-gpu --no-sandbox --hide-scrollbars --force-device-scale-factor=1 --window-size=' + $Width + ',2400 --user-data-dir="' + $ud + '" --screenshot="' + $OutPng + '" "' + $shotUrl + '"'
    $psi2.UseShellExecute = $false
    $psi2.RedirectStandardOutput = $true
    $psi2.RedirectStandardError = $true
    $psi2.CreateNoWindow = $true
    $p2 = [System.Diagnostics.Process]::Start($psi2)
    $null = $p2.StandardOutput.ReadToEnd()
    $null = $p2.StandardError.ReadToEnd()
    $p2.WaitForExit()
  } finally {
    if (Test-Path -LiteralPath $ud) { Remove-Item -LiteralPath $ud -Recurse -Force -ErrorAction SilentlyContinue }
  }
  if (Test-Path -LiteralPath $OutPng) {
    $len = (Get-Item -LiteralPath $OutPng).Length
    Write-Host ("screenshot: " + $OutPng + " (" + $len + " bytes)") -ForegroundColor Green
  } else {
    Write-Host ("screenshot not created: " + $OutPng) -ForegroundColor Red
  }
}

exit $res.ExitCode
