// SubForge UI 共享运行时：主题、API 访问、DOM 小工具。
// 纯原生 JS，无构建。页面脚本在 app.js 之后加载，共用 SF.* 工具。
(function () {
  "use strict";

  var TOKEN_KEY = "subforge-token";
  var THEME_KEY = "subforge-theme";

  // --- 主题：优先 localStorage，其次系统偏好 ---
  function applyTheme(theme) {
    if (theme === "light" || theme === "dark") {
      document.documentElement.setAttribute("data-theme", theme);
    } else {
      document.documentElement.removeAttribute("data-theme");
    }
  }

  function currentTheme() {
    return localStorage.getItem(THEME_KEY) ||
      (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  }

  function toggleTheme() {
    var next = currentTheme() === "dark" ? "light" : "dark";
    localStorage.setItem(THEME_KEY, next);
    applyTheme(next);
    renderToggle();
  }

  function renderToggle() {
    var btn = document.getElementById("themeToggle");
    if (!btn) return;
    btn.textContent = currentTheme() === "dark" ? "☾" : "☀";
    btn.setAttribute("aria-label", currentTheme() === "dark" ? "切换到浅色" : "切换到深色");
    btn.title = btn.getAttribute("aria-label");
  }

  // --- 令牌：仅本机自用，存在浏览器本地；随请求带上 ---
  function token() {
    return localStorage.getItem(TOKEN_KEY) || "";
  }

  function rememberTokenFromURL() {
    var t = new URLSearchParams(location.search).get("token");
    if (t) localStorage.setItem(TOKEN_KEY, t);
  }

  // --- API 访问 ---
  function qs(sel, root) {
    return (root || document).querySelector(sel);
  }

  function qsa(sel, root) {
    return Array.prototype.slice.call((root || document).querySelectorAll(sel));
  }

  function esc(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }

  function qsFromForm(fields) {
    var out = new URLSearchParams();
    Object.keys(fields).forEach(function (k) {
      var v = fields[k];
      if (Array.isArray(v)) v.forEach(function (x) { if (x !== "") out.append(k, x); });
      else if (v !== "" && v != null) out.set(k, v);
    });
    return out;
  }

  function api(path, opts) {
    opts = opts || {};
    var url = path;
    if (token()) url += (url.indexOf("?") >= 0 ? "&" : "?") + "token=" + encodeURIComponent(token());
    var init = { method: opts.method || "GET", headers: {} };
    if (opts.body != null) {
      init.body = typeof opts.body === "string" ? opts.body : JSON.stringify(opts.body);
      init.headers["Content-Type"] = "application/json";
    }
    return fetch(url, init).then(function (res) {
      var ct = res.headers.get("Content-Type") || "";
      if (!res.ok) {
        var msg = res.status + " " + res.statusText;
        return res.text().then(function (t) {
          var detail = t;
          try { detail = JSON.parse(t).error || t; } catch (e) { /* 保留文本 */ }
          var err = new Error(detail || msg);
          err.status = res.status;
          throw err;
        });
      }
      return ct.indexOf("application/json") >= 0 ? res.json() : res.text();
    });
  }

  function downloadText(name, text, mime) {
    var blob = new Blob([text], { type: (mime || "text/plain") + "; charset=utf-8" });
    var a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = name;
    document.body.appendChild(a);
    a.click();
    setTimeout(function () { URL.revokeObjectURL(a.href); a.remove(); }, 100);
  }

  function copyText(text, btn) {
    var done = function () {
      if (!btn) return;
      var old = btn.textContent;
      btn.textContent = "已复制";
      setTimeout(function () { btn.textContent = old; }, 1200);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, function () { fallback(); });
    } else fallback();
    function fallback() {
      var ta = document.createElement("textarea");
      ta.value = text; document.body.appendChild(ta); ta.select();
      try { document.execCommand("copy"); } catch (e) { /* 忽略 */ }
      ta.remove(); done();
    }
  }

  function fmtTime(iso) {
    if (!iso) return "—";
    var d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    return d.toLocaleString();
  }

  function setActiveNav() {
    var path = location.pathname;
    qsa(".sidebar-nav a").forEach(function (a) {
      var href = a.getAttribute("href");
      if (href === path || (path === "/" && href === "/")) a.setAttribute("aria-current", "page");
      else a.removeAttribute("aria-current");
    });
  }

  function markPage(root) {
    qsa("[data-page]", root).forEach(function (el) {
      var p = el.getAttribute("data-page");
      if (p === location.pathname) el.setAttribute("aria-current", "page");
    });
  }

  function el(tag, attrs, text) {
    var node = document.createElement(tag);
    if (attrs) Object.keys(attrs).forEach(function (k) {
      if (k === "class") node.className = attrs[k];
      else if (k === "html") node.innerHTML = attrs[k];
      else node.setAttribute(k, attrs[k]);
    });
    if (text != null) node.textContent = text;
    return node;
  }

  document.addEventListener("DOMContentLoaded", function () {
    renderToggle();
    setActiveNav();
    var btn = document.getElementById("themeToggle");
    if (btn) btn.addEventListener("click", toggleTheme);
    var tokenInput = document.getElementById("tokenInput");
    if (tokenInput) {
      tokenInput.value = token();
      tokenInput.addEventListener("change", function () {
        if (tokenInput.value) localStorage.setItem(TOKEN_KEY, tokenInput.value);
        else localStorage.removeItem(TOKEN_KEY);
      });
    }
  });

  // 供各页面脚本使用
  window.SF = {
    token: token,
    rememberTokenFromURL: rememberTokenFromURL,
    api: api,
    qs: qs,
    qsa: qsa,
    esc: esc,
    qsFromForm: qsFromForm,
    downloadText: downloadText,
    copyText: copyText,
    fmtTime: fmtTime,
    markPage: markPage,
    el: el,
  };

  applyTheme(currentTheme());
  rememberTokenFromURL();
})();