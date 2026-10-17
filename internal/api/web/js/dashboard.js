(function () {
  "use strict";
  var SF = window.SF;
  var qcOut = SF.qs("#qcOut");
  var qcState = SF.qs("#qcState");
  var qcUrl = SF.qs("#qcUrl");
  var qcTarget = SF.qs("#qcTarget");

  function loadTargets(sel) {
    SF.api("/api/targets").then(function (targets) {
      (targets || []).forEach(function (t) {
        var o = document.createElement("option");
        o.value = t; o.textContent = t;
        sel.appendChild(o);
      });
      if (sel.id === "qcTarget" && !sel.value) {
        // 默认 clash.meta 优先
        var i = Array.prototype.findIndex.call(sel.options, function (o) { return o.value === "clash.meta" || o.value === "clashmeta"; });
        if (i >= 0) sel.selectedIndex = i;
      }
    }).catch(function (e) { showError(e); });
  }

  function loadStats() {
    SF.api("/api/stats").then(function (s) {
      var box = SF.qs("#statsBox");
      box.innerHTML = "";
      var last = s.last || {};
      var cacheRate = (last.cacheHitRate != null) ? (Math.round(last.cacheHitRate * 100) + "%") : "—";
      var items = [
        ["节点数", (last.nodes != null ? last.nodes : "—")],
        ["分组数", (last.groups != null ? last.groups : "—")],
        ["规则数", (last.rules != null ? last.rules : "—")],
        ["缓存命中率", cacheRate],
        ["日志条数", (s.logs || 0)],
      ];
      items.forEach(function (it) {
        var d = document.createElement("div");
        d.className = "stat";
        d.innerHTML = '<div class="k">' + SF.esc(it[0]) + '</div><div class="v">' + SF.esc(it[1]) + '</div>';
        box.appendChild(d);
      });
    }).catch(function () {});
  }

  function loadHistory() {
    SF.api("/api/history").then(function (rows) {
      var box = SF.qs("#historyBox");
      if (!rows || !rows.length) { box.innerHTML = '<div class="state">暂无历史记录</div>'; return; }
      var tbl = document.createElement("table");
      tbl.innerHTML = "<thead><tr><th>时间</th><th>target</th><th>节点</th><th>状态</th></tr></thead><tbody></tbody>";
      var tb = tbl.querySelector("tbody");
      rows.forEach(function (h) {
        var tr = document.createElement("tr");
        var status = (h.failures && h.failures.length) ? '<span class="badge warn">' + h.failures.length + " 失败</span>" : '<span class="badge ok">OK</span>';
        tr.innerHTML = "<td class='mono'>" + SF.esc(SF.fmtTime(h.at)) + "</td>" +
          "<td><span class='badge acc'>" + SF.esc(h.target) + "</span></td>" +
          "<td class='mono'>" + SF.esc(h.nodes) + " / " + SF.esc(h.rawNodes) + "</td>" +
          "<td>" + status + "</td>";
        tb.appendChild(tr);
      });
      box.innerHTML = "";
      box.appendChild(tbl);
    }).catch(function () { SF.qs("#historyBox").innerHTML = '<div class="state err">历史加载失败</div>'; });
  }

  function showError(e) {
    qcState.style.display = "block";
    qcState.className = "state err";
    qcState.textContent = (e && e.message) ? e.message : String(e);
  }

  function runConvert() {
    qcState.style.display = "none";
    var body = {
      target: qcTarget.value,
      urls: qcUrl.value.split(/\n/).map(function (s) { return s.trim(); }).filter(Boolean),
    };
    SF.api("/api/convert", { method: "POST", body: body }).then(function (r) {
      qcOut.value = r.body || "";
      qcState.style.display = "block";
      qcState.className = "state ok";
      qcState.textContent = (r.format || "ok") + " · " + (r.nodes || 0) + " 节点";
      if (r.failures && r.failures.length) qcState.textContent += " · " + r.failures.length + " 个来源失败";
      loadStats(); loadHistory();
    }).catch(showError);
  }

  document.addEventListener("DOMContentLoaded", function () {
    loadTargets(qcTarget);
    loadStats();
    loadHistory();
    SF.qs("#qcRun").addEventListener("click", runConvert);
    SF.qs("#qcCopy").addEventListener("click", function () { SF.copyText(qcOut.value, this); });
    SF.qs("#qcDownload").addEventListener("click", function () { SF.downloadText("result.yaml", qcOut.value, "text/yaml"); });
  });
})();