(function () {
  "use strict";
  var SF = window.SF;
  var out = SF.qs("#cvOut");
  var state = SF.qs("#cvState");
  var fmt = SF.qs("#cvFormat");
  var nodes = SF.qs("#cvNodes");
  var meta = SF.qs("#cvMeta");

  function splitLines(v) { return (v || "").split(/\n/).map(function (s) { return s.trim(); }).filter(Boolean); }

  function collect() {
    return {
      target: SF.qs("#cvTarget").value,
      urls: splitLines(SF.qs("#cvUrls").value),
      types: splitLines(SF.qs("#cvTypes").value),
      include: splitLines(SF.qs("#cvInclude").value),
      exclude: splitLines(SF.qs("#cvExclude").value),
      pipeline: SF.qs("#cvPipeline").value,
      config: SF.qs("#cvConfig").value,
      sort: SF.qs("#cvSort").value,
      list: SF.qs("#cvList").value === "1",
      override: SF.qs("#cvOverride").value,
      baseline: SF.qs("#cvBaseline").value,
    };
  }

  function showError(e) {
    state.style.display = "block";
    state.className = "state err";
    state.textContent = (e && e.message) ? e.message : String(e);
  }

  function run() {
    state.style.display = "none";
    var fileInput = SF.qs("#cvFile");
    var file = fileInput && fileInput.files && fileInput.files[0];
    var doRun = function (fileText) {
      var body = collect();
      var text = SF.qs("#cvText").value || "";
      if (fileText) text = text ? (text + "\n" + fileText) : fileText;
      if (text) body.text = text;
      SF.api("/api/convert", { method: "POST", body: body }).then(function (r) {
        out.value = r.body || "";
        fmt.textContent = r.format || "—";
        nodes.textContent = (r.nodes || 0) + " / " + (r.rawNodes || 0);
        meta.textContent = r.failures && r.failures.length ? ("来源失败: " + r.failures.join(" | ")) : "全部成功";
        state.style.display = "block";
        state.className = r.failures && r.failures.length ? "state" : "state ok";
        if (!r.failures || !r.failures.length) state.textContent = "转换完成";
      }).catch(showError);
    };
    if (file) {
      var reader = new FileReader();
      reader.onload = function (e) { doRun(e.target.result); };
      reader.onerror = function () { showError(new Error("文件读取失败")); };
      reader.readAsText(file);
    } else {
      doRun("");
    }
  }

  function renderDiff(lines) {
    var box = SF.qs("#cvDiffOut");
    box.innerHTML = "";
    if (!lines || !lines.length) {
      box.innerHTML = '<div class="ln"><span class="sign"></span><span class="muted">无差异</span></div>';
      return;
    }
    lines.forEach(function (l) {
      var cls = l.type === "+" ? "add" : l.type === "-" ? "del" : "";
      var d = document.createElement("div");
      d.className = "ln " + cls;
      d.innerHTML = '<span class="sign">' + SF.esc(l.type) + '</span><span>' + SF.esc(l.text) + '</span>';
      box.appendChild(d);
    });
    SF.qs("#diffPanel").setAttribute("open", "open");
  }

  document.addEventListener("DOMContentLoaded", function () {
    SF.api("/api/targets").then(function (targets) {
      var sel = SF.qs("#cvTarget");
      (targets || []).forEach(function (t) {
        var o = document.createElement("option"); o.value = t; o.textContent = t; sel.appendChild(o);
      });
    }).catch(function () {});
    SF.api("/api/pipelines").then(function (names) {
      var sel = SF.qs("#cvPipeline");
      var def = document.createElement("option"); def.value = ""; def.textContent = "（默认管线）"; sel.appendChild(def);
      (names || []).forEach(function (n) {
        var o = document.createElement("option"); o.value = n; o.textContent = n; sel.appendChild(o);
      });
    }).catch(function () {});

    SF.qs("#cvRun").addEventListener("click", run);
    SF.qs("#cvCopy").addEventListener("click", function () { SF.copyText(out.value, this); });
    SF.qs("#cvDownload").addEventListener("click", function () { SF.downloadText("result.yaml", out.value, "text/yaml"); });
    SF.qs("#cvDiff").addEventListener("click", function () {
      SF.api("/api/diff", { method: "POST", body: { a: SF.qs("#cvBaseline").value, b: out.value } })
        .then(renderDiff).catch(showError);
    });
  });
})();