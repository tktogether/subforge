<#
.SYNOPSIS
    将 docs/wiki/ 下的 Markdown 源文件发布到 GitHub Wiki。

.DESCRIPTION
    SubForge 的 Wiki 采用「源文件在仓库内、发布到 Wiki 仓库」的方式维护：

        docs/wiki/*.md  ──(本脚本)──>  <owner>/<repo>.wiki.git

    这样做的好处是 Wiki 内容与代码一起评审、一起提交，可以追溯每一次修改；
    代价是网页端的直接编辑会在下一次发布时被覆盖。因此请勿在网页端编辑页面。

    脚本执行流程：
      1. 校验 git 可用，且 docs/wiki/ 中存在 Home.md（Wiki 必须有首页）。
      2. 克隆或更新 Wiki 仓库到本地缓存目录（默认 .wiki-cache/，位于仓库根目录）。
      3. 把 docs/wiki/ 下的全部文件复制到缓存目录，删除缓存目录中已不再存在的
         Markdown 页面（可用 -NoPrune 关闭该行为）。
      4. 有变更时提交并推送；无变更时直接退出，不产生空提交。

.PARAMETER Remote
    Wiki 仓库地址。默认取当前 git 远程 origin 推导出的 <owner>/<repo>.wiki.git，
    也可以显式传入，便于在本地用临时仓库验证脚本行为。

.PARAMETER CacheDir
    本地缓存目录。默认为仓库根目录下的 .wiki-cache。

.PARAMETER Message
    提交信息。默认按时间生成。

.PARAMETER NoPrune
    只新增与更新页面，不删除 Wiki 中多余的页面。

.PARAMETER DryRun
    只展示将要发生的变更，不提交、不推送。

.EXAMPLE
    pwsh -File scripts/publish-wiki.ps1 -DryRun

    预览本次发布会新增、更新和删除哪些页面。

.EXAMPLE
    pwsh -File scripts/publish-wiki.ps1 -Message "docs(wiki): 补充渲染器矩阵说明"

    正式发布并指定提交信息。

.NOTES
    本脚本同时兼容 Windows PowerShell 5.1 与 PowerShell 7；若环境中没有 pwsh，
    把示例中的 pwsh 换成 powershell 即可。

    GitHub 的 Wiki 仓库在「首次创建首页」之前并不存在，git 推送会报
    "Repository not found"。此时需要先在浏览器中打开仓库的 Wiki 标签页，
    创建任意一个首页（内容可以是占位文本），之后本脚本即可正常推送。
#>
[CmdletBinding()]
param(
    [string]$Remote,
    [string]$CacheDir,
    [string]$Message,
    [switch]$NoPrune,
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# git 调用封装
#
# 原生命令写入 stderr 时，在 $ErrorActionPreference='Stop' 下会被 PowerShell
# 当作终止性错误抛出（例如对空仓库执行 clone 会产生 warning）。git 的正常
# 诊断信息大量走 stderr，因此这里统一降级错误偏好，只依据退出码判断成败。
# ---------------------------------------------------------------------------
function Invoke-Git {
    param(
        [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments
    )
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'
    try {
        $output = & git @Arguments 2>&1 | Out-String
    } finally {
        $ErrorActionPreference = $previous
    }
    return [pscustomobject]@{
        Output   = $output.Trim()
        ExitCode = $LASTEXITCODE
    }
}

# ---------------------------------------------------------------------------
# 基础路径与工具检查
# ---------------------------------------------------------------------------

$repoRoot = Split-Path -Parent $PSScriptRoot
$sourceDir = Join-Path $repoRoot 'docs/wiki'

if (-not (Test-Path -LiteralPath $sourceDir -PathType Container)) {
    throw "未找到 Wiki 源文件目录：$sourceDir"
}
if (-not (Test-Path -LiteralPath (Join-Path $sourceDir 'Home.md') -PathType Leaf)) {
    throw "Wiki 源文件目录缺少首页 Home.md：$sourceDir"
}
if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    throw '未找到 git 命令，请先安装 Git 并确保其在 PATH 中。'
}

if (-not $CacheDir) {
    $CacheDir = Join-Path $repoRoot '.wiki-cache'
}

# 默认远程地址：由 origin 推导 <owner>/<repo>.wiki.git
if (-not $Remote) {
    $origin = (Invoke-Git -C $repoRoot remote get-url origin).Output
    if (-not $origin) {
        throw '未找到 git 远程 origin，请使用 -Remote 显式指定 Wiki 仓库地址。'
    }
    $slug = $origin -replace '^git@github\.com:', '' `
                    -replace '^https?://github\.com/', '' `
                    -replace '\.git$', ''
    if ($slug -notmatch '^[^/]+/[^/]+$') {
        throw "无法从远程地址推导 Wiki 仓库：$origin，请使用 -Remote 显式指定。"
    }
    $Remote = "https://github.com/$slug.wiki.git"
}

Write-Host "Wiki 源目录：$sourceDir"
Write-Host "Wiki 远程：  $Remote"
Write-Host "本地缓存：   $CacheDir"

# ---------------------------------------------------------------------------
# 检查 Wiki 仓库是否已初始化
# ---------------------------------------------------------------------------

$probe = Invoke-Git ls-remote --heads $Remote
if ($probe.ExitCode -ne 0) {
    Write-Warning "无法访问 Wiki 仓库：$Remote"
    Write-Warning $probe.Output
    $webUrl = $Remote -replace '\.wiki\.git$', '/wiki'
    throw @"
GitHub 的 Wiki 仓库需要先在网页端创建首页后才会存在。请执行以下步骤：
  1. 打开仓库的 Wiki 标签页：$webUrl
  2. 点击 "Create the first page"，创建任意首页（内容可先写占位文本）
  3. 重新运行本脚本，脚本会用 docs/wiki/ 的内容覆盖该占位页
"@
}

# ---------------------------------------------------------------------------
# 克隆或更新本地缓存
# ---------------------------------------------------------------------------

if (Test-Path -LiteralPath (Join-Path $CacheDir '.git')) {
    Write-Host '更新本地 Wiki 缓存…'
    $null = Invoke-Git -C $CacheDir remote set-url origin $Remote
    $fetch = Invoke-Git -C $CacheDir fetch --prune origin
    if ($fetch.ExitCode -ne 0) { throw "git fetch 失败：$($fetch.Output)" }
} else {
    Write-Host '克隆 Wiki 仓库…'
    if (Test-Path -LiteralPath $CacheDir) {
        Remove-Item -LiteralPath $CacheDir -Recurse -Force
    }
    $clone = Invoke-Git -c core.autocrlf=false clone --quiet $Remote $CacheDir
    if ($clone.ExitCode -ne 0) { throw "git clone 失败：$($clone.Output)" }
}

# 强制缓存仓库使用 LF：Windows 上全局 core.autocrlf=true 时，git 检出会把
# 换行符改写成 CRLF，导致"内容其实没变、但文件哈希不同"的假差异，进而在
# 没有任何真实改动时尝试提交空变更。写入缓存仓库自己的 config，一次生效。
$null = Invoke-Git -C $CacheDir config core.autocrlf false
$null = Invoke-Git -C $CacheDir config core.eol lf

# 分支名以远程默认分支为准：GitHub Wiki 的传统默认分支是 master，
# 不能假定为 main，否则会推到一个新分支上，网页端看不到任何变化。
$defaultRef = (Invoke-Git -C $CacheDir symbolic-ref --short refs/remotes/origin/HEAD).Output
$branch = if ($defaultRef) { $defaultRef -replace '^origin/', '' } else { 'master' }
Write-Host "Wiki 分支：  $branch"

$hasHead = (Invoke-Git -C $CacheDir rev-parse --verify --quiet HEAD).ExitCode -eq 0

if ($hasHead) {
    $checkout = Invoke-Git -C $CacheDir checkout --quiet -B $branch "origin/$branch"
    if ($checkout.ExitCode -ne 0) { throw "切换到分支 $branch 失败：$($checkout.Output)" }
    $reset = Invoke-Git -C $CacheDir reset --quiet --hard "origin/$branch"
    if ($reset.ExitCode -ne 0) { throw "重置到 origin/$branch 失败：$($reset.Output)" }
} else {
    # 仓库存在但没有任何提交（首页尚未在网页端创建）：建立一个未出生分支
    $checkout = Invoke-Git -C $CacheDir checkout --quiet -B $branch
    if ($checkout.ExitCode -ne 0) { throw "创建分支 $branch 失败：$($checkout.Output)" }
}

# ---------------------------------------------------------------------------
# 同步文件：新增、更新、清理
# ---------------------------------------------------------------------------

$sourceFiles = Get-ChildItem -LiteralPath $sourceDir -File -Recurse
$added = @()
$updated = @()
$unchanged = @()

foreach ($file in $sourceFiles) {
    $relative = $file.FullName.Substring($sourceDir.Length).TrimStart('\', '/')
    $target = Join-Path $CacheDir $relative
    $targetDir = Split-Path -Parent $target
    if (-not (Test-Path -LiteralPath $targetDir)) {
        New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
    }

    $isNew = -not (Test-Path -LiteralPath $target -PathType Leaf)
    $changed = $true
    if (-not $isNew) {
        $srcHash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash
        $dstHash = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash
        $changed = $srcHash -ne $dstHash
    }

    if ($isNew) {
        $added += $relative
    } elseif ($changed) {
        $updated += $relative
    } else {
        $unchanged += $relative
    }

    if ($isNew -or $changed) {
        Copy-Item -LiteralPath $file.FullName -Destination $target -Force
    }
}

$removed = @()
if (-not $NoPrune) {
    $sourceNames = $sourceFiles | ForEach-Object {
        $_.FullName.Substring($sourceDir.Length).TrimStart('\', '/')
    }
    $existing = Get-ChildItem -LiteralPath $CacheDir -File -Recurse -Filter '*.md' |
        Where-Object { $_.FullName -notlike (Join-Path $CacheDir '.git*') }
    foreach ($file in $existing) {
        $relative = $file.FullName.Substring($CacheDir.Length).TrimStart('\', '/')
        if ($sourceNames -notcontains $relative) {
            $removed += $relative
            Remove-Item -LiteralPath $file.FullName -Force
        }
    }
}

Write-Host ''
Write-Host "新增页面：$($added.Count)"
$added | ForEach-Object { Write-Host "  + $_" }
Write-Host "更新页面：$($updated.Count)"
$updated | ForEach-Object { Write-Host "  ~ $_" }
Write-Host "删除页面：$($removed.Count)"
$removed | ForEach-Object { Write-Host "  - $_" }
Write-Host "未变化：  $($unchanged.Count)"

if (($added.Count + $updated.Count + $removed.Count) -eq 0) {
    Write-Host ''
    Write-Host 'Wiki 已是最新状态，无需发布。'
    return
}

if ($DryRun) {
    Write-Host ''
    Write-Host 'DryRun：已跳过提交与推送。'
    return
}

# ---------------------------------------------------------------------------
# 提交与推送
# ---------------------------------------------------------------------------

if (-not $Message) {
    $Message = "docs(wiki): 同步 Wiki 内容 $(Get-Date -Format 'yyyy-MM-dd HH:mm')"
}

$add = Invoke-Git -C $CacheDir add --all
if ($add.ExitCode -ne 0) { throw "git add 失败：$($add.Output)" }

# 暂存区为空说明真实内容与 Wiki 一致（哈希差异只来自换行符等表现差异），
# 此时不提交空变更，避免 git commit 以 "nothing to commit" 失败。
$staged = Invoke-Git -C $CacheDir diff --cached --quiet
if ($staged.ExitCode -eq 0) {
    Write-Host ''
    Write-Host 'Wiki 内容与仓库一致，无需提交。'
    return
}

$commit = Invoke-Git -C $CacheDir -c user.name=subforge-wiki-bot `
                                -c user.email=wiki-bot@users.noreply.github.com `
                                commit --quiet -m $Message
if ($commit.ExitCode -ne 0) { throw "git commit 失败：$($commit.Output)" }

$push = Invoke-Git -C $CacheDir push --quiet origin "HEAD:$branch"
if ($push.ExitCode -ne 0) { throw "git push 失败：$($push.Output)" }

Write-Host ''
Write-Host "发布完成：$Message"
Write-Host "查看结果：$($Remote -replace '\.wiki\.git$', '')"
