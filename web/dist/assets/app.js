// =============================================================================
// OutBalancer · Frontend SPA
// =============================================================================
'use strict';

// ----- API client ------------------------------------------------------------
const api = {
  async req(method, path, body) {
    const opts = { method, headers: {} };
    if (body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = typeof body === 'string' ? body : JSON.stringify(body);
    }
    const res = await fetch(path, opts);
    const ct = res.headers.get('content-type') || '';
    const data = ct.includes('application/json') ? await res.json() : await res.text();
    if (!res.ok) {
      const msg = (data && data.error) || ('HTTP ' + res.status);
      throw new Error(msg);
    }
    return data;
  },
  get:  (p)    => api.req('GET',  p),
  post: (p, b) => api.req('POST', p, b),
  put:  (p, b) => api.req('PUT',  p, b),
  del:  (p)    => api.req('DELETE', p),
};

// ----- App state -------------------------------------------------------------
const state = {
  servers:    [],
  metrics:    {},
  alerts:     [],
  trafficWindow: [],   // last ~60 samples for live chart
  overview:   {},
  algorithm:  'latency',
  algorithms: [],
  rules:      [],
  profiles:   [],
  schedules:  [],
  dns:        { servers: [], leak_protect: true, block_malware: false },
  listen:     { http_port: 10809, socks_port: 10808 },
  apiKey:     '',
  webhook:    '',
  system:     {},
};

// ----- WebSocket -------------------------------------------------------------
let ws = null;
let wsRetryDelay = 1000;
function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  ws = new WebSocket(`${proto}://${location.host}/ws`);
  ws.onopen = () => {
    wsRetryDelay = 1000;
    setConnDot(true);
  };
  ws.onmessage = (ev) => {
    try {
      const m = JSON.parse(ev.data);
      if (m.type === 'tick') handleTick(m);
    } catch (e) { /* ignore */ }
  };
  ws.onclose = () => {
    setConnDot(false);
    setTimeout(connectWS, wsRetryDelay);
    wsRetryDelay = Math.min(wsRetryDelay * 1.5, 8000);
  };
  ws.onerror = () => { try { ws.close(); } catch (e) {} };
}
function setConnDot(ok) {
  const d = document.getElementById('conn-dot');
  if (!d) return;
  d.style.color = ok ? 'var(--good)' : 'var(--danger)';
  d.style.boxShadow = ok ? '0 0 8px var(--good)' : '0 0 8px var(--danger)';
}

function handleTick(m) {
  if (Array.isArray(m.servers)) {
    state.servers = m.servers;
    document.getElementById('badge-servers').textContent = persianNum(m.servers.length);
  }
  if (Array.isArray(m.alerts)) {
    state.alerts = m.alerts;
    const unread = m.alerts.filter(a => !a.read).length;
    const badge = document.getElementById('badge-alerts');
    badge.textContent = persianNum(unread);
    badge.style.display = unread > 0 ? '' : 'none';
    document.getElementById('alert-dot').style.display = unread > 0 ? '' : 'none';
  }
  if (m.traffic) {
    state.trafficWindow.push(m.traffic);
    if (state.trafficWindow.length > 60) state.trafficWindow.shift();
  }
  // Re-render live bits without full re-render
  const route = location.hash.replace('#', '') || '/';
  if (route === '/')        liveUpdateDashboard();
  if (route === '/stats')   liveUpdateStats();
  if (route === '/servers') liveUpdateServersTable();
  if (route === '/alerts')  liveUpdateAlerts();
}

// ----- Helpers ---------------------------------------------------------------
function el(tag, props = {}, children = []) {
  const e = document.createElement(tag);
  for (const k in props) {
    if (k === 'class') e.className = props[k];
    else if (k === 'style') e.setAttribute('style', props[k]);
    else if (k.startsWith('on')) e[k.toLowerCase()] = props[k];
    else if (k === 'html') e.innerHTML = props[k];
    else e.setAttribute(k, props[k]);
  }
  for (const c of [].concat(children || [])) {
    if (c == null) continue;
    e.appendChild(c.nodeType ? c : document.createTextNode(c));
  }
  return e;
}
function $(sel) { return document.querySelector(sel); }
function $$(sel) { return Array.from(document.querySelectorAll(sel)); }

function persianNum(n) {
  return String(n).replace(/\d/g, d => '۰۱۲۳۴۵۶۷۸۹'[d]);
}
function fmtBytes(b) {
  if (!b || b < 0) return '۰ B';
  const u = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  while (b >= 1024 && i < u.length - 1) { b /= 1024; i++; }
  return `${b.toFixed(b < 10 ? 1 : 0)} ${u[i]}`;
}
function fmtMbps(v) {
  v = v || 0;
  if (v < 1) return v.toFixed(2);
  if (v < 100) return v.toFixed(1);
  return Math.round(v).toString();
}
function fmtPing(ms) {
  if (!ms || ms >= 9000) return 'timeout';
  return Math.round(ms) + ' ms';
}
function escHTML(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, c => ({
    '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'
  }[c]));
}
function pingColor(ms) {
  if (!ms || ms >= 9000) return { cls: 'bad', pct: 100 };
  if (ms < 80)  return { cls: '',    pct: Math.max(8,  ms / 4) };
  if (ms < 150) return { cls: '',    pct: Math.min(50, ms / 3) };
  if (ms < 250) return { cls: 'med', pct: Math.min(80, ms / 3) };
  return { cls: 'bad', pct: Math.min(100, ms / 4) };
}
function scoreColor(s) {
  if (s >= 70) return '#00ffa3';
  if (s >= 40) return '#ffb547';
  return '#ff4d6d';
}

function toast(msg, kind = 'ok') {
  const stack = $('#toasts');
  const icon = kind === 'ok' ? 'fa-circle-check' :
               kind === 'err' ? 'fa-circle-exclamation' : 'fa-circle-info';
  const t = el('div', { class: `toast ${kind}` }, [
    el('i', { class: `fa-solid ${icon} lead` }),
    el('span', {}, [msg])
  ]);
  stack.appendChild(t);
  setTimeout(() => { t.style.opacity = '0'; t.style.transition = 'opacity .3s'; }, 3500);
  setTimeout(() => t.remove(), 4000);
}

// ----- Modal -----------------------------------------------------------------
function openModal(title, bodyEl, footEls = []) {
  $('#modal-title').textContent = title;
  const body = $('#modal-body');
  body.innerHTML = '';
  body.appendChild(bodyEl);
  const foot = $('#modal-foot');
  foot.innerHTML = '';
  for (const f of footEls) foot.appendChild(f);
  $('#modal-overlay').classList.add('show');
}
function closeModal() {
  $('#modal-overlay').classList.remove('show');
}
function confirmModal(title, msg, onYes) {
  const body = el('p', { style: 'margin:0; line-height:1.7; color:var(--text-2);' }, [msg]);
  openModal(title, body, [
    el('button', { class: 'btn', onclick: closeModal }, ['انصراف']),
    el('button', { class: 'btn-danger', onclick: () => { closeModal(); onYes(); } }, ['تأیید']),
  ]);
}

// ----- Router ----------------------------------------------------------------
const routes = {
  '/':          { title: 'نمای کلی',    sub: '// real-time balancer telemetry', render: renderDashboard },
  '/servers':   { title: 'سرورها',      sub: '// مدیریت کانفیگ‌ها · ۱۰ سرور',  render: renderServers },
  '/stats':     { title: 'آمار زنده',   sub: '// نمودار ترافیک per-server',     render: renderStats },
  '/heatmap':   { title: 'هیت‌مپ مصرف',  sub: '// hour × day · last 7 days',     render: renderHeatmap },
  '/routing':   { title: 'قوانین مسیریابی', sub: '// custom rules engine',       render: renderRouting },
  '/algorithm': { title: 'الگوریتم بالانس', sub: '// 6 algorithms available',    render: renderAlgorithm },
  '/speed':     { title: 'Speed Boost',  sub: '// optimizations to maximize aggregate bandwidth', render: renderSpeed },
  '/profiles':  { title: 'پروفایل‌ها',   sub: '// چندین setup مجزا',             render: renderProfiles },
  '/schedule':  { title: 'زمانبندی',     sub: '// time-based profile switching', render: renderSchedule },
  '/alerts':    { title: 'هشدارها',      sub: '// realtime alert stream',        render: renderAlerts },
  '/dns':       { title: 'DNS هوشمند',   sub: '// servers · leak protect',       render: renderDNS },
  '/api':       { title: 'API & Webhook', sub: '// integrations',                render: renderAPI },
  '/logs':      { title: 'لاگ‌ها',        sub: '// last 500 entries',            render: renderLogs },
  '/settings':  { title: 'تنظیمات',     sub: '// listen ports · TUN',           render: renderSettings },
  '/backup':    { title: 'پشتیبان‌گیری', sub: '// import / export config',       render: renderBackup },
};

async function navigate() {
  const hash = location.hash.replace('#', '') || '/';
  const route = routes[hash] || routes['/'];

  // sidebar active state
  $$('.nav-item').forEach(n => n.classList.toggle('active', n.dataset.route === hash));

  // page title
  const titleEl = $('#page-title');
  const subEl = $('#page-subtitle');
  const t = route.title;
  // colorise last word
  const parts = t.split(' ');
  const last = parts.pop();
  titleEl.innerHTML = (parts.length ? escHTML(parts.join(' ')) + ' ' : '') + `<span>${escHTML(last)}</span>`;
  subEl.textContent = route.sub;

  $('#view').innerHTML = '<div style="padding:60px;text-align:center;color:var(--text-3);"><i class="fa-solid fa-spinner fa-spin"></i> در حال بارگذاری...</div>';

  try {
    await route.render();
  } catch (err) {
    $('#view').innerHTML = '';
    $('#view').appendChild(el('div', { class: 'panel', style: 'border-color: rgba(255,77,109,0.3);' }, [
      el('div', { class: 'panel-head' }, [
        el('div', {}, [
          el('h3', { class: 'panel-title', style: 'color:var(--danger);' }, ['خطا در بارگذاری']),
          el('div', { class: 'panel-sub' }, ['// ' + (err.message || err)]),
        ]),
      ]),
      el('p', { style: 'color:var(--text-2); margin:0;' }, ['لطفاً دوباره تلاش کنید یا backend را بررسی کنید.']),
    ]));
  }

  // close mobile sidebar after navigation
  $('#sidebar').classList.remove('open');
}

window.addEventListener('hashchange', navigate);

// ============================================================================
// PAGE: DASHBOARD (simplified - only key info)
// ============================================================================
async function renderDashboard() {
  const [overview, alerts] = await Promise.all([
    api.get('/api/overview'),
    api.get('/api/alerts'),
  ]);
  state.overview = overview;
  state.alerts = alerts;

  const view = $('#view');
  view.innerHTML = '';

  // KPI grid (4 cards - the most important metrics)
  const kpiHTML = `
    <div class="kpi-grid" id="kpi-grid">
      <div class="kpi-card" style="--glow: rgba(0,255,209,0.15);">
        <div class="kpi-head">
          <div class="kpi-icon-3d"><i class="fa-solid fa-gauge-high"></i></div>
          <div class="kpi-trend up"><i class="fa-solid fa-arrow-up"></i> live</div>
        </div>
        <div class="kpi-value" id="kpi-throughput">${persianNum(fmtMbps(overview.throughput_mbps))}<small>Mbps</small></div>
        <div class="kpi-label">throughput فعلی</div>
      </div>

      <div class="kpi-card" style="--glow: rgba(124,92,255,0.15);">
        <div class="kpi-head">
          <div class="kpi-icon-3d"><i class="fa-solid fa-stopwatch"></i></div>
          <div class="kpi-trend neutral">avg</div>
        </div>
        <div class="kpi-value" id="kpi-ping">${persianNum(Math.round(overview.avg_ping_ms || 0))}<small>ms</small></div>
        <div class="kpi-label">پینگ میانگین</div>
      </div>

      <div class="kpi-card" style="--glow: rgba(255,92,135,0.15);">
        <div class="kpi-head">
          <div class="kpi-icon-3d"><i class="fa-solid fa-server"></i></div>
          <div class="kpi-trend up" id="kpi-online-trend">${persianNum(overview.online_servers)} / ${persianNum(overview.total_servers)}</div>
        </div>
        <div class="kpi-value" id="kpi-online">${persianNum(overview.online_servers)}<small>active</small></div>
        <div class="kpi-label">سرورهای آنلاین</div>
      </div>

      <div class="kpi-card" style="--glow: rgba(255,181,71,0.15);">
        <div class="kpi-head">
          <div class="kpi-icon-3d"><i class="fa-solid fa-database"></i></div>
          <div class="kpi-trend down"><i class="fa-solid fa-arrow-up"></i> ماه</div>
        </div>
        <div class="kpi-value" id="kpi-monthly">${persianNum(fmtBytes(overview.monthly_bytes))}</div>
        <div class="kpi-label">مصرف ماهانه</div>
      </div>
    </div>
  `;
  view.insertAdjacentHTML('beforeend', kpiHTML);

  // Two-column: alerts + quick navigation cards
  const grid = el('div', { class: 'grid-2' });
  view.appendChild(grid);

  // Alerts widget (top 5)
  const alertsWidget = el('div', { class: 'widget' });
  alertsWidget.innerHTML = `
    <div class="widget-head">
      <i class="fa-solid fa-triangle-exclamation"></i>
      <div>
        <h3 class="widget-title">آخرین هشدارها</h3>
        <div class="panel-sub">// ${persianNum(alerts.length)} مورد · جدیدترین</div>
      </div>
      <div style="margin-right:auto;"><a class="chip" href="#/alerts">همه →</a></div>
    </div>
    <div id="dash-alerts"></div>
  `;
  grid.appendChild(alertsWidget);
  renderAlertList(alertsWidget.querySelector('#dash-alerts'), alerts.slice(0, 5));

  // Quick status widget
  const statusWidget = el('div', { class: 'widget' });
  statusWidget.innerHTML = `
    <div class="widget-head">
      <i class="fa-solid fa-shuffle"></i>
      <div>
        <h3 class="widget-title">وضعیت بالانسر</h3>
        <div class="panel-sub">// configuration snapshot</div>
      </div>
    </div>
    <div style="display:grid; gap:12px;">
      <div style="display:flex; justify-content:space-between; padding: 10px 12px; background: var(--glass); border-radius:10px;">
        <span style="color:var(--text-3); font-size:12px;">الگوریتم فعال</span>
        <b style="color: var(--accent);">${escHTML(algoName(overview.algorithm))}</b>
      </div>
      <div style="display:flex; justify-content:space-between; padding: 10px 12px; background: var(--glass); border-radius:10px;">
        <span style="color:var(--text-3); font-size:12px;">سرورها</span>
        <b>${persianNum(overview.online_servers)} / ${persianNum(overview.total_servers)} آنلاین</b>
      </div>
      <div style="display:flex; justify-content:space-between; padding: 10px 12px; background: var(--glass); border-radius:10px;">
        <span style="color:var(--text-3); font-size:12px;">Xray Core</span>
        <b style="color: ${overview.xray_disabled ? 'var(--warn)' : 'var(--good)'};">
          ${overview.xray_disabled ? '⚠ در دسترس نیست' : '✓ فعال'}
        </b>
      </div>
      <a class="btn-primary" href="#/stats" style="justify-content:center; text-decoration:none; margin-top:8px;">
        <i class="fa-solid fa-chart-line"></i> مشاهده آمار زنده
      </a>
    </div>
  `;
  grid.appendChild(statusWidget);

  // Quick links grid
  const links = [
    { href: '#/servers',   icon: 'server',     title: 'مدیریت سرورها',     desc: 'افزودن، ویرایش و حذف' },
    { href: '#/routing',   icon: 'route',      title: 'قوانین مسیریابی',   desc: 'هدایت دامنه‌ها' },
    { href: '#/algorithm', icon: 'shuffle',    title: 'الگوریتم بالانس',   desc: 'انتخاب از ۶ روش' },
    { href: '#/heatmap',   icon: 'fire',       title: 'هیت‌مپ مصرف',        desc: 'تحلیل ساعتی' },
    { href: '#/dns',       icon: 'shield-halved', title: 'DNS هوشمند',     desc: 'حفاظت از نشتی' },
    { href: '#/backup',    icon: 'cloud-arrow-up', title: 'پشتیبان‌گیری',  desc: 'import / export' },
  ];
  const quick = el('div', { class: 'panel' });
  quick.innerHTML = `
    <div class="panel-head">
      <div>
        <h3 class="panel-title">دسترسی سریع</h3>
        <div class="panel-sub">// shortcuts</div>
      </div>
    </div>
    <div class="grid-3" id="quick-links"></div>
  `;
  view.appendChild(quick);
  const ql = quick.querySelector('#quick-links');
  for (const l of links) {
    ql.innerHTML += `
      <a href="${l.href}" style="text-decoration:none;">
        <div class="widget" style="margin:0; cursor:pointer; transition: transform .15s, border-color .15s;"
             onmouseover="this.style.transform='translateY(-2px)'; this.style.borderColor='var(--border-bright)';"
             onmouseout="this.style.transform=''; this.style.borderColor='var(--border)';">
          <div class="widget-head">
            <i class="fa-solid fa-${l.icon}"></i>
            <div>
              <h3 class="widget-title">${l.title}</h3>
              <div class="panel-sub">${l.desc}</div>
            </div>
            <i class="fa-solid fa-arrow-left" style="margin-right:auto; color:var(--text-3);"></i>
          </div>
        </div>
      </a>
    `;
  }
}

function liveUpdateDashboard() {
  const tEl = document.getElementById('kpi-throughput');
  if (!tEl) return;
  let total = 0, count = 0, totalPing = 0, online = 0;
  for (const s of state.servers) {
    if (s.status === 'online') {
      total += s.speed_mbps || 0;
      online++;
      if (s.ping_ms > 0 && s.ping_ms < 2000) {
        totalPing += s.ping_ms;
        count++;
      }
    }
  }
  tEl.innerHTML = `${persianNum(fmtMbps(total))}<small>Mbps</small>`;
  $('#kpi-ping').innerHTML = `${persianNum(Math.round(count > 0 ? totalPing / count : 0))}<small>ms</small>`;
  $('#kpi-online').innerHTML = `${persianNum(online)}<small>active</small>`;
}

// ============================================================================
// PAGE: SERVERS
// ============================================================================
async function renderServers() {
  const servers = await api.get('/api/servers');
  state.servers = servers;

  const view = $('#view');
  view.innerHTML = '';

  const panel = el('div', { class: 'panel' });
  view.appendChild(panel);
  panel.innerHTML = `
    <div class="panel-head">
      <div>
        <h3 class="panel-title">سرورهای متصل</h3>
        <div class="panel-sub">// ${persianNum(servers.length)} سرور · drag to reorder</div>
      </div>
      <div class="panel-actions">
        <input id="server-filter" placeholder="فیلتر..." style="width:180px; padding:6px 12px; font-size:12px;"/>
        <div class="chip active" data-filter="all">همه</div>
        <div class="chip" data-filter="online">آنلاین</div>
        <div class="chip" data-filter="degraded">degraded</div>
        <div class="chip" data-filter="offline">آفلاین</div>
        <button class="btn" onclick="openImportModal()"><i class="fa-solid fa-file-import"></i> import گروهی</button>
        <button class="btn-primary" onclick="openAddServerModal()"><i class="fa-solid fa-plus"></i> افزودن</button>
      </div>
    </div>
    <div class="table-wrap">
      <table>
        <thead><tr>
          <th>سرور</th><th>وضعیت</th><th>پینگ</th><th>سرعت</th>
          <th>کانکشن</th><th>مصرف ماه</th><th>وزن</th><th>نمره</th><th>عملیات</th>
        </tr></thead>
        <tbody id="servers-tbody"></tbody>
      </table>
    </div>
  `;

  let filter = 'all', search = '';
  panel.querySelectorAll('.chip[data-filter]').forEach(c => {
    c.addEventListener('click', () => {
      panel.querySelectorAll('.chip[data-filter]').forEach(x => x.classList.remove('active'));
      c.classList.add('active');
      filter = c.dataset.filter;
      drawServerRows(filter, search);
    });
  });
  panel.querySelector('#server-filter').addEventListener('input', (e) => {
    search = e.target.value.toLowerCase();
    drawServerRows(filter, search);
  });
  drawServerRows(filter, search);
}

function drawServerRows(filter, search) {
  const tbody = $('#servers-tbody');
  if (!tbody) return;
  let rows = state.servers;
  if (filter !== 'all') rows = rows.filter(s => s.status === filter);
  if (search) rows = rows.filter(s => (s.name + ' ' + s.address).toLowerCase().includes(search));

  if (rows.length === 0) {
    tbody.innerHTML = `<tr><td colspan="9">
      <div class="empty-state">
        <i class="fa-solid fa-server"></i>
        <h3>سروری یافت نشد</h3>
        <p>یا فیلتر را تغییر دهید یا کانفیگ جدیدی اضافه کنید.</p>
      </div>
    </td></tr>`;
    return;
  }

  tbody.innerHTML = rows.map(s => {
    const p = pingColor(s.ping_ms);
    const sc = s.score || 0;
    const dash = sc + ' ' + (100 - sc);
    const stroke = scoreColor(sc);
    return `
      <tr data-id="${s.id}">
        <td>
          <div class="server-cell">
            <div class="flag-3d">${s.flag || '🌐'}</div>
            <div class="server-name">
              <b>${escHTML(s.name)}</b>
              <span>${escHTML(s.protocol + '://' + s.address + ':' + s.port)}</span>
            </div>
          </div>
        </td>
        <td><span class="status ${s.status || 'unknown'}">${(s.status || 'unknown').toUpperCase()}</span></td>
        <td>
          <div class="ping-meter">
            <div class="ping-bar-bg"><div class="ping-bar-fill ${p.cls}" style="width:${p.pct}%"></div></div>
            <span class="ping-num" style="color: ${p.cls === 'bad' ? 'var(--danger)' : p.cls === 'med' ? 'var(--warn)' : ''}">${fmtPing(s.ping_ms)}</span>
          </div>
        </td>
        <td><b>${persianNum(fmtMbps(s.speed_mbps))} Mbps</b></td>
        <td>${persianNum(s.connections || 0)}</td>
        <td>${persianNum(fmtBytes(s.usage_month_bytes))}</td>
        <td><div style="background:var(--glass); padding:3px 9px; border-radius:8px; font-family:'JetBrains Mono', monospace; font-size:11px; display:inline-block;">${(s.weight || 1).toFixed(1)}</div></td>
        <td>
          <div class="score-ring">
            <svg width="38" height="38">
              <circle cx="19" cy="19" r="15" stroke="rgba(255,255,255,0.06)" stroke-width="3" fill="none"/>
              <circle cx="19" cy="19" r="15" stroke="${stroke}" stroke-width="3" fill="none"
                stroke-dasharray="${(sc / 100 * 94).toFixed(1)} 100" stroke-linecap="round"
                style="filter: drop-shadow(0 0 4px ${stroke});"/>
            </svg>
            <span class="score-num">${persianNum(sc)}</span>
          </div>
        </td>
        <td>
          <div class="row-actions">
            <div class="row-btn" onclick="testServer('${s.id}')" title="تست"><i class="fa-solid fa-bolt"></i></div>
            <div class="row-btn" onclick="openEditServerModal('${s.id}')" title="ویرایش"><i class="fa-solid fa-pen"></i></div>
            <div class="row-btn danger" onclick="deleteServer('${s.id}')" title="حذف"><i class="fa-solid fa-trash"></i></div>
          </div>
        </td>
      </tr>
    `;
  }).join('');
}

function liveUpdateServersTable() {
  // simple approach: re-draw if currently visible
  const tbody = document.getElementById('servers-tbody');
  if (!tbody) return;
  const activeChip = document.querySelector('.chip[data-filter].active');
  const filter = activeChip ? activeChip.dataset.filter : 'all';
  const searchInput = document.getElementById('server-filter');
  const search = searchInput ? searchInput.value.toLowerCase() : '';
  drawServerRows(filter, search);
}

async function testServer(id) {
  try {
    const r = await api.get('/api/servers/test/' + id);
    if (r.ok) toast(`تست موفق · پینگ: ${persianNum(Math.round(r.ping_ms))} ms`, 'ok');
    else toast(`تست ناموفق · ${r.error || 'unreachable'}`, 'err');
  } catch (e) { toast(e.message, 'err'); }
}

async function deleteServer(id) {
  const srv = state.servers.find(s => s.id === id);
  confirmModal('حذف سرور', `آیا از حذف "${srv ? srv.name : id}" مطمئن هستید؟`, async () => {
    try {
      await api.del('/api/servers/' + id);
      toast('سرور حذف شد', 'ok');
      renderServers();
    } catch (e) { toast(e.message, 'err'); }
  });
}

function openAddServerModal() {
  const body = el('div', {});
  body.innerHTML = `
    <div class="form-field" style="margin-bottom:14px;">
      <label>vless:// یا vmess:// URL</label>
      <textarea id="vless-input" placeholder="vless://uuid@host:port?type=tcp&security=tls#name" style="min-height:90px;"></textarea>
    </div>
    <div class="form-field">
      <label>یا چندتا با هم (هر خط یکی)</label>
      <textarea id="vless-bulk" placeholder="vless://...&#10;vless://...&#10;vless://..." style="min-height:120px;"></textarea>
    </div>
  `;
  openModal('افزودن کانفیگ جدید', body, [
    el('button', { class: 'btn', onclick: closeModal }, ['انصراف']),
    el('button', { class: 'btn-primary', onclick: submitAddServer }, [
      el('i', { class: 'fa-solid fa-plus' }), ' افزودن'
    ]),
  ]);
}

async function submitAddServer() {
  const single = $('#vless-input').value.trim();
  const bulk = $('#vless-bulk').value.trim();
  try {
    if (single && !bulk) {
      await api.post('/api/servers', { url: single });
      toast('سرور اضافه شد', 'ok');
    } else if (bulk) {
      const r = await api.post('/api/servers/import', bulk);
      toast(`اضافه شد: ${persianNum(r.added)} · ناموفق: ${persianNum((r.failed || []).length)}`, 'ok');
    } else {
      toast('چیزی وارد نکردی', 'err');
      return;
    }
    closeModal();
    if ((location.hash.replace('#', '') || '/') === '/servers') renderServers();
  } catch (e) { toast(e.message, 'err'); }
}

function openImportModal() { openAddServerModal(); }

function openEditServerModal(id) {
  const s = state.servers.find(x => x.id === id);
  if (!s) return;
  const body = el('div', {});
  body.innerHTML = `
    <div class="form-row">
      <div class="form-field">
        <label>نام</label>
        <input id="ed-name" type="text" value="${escHTML(s.name)}"/>
      </div>
      <div class="form-field">
        <label>وزن</label>
        <input id="ed-weight" type="number" step="0.1" min="0" value="${s.weight || 1}"/>
      </div>
    </div>
    <div class="form-row">
      <div class="form-field">
        <label>سهمیه (GB) — صفر یعنی نامحدود</label>
        <input id="ed-quota" type="number" min="0" value="${s.quota_gb || 0}"/>
      </div>
      <div class="form-field" style="display:flex; flex-direction:row; align-items:center; gap:12px; padding-top:24px;">
        <label class="switch"><input id="ed-enabled" type="checkbox" ${s.enabled ? 'checked' : ''}/><span class="slider"></span></label>
        <span style="font-size:13px;">فعال</span>
      </div>
    </div>
  `;
  openModal('ویرایش ' + s.name, body, [
    el('button', { class: 'btn', onclick: closeModal }, ['انصراف']),
    el('button', { class: 'btn-primary', onclick: () => submitEditServer(id) }, ['ذخیره']),
  ]);
}

async function submitEditServer(id) {
  const s = state.servers.find(x => x.id === id);
  if (!s) return;
  const updated = {
    ...s,
    name: $('#ed-name').value,
    weight: parseFloat($('#ed-weight').value || 1),
    quota_gb: parseInt($('#ed-quota').value || 0, 10),
    enabled: $('#ed-enabled').checked,
  };
  try {
    await api.put('/api/servers/' + id, updated);
    toast('ذخیره شد', 'ok');
    closeModal();
    renderServers();
  } catch (e) { toast(e.message, 'err'); }
}

// ============================================================================
// PAGE: STATS (live traffic chart)
// ============================================================================
async function renderStats() {
  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">ترافیک زنده per-server</h3>
          <div class="panel-sub">// 60s rolling window · 1Hz refresh</div>
        </div>
        <div class="panel-actions" id="stats-chips">
          <div class="chip active" data-range="60">۱ دقیقه</div>
          <div class="chip" data-range="600">۱۰ دقیقه</div>
        </div>
      </div>
      <div id="stats-chart" style="height: 360px;"></div>
      <div id="stats-legend" style="display:flex; gap:14px; flex-wrap:wrap; margin-top:14px;"></div>
    </div>

    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">برترین سرورها (now)</h3>
          <div class="panel-sub">// sorted by current throughput</div>
        </div>
      </div>
      <div id="stats-top"></div>
    </div>
  `;
  liveUpdateStats();
}

const COLORS = ['#00ffd1','#7c5cff','#ff5c87','#ffb547','#00ffa3','#ff7a93','#5cf2ff','#c594ff','#ff9047','#94e0ff'];

function liveUpdateStats() {
  const chartEl = document.getElementById('stats-chart');
  const legendEl = document.getElementById('stats-legend');
  const topEl = document.getElementById('stats-top');
  if (!chartEl) return;

  const window = state.trafficWindow;
  const servers = state.servers.slice(0, 10); // limit lines

  if (window.length === 0) {
    chartEl.innerHTML = '<div class="empty-state"><i class="fa-solid fa-chart-line"></i><h3>در انتظار داده...</h3></div>';
    return;
  }

  const W = chartEl.clientWidth || 800, H = 360;
  const pad = { l: 50, r: 10, t: 10, b: 24 };
  const cw = W - pad.l - pad.r;
  const ch = H - pad.t - pad.b;
  let maxY = 1;
  for (const sample of window) {
    for (const id in sample.per_server) {
      if (sample.per_server[id] > maxY) maxY = sample.per_server[id];
    }
  }
  maxY = Math.ceil(maxY / 100) * 100 || 100;

  const x = (i) => pad.l + (i / Math.max(1, window.length - 1)) * cw;
  const y = (v) => pad.t + ch - (v / maxY) * ch;

  let svg = `<svg width="100%" height="${H}" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" xmlns="http://www.w3.org/2000/svg">`;
  // grid
  svg += '<g stroke="rgba(255,255,255,0.05)" stroke-width="1">';
  for (let i = 1; i <= 4; i++) svg += `<line x1="${pad.l}" x2="${W - pad.r}" y1="${pad.t + ch * i / 4}" y2="${pad.t + ch * i / 4}"/>`;
  svg += '</g>';
  // y labels
  svg += '<g fill="rgba(168,176,201,0.5)" font-size="10" font-family="JetBrains Mono">';
  for (let i = 0; i <= 4; i++) {
    const v = maxY - (maxY * i / 4);
    svg += `<text x="${pad.l - 6}" y="${(pad.t + ch * i / 4) + 4}" text-anchor="end">${Math.round(v)}</text>`;
  }
  svg += '</g>';

  // lines per server
  servers.forEach((s, idx) => {
    const color = COLORS[idx % COLORS.length];
    let path = '';
    window.forEach((sample, i) => {
      const v = sample.per_server[s.id] || 0;
      path += (i === 0 ? 'M' : 'L') + x(i).toFixed(1) + ',' + y(v).toFixed(1) + ' ';
    });
    let area = path + ` L${x(window.length - 1).toFixed(1)},${(pad.t + ch).toFixed(1)} L${pad.l},${(pad.t + ch).toFixed(1)} Z`;

    svg += `<defs><linearGradient id="g_${idx}" x1="0" x2="0" y1="0" y2="1">
      <stop offset="0" stop-color="${color}" stop-opacity="0.30"/>
      <stop offset="1" stop-color="${color}" stop-opacity="0"/></linearGradient></defs>`;
    svg += `<path d="${area}" fill="url(#g_${idx})"/>`;
    svg += `<path d="${path}" fill="none" stroke="${color}" stroke-width="1.8" stroke-linejoin="round"/>`;
  });
  svg += '</svg>';
  chartEl.innerHTML = svg;

  legendEl.innerHTML = servers.map((s, i) => {
    const color = COLORS[i % COLORS.length];
    const sample = window[window.length - 1];
    const v = sample ? (sample.per_server[s.id] || 0) : 0;
    return `<div style="display:flex; align-items:center; gap:8px; font-size:11px; font-family:'JetBrains Mono', monospace; color: var(--text-2);">
      <span style="width:8px; height:8px; border-radius:50%; background:${color}; box-shadow:0 0 6px ${color};"></span>
      ${s.flag} ${escHTML(s.name)} <b style="color:${color};">${persianNum(fmtMbps(v))} Mbps</b>
    </div>`;
  }).join('');

  // top now sort
  const top = [...servers]
    .map(s => ({ ...s, current: window.length ? (window[window.length - 1].per_server[s.id] || 0) : 0 }))
    .sort((a, b) => b.current - a.current);
  topEl.innerHTML = top.map((s, i) => `
    <div style="display:flex; align-items:center; gap:14px; padding:10px 0; border-bottom: 1px dashed var(--border);">
      <span style="font-family:'JetBrains Mono', monospace; color: var(--text-3); width:24px;">#${persianNum(i+1)}</span>
      <div class="flag-3d" style="width:30px; height:30px; font-size:14px;">${s.flag}</div>
      <div style="flex:1; min-width:0;">
        <b>${escHTML(s.name)}</b>
        <div style="font-size:11px; color:var(--text-3); font-family:'JetBrains Mono', monospace;">${escHTML(s.address)}</div>
      </div>
      <div style="text-align:left;">
        <b style="color: var(--accent); font-family:'JetBrains Mono', monospace;">${persianNum(fmtMbps(s.current))} Mbps</b>
      </div>
    </div>
  `).join('');
}

// ============================================================================
// PAGE: HEATMAP
// ============================================================================
async function renderHeatmap() {
  const cells = await api.get('/api/heatmap');
  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">هیت‌مپ مصرف ۷ روز اخیر</h3>
          <div class="panel-sub">// hour × day · darker = more usage</div>
        </div>
      </div>
      <div style="display: grid; grid-template-columns: 60px 1fr; gap: 8px; align-items: center;">
        <div></div>
        <div style="display:flex; justify-content:space-between; padding: 0 4px; font-size:10px; color:var(--text-3); font-family:'JetBrains Mono', monospace;">
          ${[0,3,6,9,12,15,18,21].map(h => `<span>${persianNum(h.toString().padStart(2,'0'))}:۰۰</span>`).join('')}
        </div>
        ${cells.map((row, i) => `
          <div style="font-size: 11px; color: var(--text-3); font-family: 'JetBrains Mono', monospace;">${['شنبه','یک‌ش','دوش','سه‌ش','چهار','پنج‌','جمعه'][i] || 'روز ' + i}</div>
          <div class="heatmap-grid">
            ${row.map(v => `<div class="heat-cell" title="مصرف ${(v*100).toFixed(0)}%" style="background: rgba(0,255,209,${v.toFixed(2)}); ${v > 0.7 ? 'box-shadow: 0 0 6px rgba(0,255,209,0.3);' : ''}"></div>`).join('')}
          </div>
        `).join('')}
      </div>
      <div style="margin-top: 16px; display:flex; align-items:center; gap:10px; font-size:11px; color:var(--text-3); font-family:'JetBrains Mono', monospace;">
        کم
        <div style="display:flex; gap:3px;">
          ${[0.05,0.2,0.4,0.65,0.9].map(v => `<div style="width:18px; height:12px; border-radius:3px; background: rgba(0,255,209,${v});"></div>`).join('')}
        </div>
        زیاد
      </div>
    </div>

    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">پر مصرف‌ترین دامنه‌ها</h3>
          <div class="panel-sub">// last 24 hours</div>
        </div>
      </div>
      <div id="top-domains"></div>
    </div>
  `;

  const domains = await api.get('/api/topdomains');
  const tdEl = $('#top-domains');
  tdEl.innerHTML = domains.map(d => `
    <div style="display:flex; align-items:center; gap:14px; padding:10px 0; border-bottom: 1px dashed var(--border);">
      <div style="width: 32px; height: 32px; border-radius:8px; background: var(--glass); display:grid; place-items:center; color: ${d.color};">
        <i class="fa-brands fa-${d.icon}"></i>
      </div>
      <div style="flex:1; font-family:'JetBrains Mono', monospace; font-size:13px;">${escHTML(d.domain)}</div>
      <b style="color: var(--accent); font-family:'JetBrains Mono', monospace;">${persianNum(fmtBytes(d.bytes))}</b>
    </div>
  `).join('');
}

// ============================================================================
// PAGE: ROUTING RULES
// ============================================================================
async function renderRouting() {
  const rules = await api.get('/api/rules');
  state.rules = rules || [];
  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">قوانین مسیریابی</h3>
          <div class="panel-sub">// ${persianNum(state.rules.length)} قانون · بر اساس ترتیب اجرا میشن</div>
        </div>
        <div class="panel-actions">
          <button class="btn-primary" onclick="openRuleModal()"><i class="fa-solid fa-plus"></i> قانون جدید</button>
        </div>
      </div>
      <div id="rules-body"></div>
    </div>
  `;
  drawRules();
}

function drawRules() {
  const body = $('#rules-body');
  if (!state.rules.length) {
    body.innerHTML = `<div class="empty-state"><i class="fa-solid fa-route"></i><h3>هنوز قانونی نداری</h3><p>روی "قانون جدید" کلیک کن تا اولین قانون را اضافه کنی.</p></div>`;
    return;
  }
  body.innerHTML = state.rules.map((r, i) => `
    <div style="display:flex; align-items:center; gap:14px; padding:14px 12px; background: var(--glass); border:1px solid var(--border); border-radius:14px; margin-bottom:10px;">
      <div style="width:28px; height:28px; border-radius:8px; background: var(--glass-strong); display:grid; place-items:center; font-family:'JetBrains Mono', monospace; font-size:11px; color:var(--text-3);">${persianNum(i+1)}</div>
      <div style="flex:1; min-width:0;">
        <b style="display:block;">${escHTML(r.name || 'بدون نام')}</b>
        <div style="display:flex; gap:6px; margin-top:6px; flex-wrap:wrap;">
          ${(r.domains || []).slice(0,4).map(d => `<span style="background:var(--glass-strong); padding:2px 8px; border-radius:6px; font-family:'JetBrains Mono', monospace; font-size:10px;">${escHTML(d)}</span>`).join('')}
          ${(r.ips || []).slice(0,2).map(d => `<span style="background:var(--glass-strong); padding:2px 8px; border-radius:6px; font-family:'JetBrains Mono', monospace; font-size:10px;">${escHTML(d)}</span>`).join('')}
          ${r.geoip ? `<span style="background:var(--glass-strong); padding:2px 8px; border-radius:6px; font-family:'JetBrains Mono', monospace; font-size:10px;">geoip:${escHTML(r.geoip)}</span>` : ''}
        </div>
      </div>
      <i class="fa-solid fa-arrow-left" style="color:var(--text-3);"></i>
      <div style="font-family:'JetBrains Mono', monospace; font-size:12px; color: var(--accent); padding: 6px 12px; background: rgba(0,255,209,0.08); border-radius: 8px;">
        ${ruleTargetLabel(r.target)}
      </div>
      <label class="switch"><input type="checkbox" ${r.enabled ? 'checked' : ''} onchange="toggleRule('${r.id}', this.checked)"/><span class="slider"></span></label>
      <div class="row-btn" onclick="openRuleModal('${r.id}')" title="ویرایش"><i class="fa-solid fa-pen"></i></div>
      <div class="row-btn danger" onclick="deleteRule('${r.id}')" title="حذف"><i class="fa-solid fa-trash"></i></div>
    </div>
  `).join('');
}

function ruleTargetLabel(t) {
  if (t === 'direct')   return '↩ DIRECT';
  if (t === 'block')    return '⛔ BLOCK';
  if (t === 'balanced') return '⚖ BALANCED';
  const s = state.servers.find(x => x.id === t);
  return s ? `🔒 ${escHTML(s.name)}` : `⚖ BALANCED`;
}

function openRuleModal(id) {
  const r = id ? state.rules.find(x => x.id === id) : null;
  const body = el('div', {});
  const targetOpts = [
    `<option value="balanced">⚖ Balanced (همه سرورها)</option>`,
    `<option value="direct">↩ Direct (بدون پروکسی)</option>`,
    `<option value="block">⛔ Block (مسدود)</option>`,
    ...state.servers.map(s => `<option value="${s.id}">🔒 ${escHTML(s.name)}</option>`)
  ].join('');
  body.innerHTML = `
    <div class="form-row">
      <div class="form-field">
        <label>نام قانون</label>
        <input id="rule-name" type="text" value="${r ? escHTML(r.name) : ''}" placeholder="مثال: YouTube → پرسرعت"/>
      </div>
    </div>
    <div class="form-row">
      <div class="form-field">
        <label>دامنه‌ها (هر خط یکی)</label>
        <textarea id="rule-domains" placeholder="youtube.com&#10;googlevideo.com">${r && r.domains ? escHTML(r.domains.join('\n')) : ''}</textarea>
      </div>
      <div class="form-field">
        <label>IP / CIDR (هر خط یکی)</label>
        <textarea id="rule-ips" placeholder="1.1.1.1&#10;192.168.0.0/16">${r && r.ips ? escHTML(r.ips.join('\n')) : ''}</textarea>
      </div>
    </div>
    <div class="form-row">
      <div class="form-field">
        <label>GeoIP (مثل ir, us, cn — اختیاری)</label>
        <input id="rule-geo" type="text" value="${r ? escHTML(r.geoip || '') : ''}" placeholder="ir"/>
      </div>
      <div class="form-field">
        <label>هدف</label>
        <select id="rule-target">${targetOpts}</select>
      </div>
    </div>
    <div class="form-row" style="margin-top: 6px;">
      <div style="display:flex; align-items:center; gap:12px;">
        <label class="switch"><input id="rule-enabled" type="checkbox" ${(!r || r.enabled) ? 'checked' : ''}/><span class="slider"></span></label>
        <span style="font-size:13px;">فعال</span>
      </div>
    </div>
  `;
  setTimeout(() => { if (r) body.querySelector('#rule-target').value = r.target || 'balanced'; }, 0);

  openModal(r ? 'ویرایش قانون' : 'قانون جدید', body, [
    el('button', { class: 'btn', onclick: closeModal }, ['انصراف']),
    el('button', { class: 'btn-primary', onclick: () => saveRule(id) }, ['ذخیره']),
  ]);
}

async function saveRule(id) {
  const splitLines = (s) => s.split('\n').map(x => x.trim()).filter(Boolean);
  const rule = {
    id: id || '',
    name: $('#rule-name').value || 'قانون جدید',
    domains: splitLines($('#rule-domains').value),
    ips: splitLines($('#rule-ips').value),
    geoip: $('#rule-geo').value.trim(),
    target: $('#rule-target').value,
    enabled: $('#rule-enabled').checked,
  };
  try {
    if (id) await api.put('/api/rules/' + id, rule);
    else await api.post('/api/rules', rule);
    toast('ذخیره شد', 'ok');
    closeModal();
    renderRouting();
  } catch (e) { toast(e.message, 'err'); }
}

async function toggleRule(id, enabled) {
  const r = state.rules.find(x => x.id === id);
  if (!r) return;
  r.enabled = enabled;
  try { await api.put('/api/rules/' + id, r); toast('وضعیت تغییر کرد', 'ok'); } catch (e) { toast(e.message, 'err'); }
}

async function deleteRule(id) {
  confirmModal('حذف قانون', 'آیا از حذف این قانون مطمئن هستید؟', async () => {
    try { await api.del('/api/rules/' + id); toast('حذف شد', 'ok'); renderRouting(); } catch (e) { toast(e.message, 'err'); }
  });
}

// ============================================================================
// PAGE: ALGORITHM
// ============================================================================
async function renderAlgorithm() {
  const [list, current] = await Promise.all([
    api.get('/api/algorithms'),
    api.get('/api/algorithm'),
  ]);
  state.algorithms = list;
  state.algorithm = current.algorithm || 'latency';

  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">انتخاب الگوریتم بالانس</h3>
          <div class="panel-sub">// تعیین می‌کنه هر کانکشن به کدوم سرور بره</div>
        </div>
      </div>
      <div id="algo-list"></div>
    </div>
  `;
  drawAlgorithms();
}

function algoName(id) {
  const a = (state.algorithms || []).find(x => x.id === id);
  return a ? a.name : id;
}

function algoIcon(id) {
  return ({
    latency: 'fa-stopwatch', speed: 'fa-rocket', weighted: 'fa-scale-balanced',
    leastconn: 'fa-link', roundrobin: 'fa-rotate', random: 'fa-dice'
  })[id] || 'fa-shuffle';
}

function drawAlgorithms() {
  const lst = $('#algo-list');
  if (!lst) return;
  lst.innerHTML = state.algorithms.map(a => {
    const active = a.id === state.algorithm;
    return `
      <div onclick="selectAlgorithm('${a.id}')"
           style="display:flex; align-items:center; gap:16px; padding:18px 16px; background: ${active ? 'rgba(0,255,209,0.06)' : 'var(--glass)'}; border:1px solid ${active ? 'rgba(0,255,209,0.3)' : 'var(--border)'}; border-radius:14px; margin-bottom:10px; cursor:pointer; transition: all .15s;"
           onmouseover="if(!${active})this.style.background='var(--glass-strong)';"
           onmouseout="if(!${active})this.style.background='var(--glass)';">
        <div style="width:50px; height:50px; border-radius:14px; background: ${active ? 'rgba(0,255,209,0.15)' : 'var(--glass-strong)'}; display:grid; place-items:center; color: ${active ? 'var(--accent)' : 'var(--text-2)'}; font-size:20px;">
          <i class="fa-solid ${algoIcon(a.id)}"></i>
        </div>
        <div style="flex:1;">
          <b style="display:block; font-size: 15px;">${escHTML(a.name)}</b>
          <div style="font-size:12px; color: var(--text-3); margin-top:4px;">${escHTML(a.desc)}</div>
        </div>
        ${active ? '<i class="fa-solid fa-circle-check" style="color: var(--accent); font-size: 22px; filter: drop-shadow(0 0 6px var(--accent));"></i>' : '<i class="fa-regular fa-circle" style="color: var(--text-3); font-size: 22px;"></i>'}
      </div>
    `;
  }).join('');
}

async function selectAlgorithm(id) {
  try {
    await api.post('/api/algorithm', { algorithm: id });
    state.algorithm = id;
    drawAlgorithms();
    toast('الگوریتم تغییر کرد به: ' + algoName(id), 'ok');
  } catch (e) { toast(e.message, 'err'); }
}

// ============================================================================
// PAGE: SPEED BOOST — optimizations to maximize aggregate bandwidth
// ============================================================================
async function renderSpeed() {
  const speed = await api.get('/api/speed');
  const view = $('#view');

  // Hero panel — explainer
  const hero = el('div', { class: 'panel', style: 'background: linear-gradient(135deg, rgba(0,255,209,0.06), rgba(124,92,255,0.06));' });
  hero.innerHTML = `
    <div class="panel-head">
      <div>
        <h3 class="panel-title"><i class="fa-solid fa-rocket" style="color:var(--accent); margin-left:8px;"></i> Speed Boost</h3>
        <div class="panel-sub">// چطور OutBalancer سرعت رو حداکثر می‌کنه</div>
      </div>
    </div>
    <div style="display:grid; gap:14px; color:var(--text-2); font-size:13px; line-height:1.7;">
      <div style="padding:14px; background:var(--glass); border-radius:12px; border-right: 3px solid var(--accent);">
        <b style="color:var(--text-1);">۱. هیچ‌وقت کندتر از یه سرور نیست</b><br>
        با <code style="color:var(--accent); font-family:'JetBrains Mono';">leastPing</code> همیشه بهترین سرور برای هر کانکشن انتخاب میشه. در بدترین حالت سرعتت = سرعت سریع‌ترین کانفیگت.
      </div>
      <div style="padding:14px; background:var(--glass); border-radius:12px; border-right: 3px solid var(--accent-2);">
        <b style="color:var(--text-1);">۲. مرور چندتاب / استریم: جمع پهنای باند</b><br>
        یوتیوب، نتفلیکس و مرورگرها هر چند ثانیه یه chunk جدید درخواست می‌کنن. هر chunk از یه سرور متفاوت میره. <b style="color:var(--good);">۵+۵+۵ = ۱۵ مگ واقعی</b>.
      </div>
      <div style="padding:14px; background:var(--glass); border-radius:12px; border-right: 3px solid var(--accent-3);">
        <b style="color:var(--text-1);">۳. دانلود فایل بزرگ: Chunk Downloader</b><br>
        فایل‌های مستقیم HTTPS رو با Range Requests میشکنیم به N تکه و هرکدوم رو از یه سرور دانلود می‌کنیم. مثل IDM روی چند پروکسی.
      </div>
      <div style="padding:14px; background:rgba(255,181,71,0.08); border-radius:12px; border-right: 3px solid var(--warn);">
        <b style="color:var(--warn);">⚠ محدودیت ذاتی TCP:</b> یه stream واحد (مثلاً یه کال Zoom یا یه فایل با single-connection) همیشه از یه سرور میره — این محدودیت TCP است نه OutBalancer.
      </div>
    </div>
  `;
  view.innerHTML = '';
  view.appendChild(hero);

  // Settings panel
  const panel = el('div', { class: 'panel' });
  panel.innerHTML = `
    <div class="panel-head">
      <div>
        <h3 class="panel-title">تنظیمات Speed Boost</h3>
        <div class="panel-sub">// همه پیش‌فرض‌ها برای حداکثر سرعت بدون افت تنظیم شدن</div>
      </div>
    </div>

    <div style="display:grid; gap:18px;">

      <div style="display:flex; align-items:center; gap:16px; padding:16px; background:var(--glass); border-radius:14px; border:1px solid var(--border);">
        <div style="width:44px; height:44px; border-radius:12px; background:rgba(0,255,209,0.1); display:grid; place-items:center; color:var(--accent); font-size:18px;">
          <i class="fa-solid fa-link"></i>
        </div>
        <div style="flex:1;">
          <div style="font-weight:600; font-size:14px;">Sticky-by-Domain</div>
          <div style="color:var(--text-3); font-size:12px; margin-top:2px;">هر دامنه به یه سرور می‌چسبه — handshake های TLS تکرار نمیشن (پیش‌فرض: روشن)</div>
        </div>
        <label class="switch">
          <input type="checkbox" id="sp-sticky" ${speed.sticky_by_domain ? 'checked' : ''}>
          <span class="slider"></span>
        </label>
      </div>

      <div style="display:flex; align-items:center; gap:16px; padding:16px; background:var(--glass); border-radius:14px; border:1px solid var(--border);">
        <div style="width:44px; height:44px; border-radius:12px; background:rgba(124,92,255,0.1); display:grid; place-items:center; color:var(--accent-2); font-size:18px;">
          <i class="fa-solid fa-stopwatch"></i>
        </div>
        <div style="flex:1;">
          <div style="font-weight:600; font-size:14px;">Sticky TTL (ثانیه)</div>
          <div style="color:var(--text-3); font-size:12px; margin-top:2px;">مدت چسبیدن هر دامنه به سرور — کم: parallelism بیشتر، زیاد: stability بیشتر</div>
        </div>
        <input type="number" id="sp-ttl" value="${speed.sticky_ttl_sec}" min="5" max="3600" style="width:90px; text-align:center;">
      </div>

      <div style="display:flex; align-items:center; gap:16px; padding:16px; background:var(--glass); border-radius:14px; border:1px solid var(--border);">
        <div style="width:44px; height:44px; border-radius:12px; background:rgba(255,92,135,0.1); display:grid; place-items:center; color:var(--accent-3); font-size:18px;">
          <i class="fa-solid fa-shuffle"></i>
        </div>
        <div style="flex:1;">
          <div style="font-weight:600; font-size:14px;">Smart Split</div>
          <div style="color:var(--text-3); font-size:12px; margin-top:2px;">پخش هوشمند کانکشن‌های جدید بین سرورها برای جمع پهنای باند (پیش‌فرض: روشن)</div>
        </div>
        <label class="switch">
          <input type="checkbox" id="sp-smart" ${speed.smart_split ? 'checked' : ''}>
          <span class="slider"></span>
        </label>
      </div>

      <div style="padding:16px; background:linear-gradient(135deg, rgba(0,255,209,0.04), rgba(124,92,255,0.04)); border-radius:14px; border:1px solid rgba(0,255,209,0.2);">
        <div style="display:flex; align-items:center; gap:16px; margin-bottom:14px;">
          <div style="width:44px; height:44px; border-radius:12px; background:rgba(0,255,209,0.15); display:grid; place-items:center; color:var(--accent); font-size:18px;">
            <i class="fa-solid fa-down-long"></i>
          </div>
          <div style="flex:1;">
            <div style="font-weight:600; font-size:14px;">Chunk Downloader <span style="color:var(--accent); font-size:10px; background:rgba(0,255,209,0.1); padding:2px 8px; border-radius:6px; margin-right:6px;">PRO</span></div>
            <div style="color:var(--text-3); font-size:12px; margin-top:2px;">دانلود مستقیم فایل‌های بزرگ از چند سرور همزمان (Range Requests)</div>
          </div>
          <label class="switch">
            <input type="checkbox" id="sp-chunk" ${speed.chunk_downloader ? 'checked' : ''}>
            <span class="slider"></span>
          </label>
        </div>
        <div class="grid-2" style="gap:12px;">
          <div class="form-field">
            <label>حداقل سایز فایل (MB)</label>
            <input type="number" id="sp-chunk-min" value="${Math.round((speed.chunk_min_bytes||0)/1024/1024)}" min="1">
          </div>
          <div class="form-field">
            <label>تعداد تکه‌های موازی</label>
            <input type="number" id="sp-chunk-par" value="${speed.chunk_parallelism}" min="2" max="16">
          </div>
        </div>
      </div>
    </div>

    <div style="margin-top:20px; display:flex; gap:10px; justify-content:flex-end;">
      <button class="btn-primary" id="sp-save"><i class="fa-solid fa-check"></i> ذخیره تنظیمات</button>
    </div>
  `;
  view.appendChild(panel);

  $('#sp-save').onclick = async () => {
    try {
      const payload = {
        sticky_by_domain:  $('#sp-sticky').checked,
        sticky_ttl_sec:    parseInt($('#sp-ttl').value, 10) || 60,
        smart_split:       $('#sp-smart').checked,
        chunk_downloader:  $('#sp-chunk').checked,
        chunk_min_bytes:   (parseInt($('#sp-chunk-min').value, 10) || 50) * 1024 * 1024,
        chunk_parallelism: parseInt($('#sp-chunk-par').value, 10) || 4,
      };
      await api.post('/api/speed', payload);
      toast('تنظیمات سرعت ذخیره شد', 'ok');
    } catch (e) { toast(e.message, 'err'); }
  };
}

// ============================================================================
// PAGE: PROFILES
// ============================================================================
async function renderProfiles() {
  const list = await api.get('/api/profiles');
  state.profiles = list || [];
  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">پروفایل‌ها</h3>
          <div class="panel-sub">// ${persianNum(state.profiles.length)} پروفایل · امکان داشتن چندین setup مختلف</div>
        </div>
        <div class="panel-actions">
          <button class="btn-primary" onclick="openProfileModal()"><i class="fa-solid fa-plus"></i> پروفایل جدید</button>
        </div>
      </div>
      <div id="profiles-body"></div>
    </div>
  `;
  drawProfiles();
}

function drawProfiles() {
  const body = $('#profiles-body');
  if (!state.profiles.length) {
    body.innerHTML = `<div class="empty-state"><i class="fa-solid fa-layer-group"></i><h3>هنوز پروفایلی نداری</h3><p>پروفایل‌ها برای تعریف چند سناریوی مختلف هستن (مثلاً: Gaming، Streaming).</p></div>`;
    return;
  }
  body.innerHTML = state.profiles.map(p => `
    <div style="display:flex; align-items:center; gap:14px; padding:14px 16px; background: var(--glass); border:1px solid var(--border); border-radius:14px; margin-bottom:10px;">
      <div style="width:42px; height:42px; border-radius:12px; background: linear-gradient(135deg, var(--accent-2), var(--accent-3)); display:grid; place-items:center; font-size:18px; box-shadow: inset 0 1px 0 rgba(255,255,255,0.2);"><i class="fa-solid fa-layer-group"></i></div>
      <div style="flex:1;">
        <b style="display:block;">${escHTML(p.name)}</b>
        <div style="font-size:11px; color:var(--text-3); font-family:'JetBrains Mono', monospace; margin-top: 4px;">${persianNum((p.server_ids || []).length)} سرور · الگوریتم: ${escHTML(p.algorithm || '—')}</div>
      </div>
      ${p.active ? '<span class="status online">ACTIVE</span>' : '<button class="btn" onclick="activateProfile(\''+p.id+'\')">فعال‌سازی</button>'}
      <div class="row-btn" onclick="openProfileModal('${p.id}')" title="ویرایش"><i class="fa-solid fa-pen"></i></div>
      <div class="row-btn danger" onclick="deleteProfile('${p.id}')" title="حذف"><i class="fa-solid fa-trash"></i></div>
    </div>
  `).join('');
}

function openProfileModal(id) {
  const p = id ? state.profiles.find(x => x.id === id) : null;
  const body = el('div', {});
  const checked = (p && p.server_ids) || [];
  body.innerHTML = `
    <div class="form-row">
      <div class="form-field">
        <label>نام پروفایل</label>
        <input id="prof-name" type="text" value="${p ? escHTML(p.name) : ''}" placeholder="مثال: Gaming Setup"/>
      </div>
      <div class="form-field">
        <label>الگوریتم</label>
        <select id="prof-algo">
          <option value="">پیش‌فرض</option>
          ${(state.algorithms || []).map(a => `<option value="${a.id}" ${p && p.algorithm === a.id ? 'selected' : ''}>${escHTML(a.name)}</option>`).join('')}
        </select>
      </div>
    </div>
    <div class="form-field">
      <label>سرورهای فعال در این پروفایل</label>
      <div style="max-height: 240px; overflow-y: auto; padding: 4px; border-radius: 10px; background: var(--bg-1);">
        ${state.servers.map(s => `
          <label style="display:flex; align-items:center; gap:10px; padding:10px 12px; cursor:pointer; border-radius:8px;" onmouseover="this.style.background='var(--glass)'" onmouseout="this.style.background=''">
            <input type="checkbox" name="prof-srv" value="${s.id}" ${checked.includes(s.id) ? 'checked' : ''} style="width:auto;"/>
            <span class="flag-3d" style="width: 26px; height: 26px; font-size: 13px;">${s.flag}</span>
            <b style="font-size:13px; flex:1;">${escHTML(s.name)}</b>
            <span class="status ${s.status || 'unknown'}" style="font-size:9px; padding: 2px 7px;">${s.status || '?'}</span>
          </label>
        `).join('')}
      </div>
    </div>
  `;
  openModal(p ? 'ویرایش پروفایل' : 'پروفایل جدید', body, [
    el('button', { class: 'btn', onclick: closeModal }, ['انصراف']),
    el('button', { class: 'btn-primary', onclick: () => saveProfile(id) }, ['ذخیره']),
  ]);
}

async function saveProfile(id) {
  const checks = $$('input[name="prof-srv"]:checked').map(c => c.value);
  const data = {
    id: id || '',
    name: $('#prof-name').value || 'پروفایل',
    server_ids: checks,
    algorithm: $('#prof-algo').value,
    active: false,
  };
  try {
    if (id) await api.put('/api/profiles/' + id, data);
    else await api.post('/api/profiles', data);
    toast('ذخیره شد', 'ok');
    closeModal();
    renderProfiles();
  } catch (e) { toast(e.message, 'err'); }
}

async function activateProfile(id) {
  // Activate by setting active=true on this and false on others, plus set algo on app
  const target = state.profiles.find(p => p.id === id);
  if (!target) return;
  try {
    for (const p of state.profiles) {
      const updated = { ...p, active: p.id === id };
      await api.put('/api/profiles/' + p.id, updated);
    }
    if (target.algorithm) await api.post('/api/algorithm', { algorithm: target.algorithm });
    toast('پروفایل فعال شد: ' + target.name, 'ok');
    renderProfiles();
  } catch (e) { toast(e.message, 'err'); }
}

async function deleteProfile(id) {
  confirmModal('حذف پروفایل', 'مطمئنی؟', async () => {
    try { await api.del('/api/profiles/' + id); toast('حذف شد', 'ok'); renderProfiles(); } catch (e) { toast(e.message, 'err'); }
  });
}

// ============================================================================
// PAGE: SCHEDULE
// ============================================================================
async function renderSchedule() {
  const list = await api.get('/api/schedules');
  state.schedules = list || [];
  if (!state.profiles.length) {
    try { state.profiles = await api.get('/api/profiles') || []; } catch(e){}
  }
  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">زمانبندی پروفایل‌ها</h3>
          <div class="panel-sub">// تعویض خودکار بر اساس ساعت روز</div>
        </div>
        <div class="panel-actions">
          <button class="btn-primary" onclick="openScheduleModal()"><i class="fa-solid fa-plus"></i> زمانبندی جدید</button>
        </div>
      </div>
      <div id="sched-body"></div>
    </div>
  `;
  drawSchedules();
}

function drawSchedules() {
  const body = $('#sched-body');
  if (!state.schedules.length) {
    body.innerHTML = `<div class="empty-state"><i class="fa-solid fa-clock"></i><h3>هنوز زمانبندی نداری</h3><p>مثلاً: ساعت ۸ تا ۱۸ پروفایل کاری، شب پروفایل گیمینگ.</p></div>`;
    return;
  }
  body.innerHTML = state.schedules.map(s => {
    const prof = state.profiles.find(p => p.id === s.profile_id);
    return `
      <div style="display:flex; align-items:center; gap:16px; padding:14px 16px; background: var(--glass); border:1px solid var(--border); border-radius:14px; margin-bottom:10px;">
        <i class="fa-solid fa-clock" style="font-size:20px; color: var(--accent);"></i>
        <div style="flex:1;">
          <b>${escHTML(s.name)}</b>
          <div style="font-size:11px; color:var(--text-3); font-family:'JetBrains Mono', monospace; margin-top:4px;">
            از ${persianNum(s.from)} تا ${persianNum(s.to)} → پروفایل: ${escHTML(prof ? prof.name : '—')}
          </div>
        </div>
        <label class="switch"><input type="checkbox" ${s.enabled ? 'checked' : ''}/><span class="slider"></span></label>
      </div>
    `;
  }).join('');
}

function openScheduleModal() {
  if (!state.profiles.length) {
    toast('اول یک پروفایل بساز', 'err');
    return;
  }
  const body = el('div', {});
  body.innerHTML = `
    <div class="form-row">
      <div class="form-field">
        <label>نام</label>
        <input id="sch-name" type="text" placeholder="ساعات کاری"/>
      </div>
      <div class="form-field">
        <label>پروفایل</label>
        <select id="sch-prof">
          ${state.profiles.map(p => `<option value="${p.id}">${escHTML(p.name)}</option>`).join('')}
        </select>
      </div>
    </div>
    <div class="form-row">
      <div class="form-field">
        <label>از ساعت</label>
        <input id="sch-from" type="text" placeholder="08:00" value="08:00"/>
      </div>
      <div class="form-field">
        <label>تا ساعت</label>
        <input id="sch-to" type="text" placeholder="18:00" value="18:00"/>
      </div>
    </div>
  `;
  openModal('زمانبندی جدید', body, [
    el('button', { class: 'btn', onclick: closeModal }, ['انصراف']),
    el('button', { class: 'btn-primary', onclick: saveSchedule }, ['ذخیره']),
  ]);
}

async function saveSchedule() {
  const data = {
    name: $('#sch-name').value || 'زمانبندی',
    from: $('#sch-from').value || '00:00',
    to: $('#sch-to').value || '23:59',
    profile_id: $('#sch-prof').value,
    enabled: true,
  };
  try { await api.post('/api/schedules', data); toast('ذخیره شد', 'ok'); closeModal(); renderSchedule(); } catch (e) { toast(e.message, 'err'); }
}

// ============================================================================
// PAGE: ALERTS
// ============================================================================
async function renderAlerts() {
  state.alerts = await api.get('/api/alerts');
  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">تاریخچه هشدارها</h3>
          <div class="panel-sub">// ${persianNum(state.alerts.length)} مورد · ۲۰۰ تا آخرین</div>
        </div>
        <div class="panel-actions">
          <div class="chip active" data-level="all">همه</div>
          <div class="chip" data-level="crit">بحرانی</div>
          <div class="chip" data-level="warn">هشدار</div>
          <div class="chip" data-level="info">اطلاع</div>
        </div>
      </div>
      <div id="alerts-body"></div>
    </div>
  `;
  let lvl = 'all';
  view.querySelectorAll('.chip[data-level]').forEach(c => {
    c.addEventListener('click', () => {
      view.querySelectorAll('.chip[data-level]').forEach(x => x.classList.remove('active'));
      c.classList.add('active');
      lvl = c.dataset.level;
      drawAlertsList(lvl);
    });
  });
  drawAlertsList(lvl);
}

function drawAlertsList(lvl) {
  const body = $('#alerts-body');
  if (!body) return;
  let alerts = state.alerts;
  if (lvl !== 'all') alerts = alerts.filter(a => a.level === lvl);
  if (!alerts.length) {
    body.innerHTML = `<div class="empty-state"><i class="fa-solid fa-bell-slash"></i><h3>هشداری وجود نداره</h3></div>`;
    return;
  }
  renderAlertList(body, alerts);
}

function liveUpdateAlerts() {
  const body = document.getElementById('alerts-body');
  if (!body) return;
  const lvl = document.querySelector('.chip[data-level].active');
  drawAlertsList(lvl ? lvl.dataset.level : 'all');
}

function renderAlertList(container, list) {
  container.innerHTML = list.map(a => {
    const cls = a.level === 'crit' ? 'crit' : a.level === 'warn' ? 'warn' : 'info';
    const icn = a.level === 'crit' ? 'fa-circle-exclamation' :
                a.level === 'warn' ? 'fa-triangle-exclamation' : 'fa-circle-info';
    const ago = timeAgo(a.time);
    return `
      <div class="alert-item">
        <div class="alert-icon ${cls}"><i class="fa-solid ${icn}"></i></div>
        <div class="alert-text">
          <b>${escHTML(a.title)}</b>
          <small>${escHTML(a.message)} · ${ago}</small>
        </div>
      </div>
    `;
  }).join('');
}

function timeAgo(t) {
  if (!t) return '—';
  const d = new Date(t);
  const s = Math.round((Date.now() - d.getTime()) / 1000);
  if (s < 60)  return persianNum(s) + ' ثانیه قبل';
  if (s < 3600) return persianNum(Math.floor(s / 60)) + ' دقیقه قبل';
  if (s < 86400) return persianNum(Math.floor(s / 3600)) + ' ساعت قبل';
  return persianNum(Math.floor(s / 86400)) + ' روز قبل';
}

// ============================================================================
// PAGE: DNS
// ============================================================================
async function renderDNS() {
  const dns = await api.get('/api/dns');
  state.dns = dns || { servers: [], leak_protect: true, block_malware: false };
  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">پیکربندی DNS</h3>
          <div class="panel-sub">// resolvers · privacy · malware blocking</div>
        </div>
      </div>
      <div class="form-field" style="margin-bottom:18px;">
        <label>سرورهای DNS (هر خط یکی)</label>
        <textarea id="dns-servers" placeholder="1.1.1.1&#10;8.8.8.8">${(state.dns.servers || []).join('\n')}</textarea>
      </div>
      <div style="display:flex; align-items:center; gap:14px; padding:14px; background: var(--glass); border-radius:12px; margin-bottom:10px;">
        <i class="fa-solid fa-shield-halved" style="color: var(--accent); font-size: 18px;"></i>
        <div style="flex:1;">
          <b style="display:block;">حفاظت از نشتی DNS (DNS Leak Protect)</b>
          <small style="color:var(--text-3); font-family:'JetBrains Mono', monospace; font-size: 11px;">جلوگیری از ارسال DNS query به DNS سیستمی</small>
        </div>
        <label class="switch"><input id="dns-leak" type="checkbox" ${state.dns.leak_protect ? 'checked' : ''}/><span class="slider"></span></label>
      </div>
      <div style="display:flex; align-items:center; gap:14px; padding:14px; background: var(--glass); border-radius:12px; margin-bottom:18px;">
        <i class="fa-solid fa-bug-slash" style="color: var(--accent-3); font-size: 18px;"></i>
        <div style="flex:1;">
          <b style="display:block;">مسدودسازی Malware</b>
          <small style="color:var(--text-3); font-family:'JetBrains Mono', monospace; font-size: 11px;">استفاده از blocklist های شناخته‌شده</small>
        </div>
        <label class="switch"><input id="dns-malware" type="checkbox" ${state.dns.block_malware ? 'checked' : ''}/><span class="slider"></span></label>
      </div>
      <button class="btn-primary" onclick="saveDNS()"><i class="fa-solid fa-save"></i> ذخیره</button>
    </div>
  `;
}

async function saveDNS() {
  const data = {
    servers: $('#dns-servers').value.split('\n').map(s => s.trim()).filter(Boolean),
    leak_protect: $('#dns-leak').checked,
    block_malware: $('#dns-malware').checked,
  };
  try { await api.put('/api/dns', data); toast('ذخیره شد', 'ok'); } catch (e) { toast(e.message, 'err'); }
}

// ============================================================================
// PAGE: API & WEBHOOK
// ============================================================================
async function renderAPI() {
  const [keyR, hookR] = await Promise.all([
    api.get('/api/apikey').catch(() => ({ api_key: '' })),
    api.get('/api/webhook').catch(() => ({ webhook: '' })),
  ]);
  state.apiKey = keyR.api_key || '';
  state.webhook = hookR.webhook || '';

  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">API Key</h3>
          <div class="panel-sub">// برای اتوماسیون و دسترسی برنامه‌ای</div>
        </div>
      </div>
      <div class="form-field">
        <label>کلید فعلی</label>
        <div style="display:flex; gap:10px; align-items:center;">
          <input id="api-key" type="text" value="${escHTML(state.apiKey)}" readonly placeholder="هنوز ساخته نشده — روی 'Generate' کلیک کنید"/>
          <button class="btn" onclick="copyAPIKey()"><i class="fa-solid fa-copy"></i></button>
          <button class="btn-primary" onclick="genAPIKey()"><i class="fa-solid fa-key"></i> Generate</button>
        </div>
      </div>
      <div style="margin-top: 14px; padding: 12px; background: var(--bg-1); border-radius: 10px; font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--text-2); line-height: 1.7;">
        <b style="color:var(--accent);">نمونه استفاده:</b><br>
        $ curl -H "X-API-Key: &lt;your-key&gt;" http://localhost:8088/api/overview
      </div>
    </div>

    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">Webhook</h3>
          <div class="panel-sub">// ارسال هشدارها به URL خارجی (Discord/Telegram/...)</div>
        </div>
      </div>
      <div class="form-field">
        <label>Webhook URL</label>
        <input id="webhook-url" type="url" value="${escHTML(state.webhook)}" placeholder="https://discord.com/api/webhooks/..."/>
      </div>
      <div style="margin-top: 14px;">
        <button class="btn-primary" onclick="saveWebhook()"><i class="fa-solid fa-save"></i> ذخیره</button>
      </div>
    </div>
  `;
}

async function genAPIKey() {
  try {
    const r = await api.post('/api/apikey', {});
    state.apiKey = r.api_key;
    $('#api-key').value = r.api_key;
    toast('کلید جدید ساخته شد', 'ok');
  } catch (e) { toast(e.message, 'err'); }
}

function copyAPIKey() {
  const inp = $('#api-key');
  if (!inp.value) return;
  navigator.clipboard.writeText(inp.value).then(() => toast('کپی شد', 'ok'));
}

async function saveWebhook() {
  try { await api.put('/api/webhook', { webhook: $('#webhook-url').value }); toast('ذخیره شد', 'ok'); } catch (e) { toast(e.message, 'err'); }
}

// ============================================================================
// PAGE: LOGS
// ============================================================================
async function renderLogs() {
  const logs = await api.get('/api/logs');
  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">لاگ‌ها</h3>
          <div class="panel-sub">// آخرین ${persianNum(logs.length)} رویداد</div>
        </div>
        <div class="panel-actions">
          <input id="log-search" placeholder="جستجو در پیام..." style="width:200px; padding:6px 12px; font-size:12px;"/>
          <div class="chip active" data-level="all">همه</div>
          <div class="chip" data-level="info">info</div>
          <div class="chip" data-level="warn">warn</div>
          <div class="chip" data-level="error">error</div>
          <button class="btn" onclick="renderLogs()"><i class="fa-solid fa-rotate"></i></button>
        </div>
      </div>
      <div id="log-body" style="max-height: 600px; overflow-y: auto;"></div>
    </div>
  `;
  let lvl = 'all', search = '';
  view.querySelectorAll('.chip[data-level]').forEach(c => {
    c.addEventListener('click', () => {
      view.querySelectorAll('.chip[data-level]').forEach(x => x.classList.remove('active'));
      c.classList.add('active');
      lvl = c.dataset.level;
      drawLogs(logs, lvl, search);
    });
  });
  view.querySelector('#log-search').addEventListener('input', e => {
    search = e.target.value.toLowerCase();
    drawLogs(logs, lvl, search);
  });
  drawLogs(logs, lvl, search);
}

function drawLogs(logs, lvl, search) {
  let rows = logs;
  if (lvl !== 'all') rows = rows.filter(l => l.level === lvl);
  if (search) rows = rows.filter(l => (l.message || '').toLowerCase().includes(search));
  const body = $('#log-body');
  if (!rows.length) { body.innerHTML = `<div class="empty-state"><i class="fa-solid fa-file-lines"></i><h3>چیزی پیدا نشد</h3></div>`; return; }
  body.innerHTML = rows.map(l => {
    const t = l.time ? new Date(l.time) : new Date();
    const ts = t.toLocaleTimeString('fa-IR', { hour12: false });
    return `
      <div class="log-row">
        <div class="log-time">${ts}</div>
        <div class="log-level ${l.level}">${l.level}</div>
        <div class="log-source">${escHTML(l.source || '—')}</div>
        <div class="log-msg">${escHTML(l.message || '')}</div>
      </div>
    `;
  }).join('');
}

// ============================================================================
// PAGE: SETTINGS
// ============================================================================
async function renderSettings() {
  const listen = await api.get('/api/listen');
  state.listen = listen;
  const view = $('#view');
  view.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">پورت‌های Listen</h3>
          <div class="panel-sub">// where the proxy server listens</div>
        </div>
      </div>
      <div class="form-row">
        <div class="form-field">
          <label>HTTP Proxy Port</label>
          <input id="set-http" type="number" value="${listen.http_port || 10809}"/>
        </div>
        <div class="form-field">
          <label>SOCKS Proxy Port</label>
          <input id="set-socks" type="number" value="${listen.socks_port || 10808}"/>
        </div>
      </div>
      <div class="form-row">
        <div class="form-field">
          <label>آدرس Bind <small style="opacity:.6">(127.0.0.1 = فقط لوکال · 0.0.0.0 = شبکه)</small></label>
          <input id="set-addr" type="text" value="${listen.address || '127.0.0.1'}" placeholder="127.0.0.1"/>
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head">
        <div>
          <h3 class="panel-title">رابط TUN (سیستم‌گرد)</h3>
          <div class="panel-sub">// transparent proxy via TUN device</div>
        </div>
      </div>
      <div style="display:flex; align-items:center; gap:14px; padding:14px; background: var(--glass); border-radius:12px; margin-bottom:14px;">
        <i class="fa-solid fa-network-wired" style="color: var(--accent-2); font-size: 18px;"></i>
        <div style="flex:1;">
          <b style="display:block;">فعال‌سازی TUN</b>
          <small style="color:var(--text-3); font-family:'JetBrains Mono', monospace; font-size: 11px;">نیاز به اجرا با دسترسی root</small>
        </div>
        <label class="switch"><input id="set-tun" type="checkbox" ${listen.tun_enable ? 'checked' : ''}/><span class="slider"></span></label>
      </div>
      <div class="form-row">
        <div class="form-field">
          <label>نام Interface</label>
          <input id="set-tun-name" type="text" value="${listen.tun_name || 'outb0'}"/>
        </div>
        <div class="form-field">
          <label>IP/CIDR</label>
          <input id="set-tun-addr" type="text" value="${listen.tun_addr || '10.10.0.1/24'}"/>
        </div>
      </div>
    </div>

    <div style="display: flex; gap: 12px; align-items: center;">
      <button class="btn-primary" onclick="saveSettings()"><i class="fa-solid fa-save"></i> ذخیره و اعمال</button>
      <button class="btn" onclick="restartCore()"><i class="fa-solid fa-rotate-right"></i> ری‌استارت Core</button>
    </div>
  `;
}

async function saveSettings() {
  const data = {
    http_port: parseInt($('#set-http').value, 10),
    socks_port: parseInt($('#set-socks').value, 10),
    address: ($('#set-addr') && $('#set-addr').value) || '',
    tun_name: $('#set-tun-name').value,
    tun_addr: $('#set-tun-addr').value,
    tun_enable: $('#set-tun').checked,
  };
  try {
    await api.put('/api/listen', data);
    toast('ذخیره شد · پورت‌ها روی هسته اعمال شدن', 'ok');
  } catch (e) { toast(e.message, 'err'); }
}

async function restartCore() {
  try { await api.get('/api/apply'); toast('Core ری‌استارت شد', 'ok'); } catch (e) { toast(e.message, 'err'); }
}

// ============================================================================
// PAGE: BACKUP
// ============================================================================
async function renderBackup() {
  const view = $('#view');
  view.innerHTML = `
    <div class="grid-2">
      <div class="widget">
        <div class="widget-head">
          <i class="fa-solid fa-download"></i>
          <div>
            <h3 class="widget-title">دانلود پشتیبان</h3>
            <div class="panel-sub">// تمام تنظیمات در یک فایل JSON</div>
          </div>
        </div>
        <p style="color: var(--text-2); font-size: 13px; line-height: 1.7; margin: 12px 0;">
          فایل پشتیبان شامل تمام سرورها، قوانین، پروفایل‌ها، DNS و تنظیمات سیستم است. قبل از تغییرات بزرگ، حتماً پشتیبان بگیرید.
        </p>
        <a class="btn-primary" href="/api/backup" download style="text-decoration:none; justify-content: center;">
          <i class="fa-solid fa-cloud-arrow-down"></i> دانلود فایل JSON
        </a>
      </div>

      <div class="widget">
        <div class="widget-head">
          <i class="fa-solid fa-upload"></i>
          <div>
            <h3 class="widget-title">بازیابی</h3>
            <div class="panel-sub">// جایگزینی کامل تنظیمات</div>
          </div>
        </div>
        <p style="color: var(--text-2); font-size: 13px; line-height: 1.7; margin: 12px 0;">
          ⚠ بازیابی، تنظیمات فعلی را به طور کامل بازنویسی می‌کند. مطمئن شوید فایل از همین نسخه است.
        </p>
        <input id="restore-file" type="file" accept=".json" style="margin-bottom: 10px;"/>
        <button class="btn-primary" onclick="doRestore()" style="justify-content: center; width:100%;">
          <i class="fa-solid fa-cloud-arrow-up"></i> آپلود و بازیابی
        </button>
      </div>
    </div>
  `;
}

async function doRestore() {
  const f = $('#restore-file').files[0];
  if (!f) { toast('فایلی انتخاب نکردی', 'err'); return; }
  const text = await f.text();
  confirmModal('بازیابی', 'تنظیمات فعلی به طور کامل بازنویسی می‌شود. ادامه؟', async () => {
    try { await api.post('/api/restore', text); toast('بازیابی شد', 'ok'); setTimeout(() => location.reload(), 800); } catch (e) { toast(e.message, 'err'); }
  });
}

// ============================================================================
// Bootstrap
// ============================================================================
async function bootstrap() {
  // load algorithms (used widely)
  try { state.algorithms = await api.get('/api/algorithms'); } catch (e) {}

  // initial system info
  try {
    const sys = await api.get('/api/system');
    state.system = sys;
    $('#sb-core').textContent = sys.core || 'Xray';
    $('#sb-go').textContent = persianNum(sys.goroutines || 0);
    $('#sb-ram').textContent = persianNum((sys.alloc_mb || 0).toFixed(1)) + ' MB';
    if (sys.xray_disabled) $('#sb-status').innerHTML = '<span style="color:var(--warn);">controller-only</span>';
  } catch (e) {}
  try {
    const listen = await api.get('/api/listen');
    $('#sb-socks').textContent = ':' + persianNum(listen.socks_port);
    $('#sb-http').textContent = ':' + persianNum(listen.http_port);
  } catch (e) {}

  setInterval(async () => {
    try {
      const sys = await api.get('/api/system');
      $('#sb-go').textContent = persianNum(sys.goroutines || 0);
      $('#sb-ram').textContent = persianNum((sys.alloc_mb || 0).toFixed(1)) + ' MB';
    } catch (e) {}
  }, 5000);

  connectWS();
  await navigate();
}

// keyboard shortcut
window.addEventListener('keydown', (e) => {
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    e.preventDefault();
    document.getElementById('search-input').focus();
  }
  if (e.key === 'Escape') closeModal();
});

bootstrap();
