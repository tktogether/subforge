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

  function groupOf(rule) {
    var parts = String(rule).split(",");
    if (parts.length >= 3) {
      var g = parts[parts.length - 1].trim();
      if (g) return g;
    }
    return "(未指定)";
  }

  function renderRuleTree(rules) {
    var box = SF.qs("#rulesTreeBox");
    if (!rules || !rules.length) { box.className = "state"; box.textContent = "当前模板未声明规则。"; return; }
    var groups = {};
    var order = [];
    rules.forEach(function (r) {
      var g = groupOf(r);
      if (!groups[g]) { groups[g] = []; order.push(g); }
      groups[g].push(r);
    });
    box.className = "";
    box.innerHTML = "";
    order.forEach(function (g) {
      var h = document.createElement("div");
      h.className = "muted";
      h.style.fontWeight = "600";
      h.style.marginTop = "8px";
      h.textContent = g + " (" + groups[g].length + ")";
      box.appendChild(h);
      var ul = document.createElement("ul");
      ul.style.margin = "4px 0 8px 16px";
      ul.style.padding = "0";
      groups[g].forEach(function (r) {
        var li = document.createElement("li");
        li.style.fontSize = "12.5px";
        li.style.fontFamily = "var(--font-mono)";
        li.textContent = r;
        ul.appendChild(li);
      });
      box.appendChild(ul);
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    SF.api("/api/rulesets").then(function (res) {
      renderRulesets(res && res.rulesets ? res.rulesets : res);
      renderRuleTree(res && res.rules ? res.rules : []);
    }).catch(function () { SF.qs("#rulesetsBox").textContent = "规则集加载失败"; });
    SF.qs("#reCopy").addEventListener("click", function () { SF.copyText(SF.qs("#reEditor").value, this); });
    SF.qs("#reClear").addEventListener("click", function () { SF.qs("#reEditor").value = ""; });
  });
})();