#!/usr/bin/env node
/**
 * SubForge 文档自检脚本（零依赖，Node.js >= 20）。
 *
 * 用途：在提交前检查 Markdown 文档与 GitHub Wiki 源文件的结构性问题。
 * 覆盖四类检查：
 *   1. 代码围栏闭合性与 Mermaid 图类型合法性；
 *   2. Mermaid 语法高危写法（flowchart 节点文本内的英文括号/方括号、未配对引号等）；
 *   3. Markdown 链接可达性（仓库内相对路径链接、Wiki 页面名链接）；
 *   4. Wiki 页面与文件名的一致性、页面必备结构。
 *
 * 用法：
 *   node scripts/check-docs.mjs            # 检查全部文档
 *   node scripts/check-docs.mjs --quiet    # 只输出问题与统计
 *
 * 退出码：0 表示无问题，1 表示存在错误（警告不影响退出码）。
 */

import { readFileSync, readdirSync, existsSync, statSync } from 'node:fs';
import { join, dirname, resolve, relative, basename, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, '..');
const quiet = process.argv.includes('--quiet');

/**
 * 版本库已跟踪文件的绝对路径集合。
 *
 * 用途：链接目标在本机存在、却没进版本库（典型情况是被 .gitignore 忽略的本地文件）
 * 时，别人克隆下来点这个链接就是 404——本机自检却会通过。这类"本机能跑、CI 就红"
 * 的差异必须在这里挡住。
 *
 * 非 git 环境（例如解压出来的源码包）返回 null，此时跳过该项校验。
 */
const trackedFiles = (() => {
  try {
    const out = execFileSync('git', ['ls-files', '-z'], {
      cwd: repoRoot,
      encoding: 'utf8',
      maxBuffer: 64 * 1024 * 1024,
      stdio: ['ignore', 'pipe', 'ignore'],
    });
    return new Set(out.split('\0').filter(Boolean).map((p) => resolve(repoRoot, p)));
  } catch {
    return null;
  }
})();

/**
 * 至少包含一个已跟踪文件的目录集合。
 *
 * `git ls-files` 只列文件，而文档里的链接经常指向目录（例如 `deploy/openwrt/`）：
 * 判断"克隆后这个目录链接还在不在"，要看它下面有没有已跟踪的文件。
 */
const trackedDirs = (() => {
  if (trackedFiles === null) return null;
  const dirs = new Set();
  for (const p of trackedFiles) {
    let d = dirname(p);
    while (d.startsWith(repoRoot) && !dirs.has(d)) {
      dirs.add(d);
      const parent = dirname(d);
      if (parent === d) break;
      d = parent;
    }
  }
  return dirs;
})();

const errors = [];
const warnings = [];

/** 收集待检查的 Markdown 文件：仓库根目录的 *.md、docs/ 下的全部 *.md。 */
function collectMarkdownFiles() {
  const files = [];
  for (const entry of readdirSync(repoRoot)) {
    if (extname(entry) === '.md' && statSync(join(repoRoot, entry)).isFile()) {
      files.push(join(repoRoot, entry));
    }
  }
  const walk = (dir) => {
    if (!existsSync(dir)) return;
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) {
        if (entry === 'node_modules' || entry === '.git') continue;
        walk(full);
      } else if (extname(entry) === '.md') {
        files.push(full);
      }
    }
  };
  walk(join(repoRoot, 'docs'));
  return files.sort();
}

/** Mermaid 支持的图类型关键字。出现未知关键字通常是拼写错误。 */
const DIAGRAM_KEYWORDS = [
  'flowchart', 'graph', 'sequenceDiagram', 'stateDiagram-v2', 'stateDiagram',
  'erDiagram', 'classDiagram', 'journey', 'gantt', 'pie', 'mindmap', 'timeline',
  'gitGraph', 'quadrantChart', 'xychart-beta', 'sankey-beta', 'block-beta',
  'packet-beta', 'architecture-beta', 'radar-beta', 'treemap-beta', 'C4Context',
  'requirementDiagram', 'zenuml',
];

const rel = (p) => relative(repoRoot, p).replace(/\\/g, '/');

/**
 * 切分围栏代码块，返回正文行与 mermaid 块。
 * 同时检查围栏是否闭合、mermaid 块是否声明了合法图类型。
 */
function splitFences(lines, file) {
  const body = [];
  const mermaidBlocks = [];
  let inFence = false;
  let fenceMarker = '';
  let fenceStart = 0;
  let fenceLang = '';
  let buffer = [];

  lines.forEach((entry) => {
    const line = entry.text;
    const lineNo = entry.lineNo;
    const fence = line.match(/^\s*(`{3,}|~{3,})\s*([A-Za-z0-9_+-]*)\s*$/);
    if (!inFence && fence) {
      inFence = true;
      fenceMarker = fence[1][0];
      fenceStart = lineNo;
      fenceLang = (fence[2] || '').toLowerCase();
      buffer = [];
      return;
    }
    if (inFence && fence && fence[1][0] === fenceMarker) {
      if (fenceLang === 'mermaid') {
        mermaidBlocks.push({ startLine: fenceStart, lines: buffer });
      }
      inFence = false;
      fenceLang = '';
      buffer = [];
      return;
    }
    if (inFence) {
      buffer.push({ text: line, lineNo });
    } else {
      body.push({ text: line, lineNo });
    }
  });

  if (inFence) {
    errors.push(`${rel(file)}:${fenceStart} 代码围栏未闭合（缺少结尾的 ${fenceMarker.repeat(3)}）`);
  }

  return { body, mermaidBlocks };
}

/** 检查单个 mermaid 代码块。 */
function checkMermaidBlock(block, file) {
  const where = `${rel(file)}:${block.startLine}`;
  const content = block.lines.map((l) => l.text);
  const meaningful = content.filter((l) => l.trim() !== '' && !l.trim().startsWith('%%'));

  if (meaningful.length === 0) {
    errors.push(`${where} mermaid 代码块为空`);
    return;
  }

  const first = meaningful[0].trim();
  const keyword = first.split(/[\s({[]/)[0];
  if (!DIAGRAM_KEYWORDS.includes(keyword)) {
    errors.push(`${where} mermaid 图类型无法识别：${keyword}（合法取值如 flowchart / sequenceDiagram / stateDiagram-v2 / erDiagram）`);
  }

  content.forEach((text, index) => {
    const lineNo = block.lines[index].lineNo;
    const at = `${rel(file)}:${lineNo}`;
    const trimmed = text.trim();
    if (trimmed === '' || trimmed.startsWith('%%')) return;

    if (/\t/.test(text)) {
      warnings.push(`${at} mermaid 行内包含制表符，建议改用空格缩进`);
    }

    // 未配对的双引号会让 Mermaid 解析失败。
    const quotes = (trimmed.match(/"/g) || []).length;
    if (quotes % 2 !== 0) {
      errors.push(`${at} mermaid 行内双引号未配对：${trimmed}`);
    }

    // flowchart / graph：节点文本使用 [...] 或 {...} 时，内部不能再出现英文括号，
    // 否则 GitHub 的 Mermaid 渲染会报语法错误。这是本仓库最常见的问题写法。
    if (/^(flowchart|graph)\b/.test(first) || /-->|---|==>/.test(trimmed)) {
      const nodeTexts = trimmed.match(/\[[^\]]*\]|\{[^}]*\}/g) || [];
      for (const nodeText of nodeTexts) {
        let inner = nodeText.slice(1, -1);
        // 圆柱体形状 [(文本)] 的括号属于语法本身，先剥掉一层再判断。
        if (inner.startsWith('(') && inner.endsWith(')')) {
          inner = inner.slice(1, -1);
        }
        if (/[()]/.test(inner)) {
          errors.push(`${at} flowchart 节点文本内出现英文括号，会导致渲染失败：${nodeText}`);
        }
        if (/[\[\]{}]/.test(inner)) {
          errors.push(`${at} flowchart 节点文本内出现嵌套的方括号或花括号：${nodeText}`);
        }
      }
    }

    // 边标签语法 -->|文本| 中不能再出现竖线。
    const edgeLabels = trimmed.match(/\|[^|]*\|/g) || [];
    for (const label of edgeLabels) {
      if (label.length > 120) {
        warnings.push(`${at} 边标签过长，建议缩短：${label.slice(0, 40)}…`);
      }
    }

    // 剔除合法的内联标签与箭头记号后再判断是否残留裸尖括号。
    const withoutTags = trimmed
      .replace(/<\/?(?:br|b|i|u|s|em|strong|code|small|sub|sup|span|div)\s*\/?>/g, '')
      .replace(/<?-{1,2}[->)x]+>?/g, '')
      .replace(/<<-?>>/g, '');
    if (/[<>]/.test(withoutTags)) {
      warnings.push(`${at} mermaid 行内出现裸尖括号，可能被当作 HTML 标签：${trimmed}`);
    }
  });
}

/** 检查 Markdown 链接。 */
function checkLinks(lines, file) {
  const wikiDir = join(repoRoot, 'docs', 'wiki');
  const isWikiPage = dirname(file) === wikiDir;
  const linkPattern = /\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;

  lines.forEach(({ text, lineNo }) => {
    // 行内代码里的方括号写法（例如文档在讲解链接语法本身）不是真链接，先剔除。
    const searchable = text.replace(/`[^`]*`/g, (span) => ' '.repeat(span.length));
    let match;
    while ((match = linkPattern.exec(searchable)) !== null) {
      const target = match[1];
      if (/^(https?:|mailto:|#)/.test(target)) continue;
      const path = target.split('#')[0];
      if (path === '') continue;

      const decoded = decodeURIComponent(path);
      const hasExtension = extname(decoded) !== '';
      const hasSlash = decoded.includes('/');

      // Wiki 页面名链接：无扩展名、无路径分隔符，指向同目录下的 <页面名>.md
      if (isWikiPage && !hasExtension && !hasSlash) {
        if (!existsSync(join(wikiDir, `${decoded}.md`))) {
          errors.push(`${rel(file)}:${lineNo} Wiki 页面链接指向不存在的页面：${decoded}`);
        }
        continue;
      }

      const absolute = resolve(dirname(file), decoded);
      if (!existsSync(absolute)) {
        errors.push(`${rel(file)}:${lineNo} 链接指向不存在的路径：${target}`);
      } else if (trackedFiles !== null && trackedDirs !== null) {
        const isDir = statSync(absolute).isDirectory();
        const tracked = isDir ? trackedDirs.has(absolute) : trackedFiles.has(absolute);
        if (!tracked) {
          errors.push(
            `${rel(file)}:${lineNo} 链接指向未被版本库跟踪的${isDir ? '目录' : '文件'}（克隆后即 404）：${target}`,
          );
        }
      }
    }
  });
}

/** 检查 Wiki 页面结构：H1 标题、导读、相关页面小节。 */
function checkWikiPageStructure(bodyLines, allLines, file) {
  if (dirname(file) !== join(repoRoot, 'docs', 'wiki')) return;
  const name = basename(file, '.md');
  if (name.startsWith('_')) return; // _Sidebar / _Footer 为 GitHub Wiki 特殊页面

  // Home 是 Wiki 落地页：标题由页面自身定位决定，也不要求结尾挂"相关页面"。
  const isHome = name === 'Home';

  // 只统计代码围栏之外的标题：示例里的 # 注释不是标题。
  const h1 = bodyLines.filter((l) => /^#\s+/.test(l.text));
  if (h1.length === 0) {
    errors.push(`${rel(file)} 缺少一级标题（# 标题）`);
  } else if (h1.length > 1) {
    errors.push(`${rel(file)} 出现 ${h1.length} 个一级标题，Wiki 页面应只有一个`);
  } else {
    const title = h1[0].text.replace(/^#\s+/, '').trim();
    // 文件名用连字符代替空格（Wiki 页面名不便含空格），比较时把两者归一：
    // 「INI-模板配置详解.md」的标题写「INI 模板配置详解」视为一致。
    const normalize = (s) => s.toLowerCase().replace(/[-_\s]+/g, ' ').trim();
    if (!isHome && normalize(title) !== normalize(name)) {
      warnings.push(`${rel(file)} 一级标题「${title}」与文件名「${name}」不一致`);
    }
  }

  const text = allLines.map((l) => l.text).join('\n');
  if (!isHome && !/相关页面|延伸阅读|参见/.test(text)) {
    warnings.push(`${rel(file)} 缺少「相关页面」小节，建议在结尾补充 Wiki 页面链接`);
  }
}

// ---------------------------------------------------------------------------
// 主流程
// ---------------------------------------------------------------------------

const files = collectMarkdownFiles();
let mermaidTotal = 0;
let lineTotal = 0;
const perFile = [];

for (const file of files) {
  const raw = readFileSync(file, 'utf8');
  const rawLines = raw.split(/\r?\n/);
  // 以换行结尾的文件，split 会多出一个空元素；去掉它，行数才等于编辑器里的行数。
  if (rawLines.length > 0 && rawLines[rawLines.length - 1] === '') rawLines.pop();
  const lines = rawLines.map((text, index) => ({ text, lineNo: index + 1 }));
  lineTotal += lines.length;

  const { body, mermaidBlocks } = splitFences(lines, file);
  mermaidTotal += mermaidBlocks.length;
  perFile.push({ file: rel(file), lines: lines.length, mermaid: mermaidBlocks.length });

  for (const block of mermaidBlocks) {
    checkMermaidBlock(block, file);
  }
  checkLinks(body, file);
  checkWikiPageStructure(body, lines, file);
}

if (!quiet) {
  console.log('文档自检结果');
  console.log('='.repeat(64));
  for (const item of perFile) {
    console.log(`${item.file.padEnd(48)} ${String(item.lines).padStart(5)} 行  mermaid=${item.mermaid}`);
  }
  console.log('='.repeat(64));
}

console.log(`检查文件 ${files.length} 个，共 ${lineTotal} 行，Mermaid 图 ${mermaidTotal} 张`);

if (warnings.length > 0) {
  console.log(`\n警告 ${warnings.length} 条：`);
  for (const message of warnings) console.log(`  [WARN] ${message}`);
}

if (errors.length > 0) {
  console.log(`\n错误 ${errors.length} 条：`);
  for (const message of errors) console.log(`  [ERROR] ${message}`);
  process.exit(1);
}

console.log('\n未发现错误。');
