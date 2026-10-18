// subscription.js — 订阅管理页面逻辑（对标 sub-poly / Sub-Store 的自建订阅面）。
// 依赖 app.js 暴露的 window.SF：token 注入、统一错误、转义与 DOM 工具。
// 设计约束（.impeccable.md）：渐进披露、不用模态、明示 loading/error/空状态。
(function () {
  "use strict";

  var state = {
    airports: [], groups: [], rules: [], tokens: [],
    loaded: { airports: false, groups: false, rules: false, tokens: false },
    confirmTimer: {}
  };

  var SUB_TARGETS = ["clash.meta", "clash", "sing-box", "v2ray", "base64", "mixed"];

  // 端点集中声明成字面量：静态契约测试会扫描脚本里的路径，动态拼接会整页漏扫。
  var API = {
    airports: "/api/airports",
    groups: "/api/groups",
    rules: "/api/rules",
    tokens: "/api/tokens"
  };

  function $(id) { return document.getElementById(id); }

  // --- 状态展示 -------------------------------------------------------------

  function setState(kind, text, cls) {
    var b = $("tab-" + kind);
    if (b) b = b.querySelector(".list-box");
    if (!b) return;
    b.className = cls ? "list-box state " + cls : "list-box state";
    b.textContent = text;
  }

  function listBox(kind) {
    var b = $("tab-" + kind);
    return b ? b.querySelector(".list-box") : null;
  }

  function paint(kind, node) {
    var b = listBox(kind);
    if (!b) return;
    b.className = "list-box";
    b.textContent = "";
    b.appendChild(node);
  }

  function empty(title, hint, actionLabel, action) {
    var wrap = SF.el("div", { class: "empty" });
    wrap.appendChild(SF.el("p", { class: "empty-title" }, title));
    wrap.appendChild(SF.el("p", { class: "muted" }, hint));
    if (actionLabel && action) {
      var btn = SF.el("button", { class: "btn primary" }, actionLabel);
      btn.addEventListener("click", action);
      wrap.appendChild(btn);
    }
    return wrap;
  }

  function setBusy(btn, busy) { if (btn) btn.disabled = !!busy; }

  // --- 加载 -----------------------------------------------------------------

  function load(kind, force) {
    if (state.loaded[kind] && !force) return Promise.resolve();
    setState(kind, "加载中…");
    return SF.api(API[kind]).then(function (data) {
      state.loaded[kind] = true;
      state[kind] = (data && data[kind]) || [];
      render(kind);
      if (kind === "tokens") renderSubscribe();
      if (kind === "groups" && state.loaded.rules) renderRules();
    }, function (err) {
      state.loaded[kind] = false;
      setState(kind, "加载失败：" + err.message, "err");
      var b = listBox(kind);
      if (b) {
        var retry = SF.el("button", { class: "btn" }, "重试");
        retry.style.marginTop = "8px";
        retry.addEventListener("click", function () { load(kind, true); });
        b.appendChild(retry);
      }
      if (kind === "tokens") renderSubscribe();
    });
  }

  function reload(kind) { return load(kind, true); }

  function render(kind) {
    if (kind === "airports") renderAirports();
    else if (kind === "groups") renderGroups();
    else if (kind === "rules") renderRules();
    else renderTokens();
  }

  // --- 表格小工具 -----------------------------------------------------------

  function td(text, cls) { return SF.el("td", cls ? { class: cls } : null, text == null ? "" : String(text)); }

  function row(cells) {
    var tr = SF.el("tr");
    cells.forEach(function (c) { tr.appendChild(c); });
    return tr;
  }

  function table(headers, rows) {
    var t = SF.el("table");
    var thead = SF.el("thead");
    var htr = SF.el("tr");
    headers.forEach(function (h) { htr.appendChild(SF.el("th", null, h)); });
    thead.appendChild(htr);
    t.appendChild(thead);
    var tb = SF.el("tbody");
    rows.forEach(function (r) { tb.appendChild(r); });
    t.appendChild(tb);
    return t;
  }

  function actions(kind, id) {
    var cell = SF.el("td", { class: "actions" });
    var edit = SF.el("button", { class: "btn" }, "编辑");
    var del = SF.el("button", { class: "btn danger" }, "删除");
    edit.addEventListener("click", function () { openEditor(kind, id); });
    del.addEventListener("click", function () { confirmDelete(kind, id, del); });
    cell.appendChild(edit);
    cell.appendChild(del);
    return cell;
  }

  // --- 列表渲染 -------------------------------------------------------------

  function renderAirports() {
    if (!state.airports.length) {
      paint("airports", empty("还没有订阅源",
        "订阅源是你自己的机场/上游链接。SubForge 只保存链接，拉取与解析在你每次请求订阅链接时实时发生。",
        "添加第一个订阅", function () { openEditor("airports", null); }));
      return;
    }
    var rows = state.airports.map(function (a) {
      return row([
        td(a.id, "mono"), td(a.name),
        td(a.subscription_url, "mono url"), td(a.remarks || ""),
        td(a.enabled ? "启用" : "停用", a.enabled ? "badge ok" : "badge"),
        actions("airports", a.id)
      ]);
    });
    paint("airports", table(["ID", "名称", "订阅 URL", "备注", "状态", "操作"], rows));
  }

  function renderGroups() {
    if (!state.groups.length) {
      paint("groups", empty("还没有代理分组",
        "分组决定节点怎么归拢（regex 匹配节点名）。没有分组时，生成的配置只会带最小可用默认组。",
        "新建分组", function () { openEditor("groups", null); }));
      return;
    }
    var rows = state.groups.map(function (g) {
      return row([
        td(g.id, "mono"), td(g.name), td(g.type, "mono"),
        td(g.regex || "", "mono"), td(g.url || "", "mono url"),
        td(g.interval || 0, "mono"), actions("groups", g.id)
      ]);
    });
    paint("groups", table(["ID", "名称", "类型", "Regex", "测活 URL", "间隔(s)", "操作"], rows));
  }

  function policyName(id) {
    if (!id) return "DIRECT";
    if (Number(id) === -1) return "REJECT";
    var hit = null;
    state.groups.forEach(function (g) { if (Number(g.id) === Number(id)) hit = g; });
    return hit ? hit.name : "DIRECT (悬空 id=" + id + ")";
  }

  function renderRules() {
    if (!state.rules.length) {
      paint("rules", empty("还没有规则",
        "规则按顺序生成分流规则；策略列指向一个代理分组，指向空则落到 DIRECT。",
        "新建规则", function () { openEditor("rules", null); }));
      return;
    }
    var rows = state.rules.map(function (r) {
      return row([
        td(r.id, "mono"), td(r.rule_type, "mono"), td(r.rule_param, "mono url"),
        td(policyName(r.rule_config), "mono"),
        td(r.resolve_dns ? "no-resolve" : "", "mono"),
        td(r.enabled ? "启用" : "停用", r.enabled ? "badge ok" : "badge"),
        actions("rules", r.id)
      ]);
    });
    paint("rules", table(["ID", "类型", "参数", "策略分组", "DNS", "状态", "操作"], rows));
  }

  function renderTokens() {
    if (!state.tokens.length) {
      paint("tokens", empty("还没有订阅令牌",
        "令牌是订阅链接的凭据，一个客户端一个。生成后 /subscribe?token=… 就是你的订阅地址，内容永远跟随当前配置。",
        "生成令牌", function () { openEditor("tokens", null); }));
      return;
    }
    var rows = state.tokens.map(function (t) {
      var cell = SF.el("td", { class: "actions" });
      var copy = SF.el("button", { class: "btn" }, "订阅链接");
      copy.addEventListener("click", function () { showLink(t.token, copy); });
      var del = SF.el("button", { class: "btn danger" }, "删除");
      del.addEventListener("click", function () { confirmDelete("tokens", t.token, del); });
      cell.appendChild(copy);
      cell.appendChild(del);
      return row([td(t.token, "mono token"), td(t.name || ""), td(SF.fmtTime(t.created_at)), cell]);
    });
    paint("tokens", table(["Token", "名称", "创建时间", "操作"], rows));
  }

  // --- 内联编辑器（不使用 prompt / 模态） ------------------------------------

  function findById(list, id) {
    if (id == null) return null;
    var hit = null;
    list.forEach(function (x) {
      if (String(x.id) === String(id) || String(x.token) === String(id)) hit = x;
    });
    return hit;
  }

  function field(labelText, node) {
    var wrap = SF.el("label", { class: "field" });
    wrap.appendChild(SF.el("span", { class: "field-label" }, labelText));
    wrap.appendChild(node);
    return wrap;
  }

  function textInput(value, placeholder, cls) {
    var a = { type: "text", value: value == null ? "" : String(value) };
    if (placeholder) a.placeholder = placeholder;
    if (cls) a.class = cls;
    return SF.el("input", a);
  }

  function urlInput(value, placeholder) {
    return SF.el("input", { type: "url", value: value == null ? "" : String(value), placeholder: placeholder || "", class: "mono" });
  }

  function numInput(value) {
    return SF.el("input", { type: "number", value: value == null ? "" : String(value) });
  }

  function check(labelText, checked) {
    var wrap = SF.el("label", { class: "check" });
    var c = SF.el("input", { type: "checkbox" });
    c.checked = !!checked;
    wrap.appendChild(c);
    wrap.appendChild(SF.el("span", null, labelText));
    wrap._input = c;
    return wrap;
  }

  function select(options, value) {
    var s = SF.el("select");
    options.forEach(function (opt) {
      var o = SF.el("option", { value: String(opt.value) }, opt.label);
      if (String(opt.value) === String(value)) o.selected = true;
      s.appendChild(o);
    });
    return s;
  }

  function closeEditor(kind) {
    var slot = $(kind + "Editor");
    if (slot) { slot.textContent = ""; slot.classList.remove("open"); }
  }

  function openEditor(kind, id) {
    var slot = $(kind + "Editor");
    if (!slot) return;
    if (kind === "rules" && !state.loaded.groups) load("groups", true);
    slot.textContent = "";
    slot.classList.add("open");
    var form = SF.el("form", { class: "editor" });
    form.appendChild(SF.el("h3", null, id == null ? (kind === "tokens" ? "生成订阅令牌" : "新增") : "编辑 #" + id));
    var msg = SF.el("p", { class: "editor-msg" });
    var submit = null;

    if (kind === "airports") {
      var a0 = findById(state.airports, id) || {};
      var an = textInput(a0.name, "例：主机场");
      var au = urlInput(a0.subscription_url, "上游订阅链接");
      var ar = textInput(a0.remarks, "备注（可选）");
      var ae = check("参与生成配置", a0.enabled == null ? true : a0.enabled);
      form.appendChild(field("名称", an));
      form.appendChild(field("订阅 URL", au));
      form.appendChild(field("备注", ar));
      form.appendChild(field("启用", ae));
      submit = function () {
        var body = { name: an.value.trim(), subscription_url: au.value.trim(), remarks: ar.value.trim(), enabled: ae._input.checked };
        if (!body.name || !body.subscription_url) { msg.textContent = "名称和订阅 URL 不能为空"; return null; }
        return save("airports", id, body, msg);
      };
    } else if (kind === "groups") {
      var g0 = findById(state.groups, id) || {};
      var gn = textInput(g0.name, "例：香港");
      var gt = select([{ value: "select", label: "select" }, { value: "url-test", label: "url-test" }, { value: "fallback", label: "fallback" }, { value: "relay", label: "relay" }], g0.type || "select");
      var gr = textInput(g0.regex, "节点名正则，例：(?i)香港|HK", "mono");
      var gu = urlInput(g0.url, "测活地址");
      var gi = numInput(g0.interval || 300);
      form.appendChild(field("名称", gn));
      form.appendChild(field("类型", gt));
      form.appendChild(field("Regex", gr));
      form.appendChild(field("测活 URL", gu));
      form.appendChild(field("测活间隔（秒）", gi));
      submit = function () {
        var body = { name: gn.value.trim(), type: gt.value, regex: gr.value.trim(), url: gu.value.trim(), interval: parseInt(gi.value || "0", 10) || 0 };
        if (!body.name) { msg.textContent = "分组名不能为空"; return null; }
        return save("groups", id, body, msg);
      };
    } else if (kind === "rules") {
      var r0 = findById(state.rules, id) || {};
      var rt = textInput(r0.rule_type, "DOMAIN-SUFFIX / IP-CIDR / GEOIP / RULE-SET", "mono");
      var rp = textInput(r0.rule_param, "google.com / CN / …", "mono");
      var opts = [{ value: "0", label: "DIRECT" }, { value: "-1", label: "REJECT" }];
      state.groups.forEach(function (g) { opts.push({ value: String(g.id), label: g.name + "（" + g.type + "）" }); });
      var rpol = select(opts, r0.rule_config == null ? "0" : String(r0.rule_config));
      var rd = check("跳过 DNS 解析（no-resolve）", r0.resolve_dns);
      var re = check("启用", r0.enabled == null ? true : r0.enabled);
      form.appendChild(field("规则类型", rt));
      form.appendChild(field("参数", rp));
      form.appendChild(field("策略分组", rpol));
      form.appendChild(field("DNS", rd));
      form.appendChild(field("状态", re));
      submit = function () {
        var body = { rule_type: rt.value.trim(), rule_param: rp.value.trim(), rule_config: parseInt(rpol.value, 10) || 0, resolve_dns: rd._input.checked, enabled: re._input.checked };
        if (!body.rule_type || !body.rule_param) { msg.textContent = "规则类型和参数不能为空"; return null; }
        return save("rules", id, body, msg);
      };
    } else {
      var tn = textInput("", "例：iPhone / 笔记本");
      form.appendChild(field("令牌名称", tn));
      form.appendChild(SF.el("p", { class: "muted" }, "提交后由服务端生成随机令牌，页面会直接给出订阅链接。"));
      submit = function () { return save("tokens", null, { name: tn.value.trim() || "未命名" }, msg); };
    }

    var bar = SF.el("div", { class: "editor-bar" });
    var ok = SF.el("button", { class: "btn primary", type: "submit" }, "保存");
    var cancel = SF.el("button", { class: "btn ghost", type: "button" }, "取消");
    cancel.addEventListener("click", function () { closeEditor(kind); });
    bar.appendChild(ok);
    bar.appendChild(cancel);
    form.appendChild(bar);
    form.appendChild(msg);
    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      var p = submit();
      if (!p) return;
      setBusy(ok, true);
      msg.className = "editor-msg";
      msg.textContent = "提交中…";
      p.then(function () { setBusy(ok, false); }, function () { setBusy(ok, false); });
    });
    slot.appendChild(form);
    var first = form.querySelector("input, select");
    if (first) first.focus();
  }

  function save(kind, id, body, msg) {
    var path = API[kind] + (id == null ? "" : "/" + encodeURIComponent(id));
    var method = id == null ? "POST" : "PUT";
    if (kind === "tokens") { path = API.tokens; method = "POST"; }
    return SF.api(path, { method: method, body: body }).then(function (data) {
      closeEditor(kind);
      if (kind === "tokens" && data && data.token && data.token.token) {
        return reload("tokens").then(function () { showLink(data.token.token, null); });
      }
      return reload(kind);
    }, function (err) {
      msg.className = "editor-msg err-text";
      msg.textContent = "保存失败：" + err.message;
      throw err;
    });
  }

  // --- 两步确认删除（无模态） ------------------------------------------------

  function confirmDelete(kind, id, btn) {
    var key = kind + ":" + id;
    if (state.confirmTimer[key]) {
      clearTimeout(state.confirmTimer[key]);
      delete state.confirmTimer[key];
      remove(kind, id, btn);
      return;
    }
    btn.textContent = "确认删除？";
    state.confirmTimer[key] = setTimeout(function () {
      delete state.confirmTimer[key];
      btn.textContent = "删除";
    }, 4000);
  }

  function remove(kind, id, btn) {
    setBusy(btn, true);
    return SF.api(API[kind] + "/" + encodeURIComponent(id), { method: "DELETE" }).then(function () {
      return reload(kind);
    }, function (err) {
      setBusy(btn, false);
      setState(kind, "删除失败：" + err.message, "err");
    });
  }

  // --- 订阅链接与预览 --------------------------------------------------------

  function currentToken() {
    var sel = $("subToken");
    if (sel && sel.value) return sel.value;
    return state.tokens.length ? state.tokens[0].token : "";
  }

  function currentTarget() {
    var sel = $("subTarget");
    return sel && sel.value ? sel.value : "clash.meta";
  }

  function subscribeURL(token) {
    var t = token || currentToken();
    if (!t) return "";
    return location.origin + "/subscribe?token=" + encodeURIComponent(t) + "&target=" + encodeURIComponent(currentTarget());
  }

  function renderSubscribe() {
    var sel = $("subToken");
    if (!sel) return;
    var keep = sel.value;
    sel.textContent = "";
    if (!state.tokens.length) {
      sel.appendChild(SF.el("option", { value: "" }, "（先在「令牌」里生成一个）"));
    } else {
      state.tokens.forEach(function (t) {
        sel.appendChild(SF.el("option", { value: t.token }, t.name || t.token.slice(0, 10) + "…"));
      });
      if (keep) sel.value = keep;
      if (!sel.value) sel.value = state.tokens[0].token;
    }
    var out = $("subURL");
    if (out) out.textContent = subscribeURL() || "—";
  }

  function showLink(token, btn) {
    var url = subscribeURL(token);
    if (!url) return;
    var s = $("subToken");
    if (s) s.value = token;
    var out = $("subURL");
    if (out) out.textContent = url;
    var panel = $("subPanel");
    if (panel && panel.scrollIntoView) panel.scrollIntoView({ block: "nearest" });
    if (btn) SF.copyText(url, btn);
  }

  function preview() {
    var url = subscribeURL();
    var out = $("subPreview");
    var status = $("subStatus");
    var btn = $("subPreview");
    if (!url) { status.className = "badge err"; status.textContent = "先生成一个订阅令牌"; return; }
    status.className = "muted";
    status.textContent = "正在按当前数据库状态编译 " + currentTarget() + " …";
    setBusy(btn, true);
    return fetch(url, { headers: { Accept: "text/plain,*/*" } }).then(function (res) {
      return res.text().then(function (text) {
        setBusy(btn, false);
        out.textContent = text;
        if (res.ok) {
          status.className = "badge ok";
          status.textContent = "HTTP " + res.status + " · " + text.length + " 字节 · " + text.split("\n").length + " 行";
        } else {
          status.className = "badge err";
          status.textContent = "HTTP " + res.status + "：" + text.slice(0, 200);
        }
      });
    }, function (err) {
      setBusy(btn, false);
      status.className = "badge err";
      status.textContent = "拉取失败：" + err.message;
    });
  }

  // --- 绑定与启动 ------------------------------------------------------------

  function switchTab(tab) {
    SF.qsa(".tab").forEach(function (b) {
      b.classList.toggle("active", b.getAttribute("data-tab") === tab);
    });
    SF.qsa(".tab-panel").forEach(function (p) {
      p.classList.toggle("active", p.id === "tab-" + tab);
    });
    load(tab);
  }

  function bindTabs() {
    SF.qsa(".tab").forEach(function (btn) {
      btn.addEventListener("click", function () { switchTab(btn.getAttribute("data-tab")); });
    });
  }

  function bindAdds() {
    $("airportAdd").addEventListener("click", function () { openEditor("airports", null); });
    $("groupAdd").addEventListener("click", function () { openEditor("groups", null); });
    $("ruleAdd").addEventListener("click", function () { openEditor("rules", null); });
    $("tokenAdd").addEventListener("click", function () { openEditor("tokens", null); });
  }

  function bindSubscribePanel() {
    var target = $("subTarget");
    SUB_TARGETS.forEach(function (t) { target.appendChild(SF.el("option", { value: t }, t)); });
    target.value = "clash.meta";
    target.addEventListener("change", renderSubscribe);
    var sel = $("subToken");
    sel.addEventListener("change", renderSubscribe);
    $("subCopy").addEventListener("click", function () {
      var url = subscribeURL();
      if (url) SF.copyText(url, $("subCopy"));
    });
    $("subOpen").addEventListener("click", function () {
      var url = subscribeURL();
      if (url) window.open(url, "_blank", "noopener");
    });
    $("subDownload").addEventListener("click", function () {
      var url = subscribeURL();
      var out = $("subPreview");
      if (!url) return;
      if (out && out.textContent) {
        SF.downloadText("subforge-" + currentTarget() + ".yaml", out.textContent);
        return;
      }
      fetch(url).then(function (r) { return r.text(); }).then(function (t) { SF.downloadText("subforge-" + currentTarget() + ".yaml", t); });
    });
    $("subPreview").addEventListener("click", preview);
  }

  function init() {
    bindTabs();
    bindAdds();
    bindSubscribePanel();
    load("tokens", true);
    load("airports", true);
    load("groups", true);
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
  else init();
})();