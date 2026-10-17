(function () {
  "use strict";
  var SF = window.SF;

  function renderRulesets(map) {
    var box = SF.qs("#rulesetsBox");
    var keys = Object.keys(map || {});
    if (!keys.length) { box.className = "state"; box.textContent = "当前模板未声明 [ruleset] 规则集映射。"; return; }
    var tbl = document.createElement("table");
    tbl.innerHTML = "<thead><tr><th>名称</th><th>形态</th></tr></thead><tbody></tbody>";
    var tb = tbl.querySelector("tbody");
    keys.sort().forEach(function (k) {
      var tr = document.createElement("tr");
      tr.innerHTML = "<td><code>" + SF.esc(k) + "</code></td><td><span class='badge'>" + SF.esc(map[k]) + "</span></td>";
      tb.appendChild(tr);
    });
    box.className = "";
    box.innerHTML = "";
    box.appendChild(tbl);
  }

  document.addEventListener("DOMContentLoaded", function () {
    SF.api("/api/rulesets").then(function (res) {
      renderRulesets(res && res.rulesets ? res.rulesets : res);
    }).catch(function () { SF.qs("#rulesetsBox").textContent = "规则集加载失败"; });

    SF.qs("#reCopy").addEventListener("click", function () { SF.copyText(SF.qs("#reEditor").value, this); });
    SF.qs("#reClear").addEventListener("click", function () { SF.qs("#reEditor").value = ""; });
  });
})();