// subscription.js — 订阅管理页面逻辑
(function() {
  'use strict';

  // Tab switching
  document.querySelectorAll('.tab').forEach(function(btn) {
    btn.addEventListener('click', function() {
      document.querySelectorAll('.tab').forEach(function(b) { b.classList.remove('active'); });
      document.querySelectorAll('.tab-panel').forEach(function(p) { p.classList.remove('active'); });
      btn.classList.add('active');
      document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
    });
  });

  // API helper
  function api(method, path, body) {
    return fetch(path, {
      method: method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined
    }).then(function(res) {
      return res.json().then(function(data) {
        return { ok: res.ok, status: res.status, data: data };
      });
    });
  }

  // Load airports
  function loadAirports() {
    api('GET', '/api/airports').then(function(r) {
      var box = document.getElementById('airportsBox');
      if (!r.ok) { box.textContent = '加载失败: ' + (r.data && r.data.error || r.status); return; }
      var airports = r.data.airports || [];
      if (airports.length === 0) { box.innerHTML = '<p class="muted">暂无订阅，点击新增添加。</p>'; return; }
      var html = '<table><thead><tr><th>ID</th><th>名称</th><th>URL</th><th>备注</th><th>启用</th><th>操作</th></tr></thead><tbody>';
      airports.forEach(function(a) {
        html += '<tr>';
        html += '<td>' + a.id + '</td>';
        html += '<td>' + esc(a.name) + '</td>';
        html += '<td class="mono">' + esc(a.subscription_url) + '</td>';
        html += '<td>' + esc(a.remarks || '') + '</td>';
        html += '<td>' + (a.enabled ? '是' : '否') + '</td>';
        html += '<td><button class="btn" onclick="editAirport(' + a.id + ')">编辑</button> <button class="btn danger" onclick="deleteAirport(' + a.id + ')">删除</button></td>';
        html += '</tr>';
      });
      html += '</tbody></table>';
      box.innerHTML = html;
    });
  }

  // Load groups
  function loadGroups() {
    api('GET', '/api/groups').then(function(r) {
      var box = document.getElementById('groupsBox');
      if (!r.ok) { box.textContent = '加载失败: ' + (r.data && r.data.error || r.status); return; }
      var groups = r.data.groups || [];
      if (groups.length === 0) { box.innerHTML = '<p class="muted">暂无分组，点击新增添加。</p>'; return; }
      var html = '<table><thead><tr><th>ID</th><th>名称</th><th>类型</th><th>Regex</th><th>URL</th><th>Interval</th><th>操作</th></tr></thead><tbody>';
      groups.forEach(function(g) {
        html += '<tr>';
        html += '<td>' + g.id + '</td>';
        html += '<td>' + esc(g.name) + '</td>';
        html += '<td>' + esc(g.type) + '</td>';
        html += '<td class="mono">' + esc(g.regex || '') + '</td>';
        html += '<td class="mono">' + esc(g.url || '') + '</td>';
        html += '<td>' + (g.interval || 0) + '</td>';
        html += '<td><button class="btn" onclick="editGroup(' + g.id + ')">编辑</button> <button class="btn danger" onclick="deleteGroup(' + g.id + ')">删除</button></td>';
        html += '</tr>';
      });
      html += '</tbody></table>';
      box.innerHTML = html;
    });
  }

  // Load rules
  function loadRules() {
    api('GET', '/api/rules').then(function(r) {
      var box = document.getElementById('rulesBox');
      if (!r.ok) { box.textContent = '加载失败: ' + (r.data && r.data.error || r.status); return; }
      var rules = r.data.rules || [];
      if (rules.length === 0) { box.innerHTML = '<p class="muted">暂无规则，点击新增添加。</p>'; return; }
      var html = '<table><thead><tr><th>ID</th><th>类型</th><th>参数</th><th>配置</th><th>操作</th></tr></thead><tbody>';
      rules.forEach(function(r) {
        html += '<tr>';
        html += '<td>' + r.id + '</td>';
        html += '<td>' + esc(r.type) + '</td>';
        html += '<td class="mono">' + esc(r.param) + '</td>';
        html += '<td class="mono">' + esc(r.config || '') + '</td>';
        html += '<td><button class="btn" onclick="editRule(' + r.id + ')">编辑</button> <button class="btn danger" onclick="deleteRule(' + r.id + ')">删除</button></td>';
        html += '</tr>';
      });
      html += '</tbody></table>';
      box.innerHTML = html;
    });
  }

  // Load tokens
  function loadTokens() {
    api('GET', '/api/tokens').then(function(r) {
      var box = document.getElementById('tokensBox');
      if (!r.ok) { box.textContent = '加载失败: ' + (r.data && r.data.error || r.status); return; }
      var tokens = r.data.tokens || [];
      if (tokens.length === 0) { box.innerHTML = '<p class="muted">暂无令牌，点击生成添加。</p>'; return; }
      var html = '<table><thead><tr><th>Token</th><th>名称</th><th>创建时间</th><th>操作</th></tr></thead><tbody>';
      tokens.forEach(function(t) {
        html += '<tr>';
        html += '<td class="mono">' + esc(t.token) + '</td>';
        html += '<td>' + esc(t.name || '') + '</td>';
        html += '<td>' + esc(t.created_at || '') + '</td>';
        html += '<td><button class="btn danger" onclick="deleteToken(\'' + esc(t.token) + '\')">删除</button></td>';
        html += '</tr>';
      });
      html += '</tbody></table>';
      box.innerHTML = html;
    });
  }

  // Escape HTML
  function esc(s) {
    if (!s) return '';
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // Delete handlers
  window.deleteAirport = function(id) {
    if (!confirm('确认删除订阅 ' + id + '?')) return;
    api('DELETE', '/api/airports/' + id).then(loadAirports);
  };
  window.deleteGroup = function(id) {
    if (!confirm('确认删除分组 ' + id + '?')) return;
    api('DELETE', '/api/groups/' + id).then(loadGroups);
  };
  window.deleteRule = function(id) {
    if (!confirm('确认删除规则 ' + id + '?')) return;
    api('DELETE', '/api/rules/' + id).then(loadRules);
  };
  window.deleteToken = function(token) {
    if (!confirm('确认删除令牌?')) return;
    api('DELETE', '/api/tokens/' + token).then(loadTokens);
  };

  // Add handlers (simplified - prompt based)
  document.getElementById('airportAdd').addEventListener('click', function() {
    var name = prompt('订阅名称:');
    if (!name) return;
    var url = prompt('订阅 URL:');
    if (!url) return;
    var remarks = prompt('备注:') || '';
    api('POST', '/api/airports', { name: name, subscription_url: url, remarks: remarks, enabled: true }).then(loadAirports);
  });
  document.getElementById('groupAdd').addEventListener('click', function() {
    var name = prompt('分组名称:');
    if (!name) return;
    var type = prompt('类型 (select/url-test/fallback):') || 'select';
    var regex = prompt('Regex:') || '';
    var url = prompt('URL:') || '';
    var interval = parseInt(prompt('Interval (秒):') || '0', 10);
    api('POST', '/api/groups', { name: name, type: type, regex: regex, url: url, interval: interval }).then(loadGroups);
  });
  document.getElementById('ruleAdd').addEventListener('click', function() {
    var type = prompt('规则类型 (DOMAIN/IP-CIDR/GEOIP/ etc):');
    if (!type) return;
    var param = prompt('规则参数:');
    if (!param) return;
    var config = prompt('配置 (可选):') || '';
    api('POST', '/api/rules', { type: type, param: param, config: config }).then(loadRules);
  });
  document.getElementById('tokenAdd').addEventListener('click', function() {
    var name = prompt('令牌名称:');
    if (!name) return;
    var token = prompt('令牌 (留空自动生成):') || '';
    api('POST', '/api/tokens', { token: token, name: name }).then(loadTokens);
  });

  // Initial load
  loadAirports();
  loadGroups();
  loadRules();
  loadTokens();
})();
