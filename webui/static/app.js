'use strict';

const RPC_BASE = window.location.origin;
let refreshTimer = null;
let refreshInterval = 2000;
let speedHistory = [];
const MAX_HISTORY = 60;
let chartCtx = null;
let sidebarOpen = false;
let knownFiles = {};
let logSource = null;
let logReconnectTimer = null;
let logPaused = false;

const fileTypeIcons = {
  zip:'🗜️', rar:'🗜️', '7z':'🗜️', tar:'🗜️', gz:'🗜️', bz2:'🗜️', xz:'🗜️',
  mp4:'🎬', mkv:'🎬', avi:'🎬', mov:'🎬', webm:'🎬', ts:'🎬', flv:'🎬',
  mp3:'🎵', flac:'🎵', wav:'🎵', aac:'🎵', ogg:'🎵', opus:'🎵',
  jpg:'🖼️', jpeg:'🖼️', png:'🖼️', gif:'🖼️', webp:'🖼️', svg:'🖼️',
  pdf:'📄', doc:'📝', docx:'📝', xls:'📊', xlsx:'📊', ppt:'📊',
  exe:'⚙️', msi:'⚙️', deb:'⚙️', apk:'⚙️', dmg:'⚙️',
  iso:'💿', bin:'💿', img:'💿',
  m3u8:'📺', m3u:'📺',
  json:'🔧', xml:'🔧', csv:'🔧',
  had:'💾',
};

function getFileIcon(name) {
  if (!name) return '📁';
  const ext = name.split('.').pop().toLowerCase();
  return fileTypeIcons[ext] || '📁';
}

function getToken() {
  return localStorage.getItem('had_token') || '';
}

function tokenQS() {
  const t = getToken();
  return t ? '?token=' + encodeURIComponent(t) : '';
}

async function rpc(method, params = {}) {
  const token = getToken();
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = 'Bearer ' + token;

  const res = await fetch(RPC_BASE + '/jsonrpc', {
    method: 'POST',
    headers,
    body: JSON.stringify({ id: Date.now(), method, params }),
  });
  if (!res.ok) throw new Error('HTTP ' + res.status);
  const data = await res.json();
  if (data.error) throw new Error(data.error.message || 'RPC error');
  return data.result;
}

async function apiGet(path) {
  const token = getToken();
  const headers = {};
  if (token) headers['Authorization'] = 'Bearer ' + token;
  const res = await fetch(RPC_BASE + path, { headers });
  if (!res.ok) throw new Error('HTTP ' + res.status);
  return res.json();
}

async function apiSend(path, method, body) {
  const token = getToken();
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = 'Bearer ' + token;
  const res = await fetch(RPC_BASE + path, { method, headers, body: body ? JSON.stringify(body) : undefined });
  if (!res.ok) throw new Error('HTTP ' + res.status);
  return res.json();
}

function toast(msg, type = 'info', duration = 3500) {
  const icons = { success: '✅', error: '❌', info: 'ℹ️', warning: '⚠️' };
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.innerHTML = `<span class="toast-icon">${icons[type]}</span><span class="toast-msg">${escHtml(String(msg))}</span>`;
  document.getElementById('toast-container').appendChild(el);
  setTimeout(() => {
    el.classList.add('toast-out');
    setTimeout(() => el.remove(), 350);
  }, duration);
}

function switchTab(name, el) {
  document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
  document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
  document.getElementById('tab-' + name).classList.add('active');
  if (el) el.classList.add('active');
  if (window.innerWidth <= 768) {
    document.getElementById('sidebar').classList.remove('open');
    sidebarOpen = false;
  }
  if (name === 'sessions') refreshSessions();
  if (name === 'history') refreshHistory();
  if (name === 'tools') refreshBWSchedule();
}

function toggleSidebar() {
  sidebarOpen = !sidebarOpen;
  document.getElementById('sidebar').classList.toggle('open', sidebarOpen);
}

function setRefreshInterval(val) {
  clearInterval(refreshTimer);
  refreshInterval = parseInt(val);
  if (refreshInterval > 0) startRefresh();
}

function startRefresh() {
  if (refreshTimer) clearInterval(refreshTimer);
  if (refreshInterval > 0) {
    refreshTimer = setInterval(refreshAll, refreshInterval);
  }
}

async function refreshAll() {
  await Promise.all([refreshFiles(), refreshGlobalStat()]);
}

async function refreshFiles() {
  try {
    const files = await apiGet('/api/files');
    renderFiles(files);
    setConnected(true);
  } catch {
    setConnected(false);
  }
}

async function refreshGlobalStat() {
  try {
    const stat = await apiGet('/api/status');
    updateTopbar(stat);
    updateStatsTab(stat);
    updateSettingsStatus(stat);
  } catch {}
}

function setConnected(ok) {
  const dot = document.getElementById('conn-dot');
  const label = document.getElementById('conn-label');
  dot.className = 'conn-indicator ' + (ok ? 'online' : 'offline');
  label.textContent = ok ? 'connected' : 'offline';
}

function updateTopbar(stat) {
  if (!stat) return;
  document.getElementById('global-speed').textContent = stat.speed_human || '0 B/s';
  document.getElementById('global-eta').textContent = stat.eta || '—';
  document.getElementById('global-uptime').textContent = stat.uptime || '—';
  document.getElementById('global-active').textContent = stat.active_downloads || 0;

  const badge = document.getElementById('nav-badge-active');
  const active = stat.active_downloads || 0;
  if (active > 0) {
    badge.style.display = 'inline-flex';
    badge.textContent = active;
  } else {
    badge.style.display = 'none';
  }
}

function updateStatsTab(stat) {
  if (!stat) return;
  document.getElementById('stat-speed').textContent = stat.speed_human || '0 B/s';
  document.getElementById('stat-total-dl').textContent = stat.downloaded_size || '0 B';
  document.getElementById('stat-completed').textContent = stat.completed_files ?? '0';
  document.getElementById('stat-active').textContent = stat.active_downloads ?? '0';
  document.getElementById('stat-uptime').textContent = stat.uptime || '—';
  document.getElementById('stat-eta').textContent = stat.eta || '—';

  const pct = pctOf(stat.downloaded_bytes || 0, stat.total_size || 0);
  document.getElementById('global-bar-fill').style.width = pct + '%';
  document.getElementById('global-pct').textContent = pct.toFixed(1) + '%';

  const detail = document.getElementById('progress-detail');
  detail.textContent = `${stat.downloaded_size || '0 B'} / ${stat.total_size_human || '0 B'}`;

  const info = document.getElementById('server-info-body');
  info.innerHTML = `
    <div class="info-grid">
      <span class="info-k">Version</span><span class="info-v">${stat.version || '—'}</span>
      <span class="info-k">Speed Limit</span><span class="info-v">${stat.speed_limit ? humanBytes(stat.speed_limit) + '/s' : 'unlimited'}</span>
      <span class="info-k">Max Parallel</span><span class="info-v">${stat.max_parallel ?? '—'}</span>
      <span class="info-k">Threads</span><span class="info-v">${stat.threads ?? '—'}</span>
      <span class="info-k">Output Dir</span><span class="info-v mono">${escHtml(stat.out_dir || '.')}</span>
      <span class="info-k">Paused</span><span class="info-v">${stat.paused ? '⏸ yes' : '▶ no'}</span>
    </div>
  `;

  const spd = stat.speed || 0;
  document.getElementById('chart-current-speed').textContent = stat.speed_human || '0 B/s';
  speedHistory.push(spd);
  if (speedHistory.length > MAX_HISTORY) speedHistory.shift();
  drawChart();
}

function updateSettingsStatus(stat) {
  if (!stat) return;
  const sl = document.getElementById('status-speed-limit');
  if (sl) sl.textContent = stat.speed_limit ? humanBytes(stat.speed_limit) + '/s' : 'unlimited';
  const sp = document.getElementById('status-parallel');
  if (sp) sp.textContent = stat.max_parallel ?? '—';
  const st = document.getElementById('status-threads');
  if (st) st.textContent = stat.threads ?? '—';
  const od = document.getElementById('status-outdir');
  if (od) od.textContent = stat.out_dir || '.';
}

function pctOf(done, total) {
  if (!total || total <= 0) return 0;
  return Math.min(100, (done / total) * 100);
}

function humanBytes(n) {
  if (!n || n <= 0) return '0 B';
  const units = ['B','KB','MB','GB','TB'];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
  return n.toFixed(i === 0 ? 0 : 1) + ' ' + units[i];
}

function initChart() {
  const canvas = document.getElementById('speed-chart');
  if (!canvas) return;
  chartCtx = canvas.getContext('2d');
  resizeChart();
  window.addEventListener('resize', resizeChart);
}

function resizeChart() {
  const canvas = document.getElementById('speed-chart');
  if (!canvas || !canvas.parentElement) return;
  canvas.width = canvas.parentElement.clientWidth - 40;
  canvas.height = 100;
  drawChart();
}

function drawChart() {
  if (!chartCtx) return;
  const canvas = chartCtx.canvas;
  const W = canvas.width, H = canvas.height;
  chartCtx.clearRect(0, 0, W, H);

  if (speedHistory.length < 2) return;

  const max = Math.max(...speedHistory, 1);
  const step = W / (MAX_HISTORY - 1);

  const grad = chartCtx.createLinearGradient(0, 0, 0, H);
  grad.addColorStop(0, 'rgba(124,58,237,.45)');
  grad.addColorStop(1, 'rgba(124,58,237,.02)');

  chartCtx.beginPath();
  speedHistory.forEach((v, i) => {
    const x = i * step;
    const y = H - (v / max) * (H - 10) - 4;
    i === 0 ? chartCtx.moveTo(x, y) : chartCtx.lineTo(x, y);
  });

  chartCtx.lineTo((speedHistory.length - 1) * step, H);
  chartCtx.lineTo(0, H);
  chartCtx.closePath();
  chartCtx.fillStyle = grad;
  chartCtx.fill();

  chartCtx.beginPath();
  speedHistory.forEach((v, i) => {
    const x = i * step;
    const y = H - (v / max) * (H - 10) - 4;
    i === 0 ? chartCtx.moveTo(x, y) : chartCtx.lineTo(x, y);
  });
  chartCtx.strokeStyle = '#7c3aed';
  chartCtx.lineWidth = 2;
  chartCtx.lineJoin = 'round';
  chartCtx.stroke();

  chartCtx.strokeStyle = 'rgba(255,255,255,.05)';
  chartCtx.lineWidth = 1;
  [0.25, 0.5, 0.75].forEach(f => {
    const y = H - f * (H - 10) - 4;
    chartCtx.beginPath();
    chartCtx.moveTo(0, y);
    chartCtx.lineTo(W, y);
    chartCtx.stroke();
    chartCtx.fillStyle = 'rgba(255,255,255,.25)';
    chartCtx.font = '10px JetBrains Mono, monospace';
    chartCtx.fillText(humanBytes(max * f) + '/s', 4, y - 2);
  });
}

function renderFiles(files) {
  const list = document.getElementById('file-list');
  const empty = document.getElementById('empty-state');

  if (!files || files.length === 0) {
    list.innerHTML = '';
    empty.style.display = 'flex';
    updateGlobalProgress([]);
    return;
  }
  empty.style.display = 'none';

  const existing = {};
  list.querySelectorAll('.file-card[data-name]').forEach(el => {
    existing[el.dataset.name] = el;
  });

  const seen = new Set();
  files.forEach(f => {
    seen.add(f.name);
    knownFiles[f.name] = f.url || knownFiles[f.name] || '';
    const pct = Math.min(100, f.progress || 0);
    let card = existing[f.name];
    if (!card) {
      card = document.createElement('div');
      card.className = 'file-card';
      card.dataset.name = f.name;
      list.appendChild(card);
    }
    updateFileCard(card, f, pct);
  });

  list.querySelectorAll('.file-card[data-name]').forEach(el => {
    if (!seen.has(el.dataset.name)) {
      el.classList.add('card-removing');
      setTimeout(() => el.remove(), 300);
    }
  });

  updateGlobalProgress(files);
}

function updateFileCard(card, f, pct) {
  const statusClass = 'status-' + (f.status || 'pending');
  card.className = 'file-card ' + statusClass;

  const badgeClass = {
    downloading: 'badge-downloading',
    downloaded:  'badge-downloaded',
    error:       'badge-error',
    pending:     'badge-pending',
    hls:         'badge-hls',
    queued:      'badge-queued',
    paused:      'badge-paused',
  }[f.status] || 'badge-pending';

  const isPulsing = f.status === 'downloading' || f.status === 'hls';
  const dotHtml = isPulsing ? '<span class="dot-pulse"></span>' : '';
  const icon = getFileIcon(f.name);
  const speed = f.speed || '';

  let threadsBars = '';
  if (f.threads > 0) {
    const total = Math.min(f.threads, 32);
    const bars = Array.from({ length: total }, (_, i) => {
      const done = i < f.done_threads;
      const active = !done && i < f.active_threads;
      const cls = done ? 'bar-done' : active ? 'bar-active' : '';
      const w = done ? 100 : active ? 60 : 0;
      return `<div class="thread-bar"><div class="thread-bar-fill ${cls}" style="width:${w}%"></div></div>`;
    }).join('');
    threadsBars = `<div class="thread-bars" title="${f.done_threads}/${f.threads} threads done">${bars}</div>`;
  }

  const fillClass = f.status === 'downloaded' ? 'complete' : f.status === 'error' ? 'err' : '';

  card.innerHTML = `
    <div class="file-header">
      <div class="file-type-icon">${icon}</div>
      <div class="file-meta">
        <div class="file-name" title="${escHtml(f.name)}">${escHtml(f.name)}</div>
        <div class="file-info">
          <span>${f.size_human || '?'}</span>
          <span>${f.done_human || f.downloaded_human || '0 B'}</span>
          ${speed ? `<span class="file-speed">${escHtml(speed)}</span>` : ''}
          <span class="status-badge ${badgeClass}">${dotHtml}${f.status || 'pending'}</span>
        </div>
      </div>
      <div class="file-actions">
        ${renderFileActions(f)}
      </div>
    </div>
    <div class="file-progress-wrap">
      <div class="file-bar-track">
        <div class="file-bar-fill ${fillClass}" style="width:${pct}%">
          ${isPulsing ? '<div class="bar-glow"></div>' : ''}
        </div>
      </div>
      <span class="file-pct">${pct.toFixed(1)}%</span>
    </div>
    ${threadsBars}
  `;
}

function renderFileActions(f) {
  const name = escHtml(f.name).replace(/'/g, "\\'");
  const btns = [];

  if (f.status === 'downloading' || f.status === 'hls') {
    btns.push(`<button class="file-action-btn" onclick="pauseFile('${name}')" title="Pause">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>
    </button>`);
  }
  if (f.status === 'paused') {
    btns.push(`<button class="file-action-btn" onclick="resumeFile('${name}')" title="Resume">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
    </button>`);
  }
  if (f.status === 'error') {
    btns.push(`<button class="file-action-btn retry" onclick="retryFile('${name}')" title="Retry">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/></svg>
    </button>`);
  }

  btns.push(`<button class="file-action-btn remove" onclick="removeFileByName('${name}')" title="Remove">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
  </button>`);

  return btns.join('');
}

function updateGlobalProgress(files) {
  if (!files || files.length === 0) {
    document.getElementById('global-bar-fill').style.width = '0%';
    document.getElementById('global-pct').textContent = '0%';
    return;
  }
  let totalSize = 0, totalDone = 0;
  files.forEach(f => {
    totalSize += f.size || 0;
    totalDone += f.done || 0;
  });
  const pct = pctOf(totalDone, totalSize);
  document.getElementById('global-bar-fill').style.width = pct + '%';
  document.getElementById('global-pct').textContent = pct.toFixed(1) + '%';
}

async function addDownload() {
  const input = document.getElementById('url-input');
  const raw = input.value.trim();
  if (!raw) { toast('Paste a URL first', 'warning'); return; }

  const urls = raw.split('\n').map(u => u.trim()).filter(u => u.startsWith('http') || u.startsWith('ftp') || u.startsWith('sftp') || u.startsWith('magnet:'));
  if (urls.length === 0) { toast('No valid URLs found', 'warning'); return; }

  const threads = parseInt(document.getElementById('opt-threads').value) || 8;
  const speed = parseInt(document.getElementById('opt-speed').value) || 0;
  const outdir = document.getElementById('opt-outdir').value.trim();

  const btn = document.getElementById('add-btn');
  btn.disabled = true;
  btn.innerHTML = '<span class="dot-pulse" style="background:#fff;display:inline-block"></span> Adding…';

  try {
    if (speed > 0) await rpc('had.setSpeedLimit', { speed });
    if (outdir) await rpc('had.setOutDir', { dir: outdir });
    await rpc('had.setThreads', { threads });

    await rpc('had.addUri', { urls });

    toast(`${urls.length} download${urls.length > 1 ? 's' : ''} queued`, 'success');
    input.value = '';
    document.getElementById('url-preview').style.display = 'none';
    switchTab('downloads', document.querySelector('[data-tab=downloads]'));
    setTimeout(refreshAll, 600);
  } catch (e) {
    toast('Error: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v14M8 12l4 4 4-4M4 20h16"/></svg> Start Download`;
  }
}

function onURLInput(el) {
  const raw = el.value.trim();
  const urls = raw.split('\n').map(u => u.trim()).filter(Boolean);
  const preview = document.getElementById('url-preview');

  if (urls.length > 1) {
    preview.style.display = 'block';
    preview.innerHTML = `<div class="url-preview-count">${urls.length} URLs detected</div>` +
      urls.slice(0, 8).map(u =>
        `<div class="url-preview-item"><span class="bullet">›</span><span>${escHtml(u.length > 80 ? u.slice(0, 77) + '…' : u)}</span></div>`
      ).join('') +
      (urls.length > 8 ? `<div class="url-preview-item"><span class="bullet">›</span><span style="opacity:.5">…and ${urls.length - 8} more</span></div>` : '');
  } else {
    preview.style.display = 'none';
  }
}

async function pasteFromClipboard() {
  try {
    const text = await navigator.clipboard.readText();
    const input = document.getElementById('url-input');
    input.value = text;
    onURLInput(input);
    toast('Pasted from clipboard', 'info');
  } catch {
    toast('Clipboard access denied — paste manually', 'warning');
    document.getElementById('url-input').focus();
  }
}

function clearURL() {
  document.getElementById('url-input').value = '';
  document.getElementById('url-preview').style.display = 'none';
}

function setExample(url) {
  document.getElementById('url-input').value = url;
  document.getElementById('url-preview').style.display = 'none';
  switchTab('add', document.querySelector('[data-tab=add]'));
}

function focusURLInput() {
  switchTab('add', document.querySelector('[data-tab=add]'));
  setTimeout(() => {
    document.getElementById('url-input').focus();
    pasteFromClipboard();
  }, 100);
}

async function pauseAll() {
  try {
    await rpc('had.pauseAll');
    toast('All downloads paused', 'info');
    setTimeout(refreshAll, 400);
  } catch (e) { toast(e.message, 'error'); }
}

async function resumeAll() {
  try {
    await rpc('had.resumeAll');
    toast('Downloads resumed', 'success');
    setTimeout(refreshAll, 400);
  } catch (e) { toast(e.message, 'error'); }
}

function pauseFile(name) {
  rpc('had.pauseFile', { name }).then(() => {
    toast('Paused: ' + name, 'info');
    setTimeout(refreshAll, 400);
  }).catch(e => toast(e.message, 'error'));
}

function resumeFile(name) {
  rpc('had.resumeFile', { name }).then(() => {
    toast('Resuming: ' + name, 'success');
    setTimeout(refreshAll, 400);
  }).catch(e => toast(e.message, 'error'));
}

function retryFile(name) {
  const url = knownFiles[name];
  showModal(
    'Retry Download',
    url
      ? `Retry <strong>${escHtml(name)}</strong> from<br><span class="mono" style="font-size:.78rem">${escHtml(url)}</span>?`
      : `No source URL is known for <strong>${escHtml(name)}</strong> — re-add it manually from the Add tab.`,
    async () => {
      if (!url) { switchTab('add', document.querySelector('[data-tab=add]')); return; }
      try {
        await rpc('had.removeFile', { name }).catch(() => {});
        await rpc('had.addUri', { urls: [url] });
        toast('Retrying: ' + name, 'success');
        setTimeout(refreshAll, 500);
      } catch (e) { toast(e.message, 'error'); }
    }
  );
}

function removeFileByName(name) {
  showModal(
    'Remove Download',
    `Remove <strong>${escHtml(name)}</strong> from the list? Active download will be cancelled.`,
    async () => {
      try {
        await rpc('had.removeFile', { name });
        toast('Removed: ' + name, 'success');
        setTimeout(refreshAll, 400);
      } catch (e) {
        toast(e.message, 'error');
      }
    }
  );
}

function confirmRemoveAll() {
  showModal(
    'Remove All Downloads',
    'This will cancel and remove ALL active and queued downloads.',
    async () => {
      try {
        await rpc('had.removeAll');
        toast('All downloads removed', 'success');
        setTimeout(refreshAll, 400);
      } catch (e) {
        toast(e.message, 'error');
      }
    }
  );
}

async function startScrape() {
  const url = document.getElementById('scrape-url').value.trim();
  if (!url) { toast('Enter a URL to scrape', 'warning'); return; }

  const btn = document.getElementById('scrape-btn');
  btn.disabled = true;
  btn.innerHTML = '<span class="dot-pulse" style="background:#fff;display:inline-block"></span> Scraping…';

  addScrapeLog(`⟳ Scraping: ${url}`);

  try {
    const result = await rpc('had.scrape', { url });
    addScrapeLog(`✓ Scrape started — GID: ${result.gid}`);
    toast('Scrape started', 'success');
    setTimeout(refreshAll, 1000);
  } catch (e) {
    addScrapeLog(`✗ Error: ${e.message}`);
    toast('Scrape error: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg> Start Scrape`;
  }
}

function addScrapeLog(msg) {
  const log = document.getElementById('scrape-log');
  const body = document.getElementById('scrape-log-body');
  log.style.display = 'block';
  const line = document.createElement('div');
  line.className = 'scrape-log-line';
  line.textContent = `[${new Date().toLocaleTimeString()}] ${msg}`;
  body.appendChild(line);
  body.scrollTop = body.scrollHeight;
}

function clearScrapeLog() {
  document.getElementById('scrape-log-body').innerHTML = '';
  document.getElementById('scrape-log').style.display = 'none';
}

async function refreshSessions() {
  const list = document.getElementById('sessions-list');
  const empty = document.getElementById('sessions-empty');
  list.innerHTML = '<div class="loading-row">Loading sessions…</div>';
  try {
    const sessions = await rpc('had.listSessions');
    if (!sessions || sessions.length === 0) {
      list.innerHTML = '';
      empty.style.display = 'flex';
      return;
    }
    empty.style.display = 'none';
    list.innerHTML = sessions.map(s => `
      <div class="session-card">
        <div class="session-icon">${getFileIcon(s.file_name)}</div>
        <div class="session-meta">
          <div class="session-name" title="${escHtml(s.file_name)}">${escHtml(s.file_name)}</div>
          <div class="session-info">
            <span>${s.downloaded_human} / ${s.size_human}</span>
            <span>${s.progress.toFixed(1)}%</span>
            ${s.mirrors > 1 ? `<span>${s.mirrors} mirrors</span>` : ''}
            ${s.checksum ? `<span>🔒 ${escHtml(s.algorithm)}</span>` : ''}
          </div>
        </div>
        <div class="session-actions">
          <button class="btn btn-primary btn-sm" onclick="resumeSessionFile('${escHtml(s.file).replace(/'/g, "\\'")}')">Resume</button>
          <button class="btn btn-ghost btn-sm" onclick="deleteSessionFile('${escHtml(s.file).replace(/'/g, "\\'")}')">Delete</button>
        </div>
      </div>
    `).join('');
  } catch (e) {
    list.innerHTML = `<div class="loading-row">Failed to load sessions: ${escHtml(e.message)}</div>`;
  }
}

async function resumeSessionFile(file) {
  try {
    const result = await rpc('had.resumeSession', { file });
    toast('Resuming session: ' + (result.file_name || file), 'success');
    switchTab('downloads', document.querySelector('[data-tab=downloads]'));
    setTimeout(refreshAll, 600);
  } catch (e) { toast(e.message, 'error'); }
}

function deleteSessionFile(file) {
  showModal(
    'Delete Session',
    'This will permanently delete the saved resume session.',
    async () => {
      try {
        await rpc('had.deleteSession', { file });
        toast('Session deleted', 'success');
        refreshSessions();
      } catch (e) { toast(e.message, 'error'); }
    }
  );
}

async function refreshHistory() {
  const body = document.getElementById('history-body');
  const empty = document.getElementById('history-empty');
  body.innerHTML = '<div class="loading-row">Loading history…</div>';
  try {
    const history = await rpc('had.getHistory');
    if (!history || history.length === 0) {
      body.innerHTML = '';
      empty.style.display = 'flex';
      return;
    }
    empty.style.display = 'none';
    body.innerHTML = history.map(h => `
      <div class="history-row">
        <div class="history-icon">${getFileIcon(h.file_name)}</div>
        <div class="history-name" title="${escHtml(h.file_name)}">${escHtml(h.file_name)}</div>
        <div class="history-size mono">${h.size_human}</div>
        <div class="history-speed mono">${h.avg_speed || '—'}</div>
        <div class="history-duration mono">${h.duration || '—'}</div>
        <div class="history-status"><span class="status-badge ${h.status === 'downloaded' ? 'badge-downloaded' : 'badge-error'}">${h.status}</span></div>
        <div class="history-time mono">${new Date(h.finished).toLocaleString()}</div>
      </div>
    `).join('');
  } catch (e) {
    body.innerHTML = `<div class="loading-row">Failed to load history: ${escHtml(e.message)}</div>`;
  }
}

async function clearHistoryNow() {
  showModal(
    'Clear History',
    'This will permanently remove all completed-download history entries.',
    async () => {
      try {
        await rpc('had.clearHistory');
        toast('History cleared', 'success');
        refreshHistory();
      } catch (e) { toast(e.message, 'error'); }
    }
  );
}

async function fetchMetaPreview() {
  const url = document.getElementById('tool-meta-url').value.trim();
  if (!url) { toast('Enter a URL', 'warning'); return; }
  const out = document.getElementById('tool-meta-result');
  out.style.display = 'block';
  out.innerHTML = '<div class="loading-row">Fetching metadata…</div>';
  try {
    const meta = await rpc('had.fetchMeta', { url });
    out.innerHTML = `
      <div class="info-grid">
        <span class="info-k">File Name</span><span class="info-v">${escHtml(meta.file_name || '—')}</span>
        <span class="info-k">Size</span><span class="info-v">${escHtml(meta.size_human || '—')}</span>
        <span class="info-k">Content Type</span><span class="info-v">${escHtml(meta.content_type || '—')}</span>
        <span class="info-k">Resumable</span><span class="info-v">${meta.resumable ? '✓ yes' : '✗ no'}</span>
        ${meta.duration ? `<span class="info-k">Duration</span><span class="info-v">${escHtml(meta.duration)}</span>` : ''}
        ${meta.checksum_value ? `<span class="info-k">Checksum</span><span class="info-v mono">${escHtml(meta.checksum_algo)}: ${escHtml(meta.checksum_value)}</span>` : ''}
      </div>
    `;
  } catch (e) {
    out.innerHTML = `<div class="loading-row">${escHtml(e.message)}</div>`;
  }
}

async function runMirrorTest() {
  const raw = document.getElementById('tool-mirror-urls').value.trim();
  const urls = raw.split('\n').map(u => u.trim()).filter(Boolean);
  if (urls.length === 0) { toast('Enter at least one URL', 'warning'); return; }
  const out = document.getElementById('tool-mirror-result');
  out.style.display = 'block';
  out.innerHTML = '<div class="loading-row">Testing mirrors…</div>';
  try {
    const results = await rpc('had.testMirrors', { urls });
    out.innerHTML = results.map(r => `
      <div class="mirror-row ${r.reachable ? '' : 'mirror-dead'}">
        <span class="mirror-rank">#${r.rank}</span>
        <span class="mirror-url mono" title="${escHtml(r.url)}">${escHtml(r.url.length > 60 ? r.url.slice(0, 57) + '…' : r.url)}</span>
        <span class="mirror-speed mono">${r.reachable ? r.speed_human : 'unreachable'}</span>
        <span class="mirror-latency mono">${r.reachable ? r.latency_ms + ' ms' : '—'}</span>
      </div>
    `).join('');
  } catch (e) {
    out.innerHTML = `<div class="loading-row">${escHtml(e.message)}</div>`;
  }
}

async function runChecksumVerify() {
  const file = document.getElementById('tool-checksum-file').value.trim();
  const algo = document.getElementById('tool-checksum-algo').value;
  const expected = document.getElementById('tool-checksum-expected').value.trim();
  if (!file) { toast('Enter a file name', 'warning'); return; }
  const out = document.getElementById('tool-checksum-result');
  out.style.display = 'block';
  out.innerHTML = '<div class="loading-row">Computing hash…</div>';
  try {
    const params = { file, algo };
    if (expected) params.expected = expected;
    const result = await rpc('had.verifyChecksum', params);
    out.innerHTML = `
      <div class="info-grid">
        <span class="info-k">Algorithm</span><span class="info-v">${escHtml(result.algorithm)}</span>
        <span class="info-k">Hash</span><span class="info-v mono">${escHtml(result.hash)}</span>
        ${result.match ? `<span class="info-k">Match</span><span class="info-v">${result.match === 'true' ? '✓ matches' : '✗ mismatch'}</span>` : ''}
      </div>
    `;
  } catch (e) {
    out.innerHTML = `<div class="loading-row">${escHtml(e.message)}</div>`;
  }
}

async function refreshBWSchedule() {
  try {
    const cfg = await rpc('had.getBWSchedule');
    if (cfg) {
      document.getElementById('bw-night-from').value = cfg.night_from || '';
      document.getElementById('bw-night-to').value = cfg.night_to || '';
      document.getElementById('bw-day-limit').value = cfg.day_limit > 0 ? cfg.day_limit : '';
      document.getElementById('bw-night-limit').value = cfg.night_limit > 0 ? cfg.night_limit : '';
    }
  } catch {}
}

async function applyBWSchedule() {
  const night_from = document.getElementById('bw-night-from').value.trim();
  const night_to = document.getElementById('bw-night-to').value.trim();
  const day_limit = parseInt(document.getElementById('bw-day-limit').value) || -1;
  const night_limit = parseInt(document.getElementById('bw-night-limit').value) || -1;
  try {
    await rpc('had.setBWSchedule', { night_from, night_to, day_limit, night_limit });
    toast('Bandwidth schedule applied', 'success');
  } catch (e) { toast(e.message, 'error'); }
}

function appendLogLine(level, msg, ts) {
  const body = document.getElementById('console-body');
  if (!body || logPaused) return;
  const time = ts ? new Date(ts * 1000).toLocaleTimeString() : new Date().toLocaleTimeString();
  const line = document.createElement('div');
  line.className = 'console-line console-' + (level || 'info');
  line.innerHTML = `<span class="console-time">${time}</span><span class="console-level">${(level || 'info').toUpperCase()}</span><span class="console-msg">${escHtml(msg)}</span>`;
  body.appendChild(line);
  while (body.children.length > 400) body.removeChild(body.firstChild);
  body.scrollTop = body.scrollHeight;
}

function connectLogStream() {
  if (logSource) { logSource.close(); logSource = null; }
  const dot = document.getElementById('console-dot');
  try {
    logSource = new EventSource(RPC_BASE + '/ws/log' + tokenQS());
    logSource.onopen = () => { if (dot) dot.className = 'conn-indicator online'; };
    logSource.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data);
        if (data.msg === 'ping') return;
        appendLogLine(data.level, data.msg, data.ts);
      } catch {}
    };
    logSource.onerror = () => {
      if (dot) dot.className = 'conn-indicator offline';
      if (logSource) { logSource.close(); logSource = null; }
      clearTimeout(logReconnectTimer);
      logReconnectTimer = setTimeout(connectLogStream, 3000);
    };
  } catch {}
}

function toggleLogPause() {
  logPaused = !logPaused;
  const btn = document.getElementById('console-pause-btn');
  if (btn) btn.textContent = logPaused ? 'Resume' : 'Pause';
}

function clearConsole() {
  const body = document.getElementById('console-body');
  if (body) body.innerHTML = '';
}

async function applySpeedLimit() {
  const speed = parseInt(document.getElementById('cfg-speed').value) || 0;
  try {
    await rpc('had.setSpeedLimit', { speed });
    toast(`Speed limit: ${speed === 0 ? 'unlimited' : humanBytes(speed) + '/s'}`, 'success');
    refreshGlobalStat();
  } catch (e) { toast(e.message, 'error'); }
}

async function applyMaxParallel() {
  const max = parseInt(document.getElementById('cfg-parallel').value) || 2;
  try {
    await rpc('had.setMaxParallel', { max });
    toast(`Max parallel: ${max}`, 'success');
    refreshGlobalStat();
  } catch (e) { toast(e.message, 'error'); }
}

async function applyThreads() {
  const threads = parseInt(document.getElementById('cfg-threads').value) || 8;
  try {
    await rpc('had.setThreads', { threads });
    toast(`Threads per file: ${threads}`, 'success');
    refreshGlobalStat();
  } catch (e) { toast(e.message, 'error'); }
}

async function applyOutDir() {
  const dir = document.getElementById('cfg-outdir').value.trim() || '.';
  try {
    await rpc('had.setOutDir', { dir });
    toast(`Output dir set: ${dir}`, 'success');
    refreshGlobalStat();
  } catch (e) { toast(e.message, 'error'); }
}

function shutdownHAD() {
  showModal(
    'Shutdown HAD',
    'This will stop the HAD daemon. All active downloads will be interrupted.',
    async () => {
      try {
        await rpc('had.shutdown');
        toast('HAD is shutting down…', 'warning');
        setConnected(false);
        clearInterval(refreshTimer);
      } catch {}
    }
  );
}

function showModal(title, body, onConfirm) {
  document.getElementById('modal-title').textContent = title;
  document.getElementById('modal-body').innerHTML = body;
  document.getElementById('modal-overlay').style.display = 'flex';
  const btn = document.getElementById('modal-confirm-btn');
  btn.onclick = () => { closeModal(); onConfirm(); };
}

function closeModal() {
  document.getElementById('modal-overlay').style.display = 'none';
}

function onDrop(e) {
  e.preventDefault();
  const text = e.dataTransfer.getData('text');
  if (text) {
    document.getElementById('url-input').value = text;
    switchTab('add', document.querySelector('[data-tab=add]'));
    onURLInput(document.getElementById('url-input'));
    toast('URL dropped', 'info');
  }
}

function initParticles() {
  const canvas = document.getElementById('particle-canvas');
  const ctx = canvas.getContext('2d');
  let W, H;
  const particles = [];

  function resize() {
    W = canvas.width = window.innerWidth;
    H = canvas.height = window.innerHeight;
  }
  resize();
  window.addEventListener('resize', resize);

  for (let i = 0; i < 50; i++) {
    particles.push({
      x: Math.random() * W, y: Math.random() * H,
      r: Math.random() * 1.5 + 0.3,
      vx: (Math.random() - .5) * .25, vy: (Math.random() - .5) * .25,
      a: Math.random() * .4 + .1,
      col: Math.random() > .5 ? '124,58,237' : '6,182,212',
    });
  }

  function draw() {
    ctx.clearRect(0, 0, W, H);
    particles.forEach(p => {
      p.x += p.vx; p.y += p.vy;
      if (p.x < 0) p.x = W; if (p.x > W) p.x = 0;
      if (p.y < 0) p.y = H; if (p.y > H) p.y = 0;
      ctx.beginPath();
      ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${p.col},${p.a})`;
      ctx.fill();
    });

    for (let i = 0; i < particles.length; i++) {
      for (let j = i + 1; j < particles.length; j++) {
        const dx = particles[i].x - particles[j].x;
        const dy = particles[i].y - particles[j].y;
        const d = Math.sqrt(dx * dx + dy * dy);
        if (d < 110) {
          ctx.beginPath();
          ctx.moveTo(particles[i].x, particles[i].y);
          ctx.lineTo(particles[j].x, particles[j].y);
          ctx.strokeStyle = `rgba(124,58,237,${(1 - d / 110) * .1})`;
          ctx.lineWidth = .5;
          ctx.stroke();
        }
      }
    }
    requestAnimationFrame(draw);
  }
  draw();
}

function loadSettings() {
  const token = localStorage.getItem('had_token') || '';
  const rpcAddr = localStorage.getItem('had_rpc') || '';
  if (token) document.getElementById('cfg-token').value = token;
  if (rpcAddr) document.getElementById('cfg-rpc').value = rpcAddr;

  document.getElementById('cfg-token').addEventListener('change', function () {
    localStorage.setItem('had_token', this.value.trim());
    toast('Token saved', 'success');
    connectLogStream();
  });
  document.getElementById('cfg-rpc').addEventListener('change', function () {
    localStorage.setItem('had_rpc', this.value.trim());
    toast('RPC address saved — refresh to reconnect', 'info');
  });
}

function escHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;')
    .replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;');
}

document.addEventListener('DOMContentLoaded', () => {
  initParticles();
  initChart();
  loadSettings();
  startRefresh();
  refreshAll();
  connectLogStream();

  document.getElementById('url-input').addEventListener('keydown', e => {
    if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey) {
      e.preventDefault();
      addDownload();
    }
  });

  document.getElementById('scrape-url').addEventListener('keydown', e => {
    if (e.key === 'Enter') { e.preventDefault(); startScrape(); }
  });

  const dz = document.getElementById('drop-zone');
  document.body.addEventListener('dragover', e => { e.preventDefault(); dz.classList.add('drag-over'); });
  document.body.addEventListener('dragleave', () => dz.classList.remove('drag-over'));
  document.body.addEventListener('drop', e => { dz.classList.remove('drag-over'); onDrop(e); });

  document.addEventListener('keydown', e => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();
      switchTab('add', document.querySelector('[data-tab=add]'));
      setTimeout(() => document.getElementById('url-input').focus(), 100);
    }
  });

  window.addEventListener('resize', () => {
    if (window.innerWidth > 768 && sidebarOpen) {
      document.getElementById('sidebar').classList.remove('open');
      sidebarOpen = false;
    }
  });
});