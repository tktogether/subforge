// SubForge UI · Logs 页：轮询新增日志、按关键字/级别过滤、导出。
(function () {
  "use strict";
  var SF = window.SF;

  var box = null;
  var filterInput = null;
  var lastID = 0;
  var all = [];          // 已加载的全部条目（受 filter 过滤前）
  var timer = null;

  // 从日志行里推断级别：优先 level=xxx，其次常见标记。
  function levelOf(line) {
    var m = /level[=:]\s*(\w+)/i.exec(line);
    if (m) return m[1].toLowerCase();
    if (/\b(error|err|failed|failure|fatal|panic)\b/i.test(line)) return "error";
    if (/\b(warn|warning)\b/i.test(line)) return "warn";
    if (/\b(debug|trace)\b/i.test(line)) return "debug";
    return "info";
  }

  function matches(entry) {
    var kw = (filterInput.value || "").trim().toLowerCase();
    if (!kw) return true;
    // 支持 "level:error" 或 "level=warn" 这类级别过滤
    var lm = /^level[=:]\s*(\w+)$/i.exec(kw);
    if (lm) return levelOf(entry.line) === lm[1].toLowerCase();
    return entry.line.toLowerCase().indexOf(kw) >= 0;
  }

  function render() {
    var items = all.filter(matches);
    if (!items.length) {
      box.className = "state";
      box.textContent = filterInput.value ? "无匹配日志。" : "暂无日志，等待输出。";
      return;
    }
    box.className = "logs";
    box.innerHTML = "";
    items.forEach(function (e) {
      var row = SF.el("div", { class: "log-row lvl-" + levelOf(e.line) });
      var meta = SF.el("span", { class: "log-meta" });
      meta.appendChild(SF.el("span", { class: "log-ts" }, e.ts ? e.ts.replace("T", " ").replace("Z", " UTC") : ""));
      meta.appendChild(SF.el("span", { class: "log-lvl" }, levelOf(e.line).toUpperCase()));
      row.appendChild(meta);
      row.appendChild(SF.el("span", { class: "log-line" }, e.line));
      box.appendChild(row);
    });
  }

  function append(entries) {
    if (!entries || !entries.length) return;
    entries.forEach(function (e) {
      if (e.id > lastID) lastID = e.id;
      all.push(e);
    });
    // 控制内存：最多保留 500 条
    if (all.length > 500) all = all.slice(all.length - 500);
    render();
  }

  function poll() {
    SF.api("/api/logs?since=" + lastID).then(function (res) {
      var entries = (res && res.logs) || [];
      append(entries);
    }).catch(function () { /* 静默：下次轮询再试 */ });
  }

  function exportText() {
    var items = all.filter(matches);
    var text = items.map(function (e) {
      return e.ts + " [" + levelOf(e.line).toUpperCase() + "] " + e.line;
    }).join("\n");
    SF.downloadText("subforge-logs.log", text, "text/plain");
  }

  function exportJson() {
    SF.downloadText("subforge-logs.json", JSON.stringify(all.filter(matches), null, 2), "application/json");
  }

  document.addEventListener("DOMContentLoaded", function () {
    box = SF.qs("#logBox");
    filterInput = SF.qs("#logFilter");
    filterInput.addEventListener("input", render);

    SF.qs("#logExportText").addEventListener("click", exportText);
    SF.qs("#logExportJson").addEventListener("click", exportJson);
    SF.qs("#logClear").addEventListener("click", function () {
      all = []; lastID = 0; render();
    });

    // 首次全量加载（since=0 取最近 200 条），之后增量轮询。
    SF.api("/api/logs?since=0").then(function (res) {
      all = (res && res.logs) || [];
      if (all.length) lastID = all[all.length - 1].id;
      render();
    }).catch(function () {
      box.className = "state"; box.textContent = "日志加载失败。";
    });
    timer = setInterval(poll, 2000);
  });
})();
