let allCookies = [];
let uptimeInterval = null;

async function loadSavedSettings() {
  const result = await chrome.storage.local.get(["proxyHost", "proxyPort", "proxyScheme", "autoRestore"]);
  if (result.proxyHost) document.getElementById("host").value = result.proxyHost;
  if (result.proxyPort) document.getElementById("port").value = result.proxyPort;
  if (result.proxyScheme) document.getElementById("scheme").value = result.proxyScheme;
  if (result.autoRestore) document.getElementById("autoRestore").checked = result.autoRestore;
}

async function saveSettings(host, port, scheme) {
  const autoRestore = document.getElementById("autoRestore").checked;
  await chrome.storage.local.set({ proxyHost: host, proxyPort: port, proxyScheme: scheme, autoRestore });
}

async function updateStatus() {
  const response = await chrome.runtime.sendMessage({ action: "getStatus" });
  const bar = document.getElementById("statusBar");
  const dot = document.getElementById("statusDot");
  const text = document.getElementById("statusText");
  const info = document.getElementById("statusInfo");

  if (response.enabled) {
    bar.className = "status-bar on";
    dot.className = "status-dot on";
    text.textContent = "ACTIVE";
    const cfg = response.config;
    info.innerHTML = `<span>${cfg.scheme.toUpperCase()} <span class="val">${cfg.host}:${cfg.port}</span></span>
      <span>UP <span class="val">${formatUptime(response.uptime)}</span></span>`;

    if (uptimeInterval) clearInterval(uptimeInterval);
    let uptime = response.uptime;
    uptimeInterval = setInterval(async () => {
      uptime++;
      const r = await chrome.runtime.sendMessage({ action: "getStatus" });
      if (!r.enabled) { clearInterval(uptimeInterval); updateStatus(); return; }
      info.innerHTML = `<span>${r.config.scheme.toUpperCase()} <span class="val">${r.config.host}:${r.config.port}</span></span>
        <span>UP <span class="val">${formatUptime(r.uptime)}</span></span>`;
    }, 5000);
  } else {
    bar.className = "status-bar off";
    dot.className = "status-dot";
    text.textContent = "INACTIVE";
    info.innerHTML = "";
    if (uptimeInterval) { clearInterval(uptimeInterval); uptimeInterval = null; }
  }
}

function formatUptime(sec) {
  if (sec < 60) return sec + "s";
  if (sec < 3600) return Math.floor(sec / 60) + "m " + (sec % 60) + "s";
  return Math.floor(sec / 3600) + "h " + Math.floor((sec % 3600) / 60) + "m";
}

async function getCurrentTab() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  return tab;
}

async function getCookiesForCurrentSite() {
  const tab = await getCurrentTab();
  if (!tab?.url) return [];
  const url = new URL(tab.url);
  const domain = url.hostname;
  const cookies = await chrome.cookies.getAll({});
  return cookies.filter(c => {
    const cd = c.domain.replace(/^\./, "");
    return domain === cd || domain.endsWith("." + cd) || cd.endsWith("." + domain);
  });
}

function escapeHtml(str) {
  const d = document.createElement("div");
  d.textContent = str;
  return d.innerHTML;
}

function buildCookieFlags(cookie) {
  let flags = "";
  if (cookie.secure) flags += `<span class="flag flag-secure">SECURE</span>`;
  if (cookie.httpOnly) flags += `<span class="flag flag-http">HTTP</span>`;
  if (!cookie.expirationDate) flags += `<span class="flag flag-session">SESSION</span>`;
  return flags;
}

function buildCookieMeta(cookie) {
  const parts = [];
  if (cookie.domain) parts.push(`domain: ${cookie.domain}`);
  if (cookie.path) parts.push(`path: ${cookie.path}`);
  if (cookie.expirationDate) {
    const d = new Date(cookie.expirationDate * 1000);
    parts.push(`expires: ${d.toLocaleDateString()}`);
  }
  return parts.map(p => `<span>${p}</span>`).join("");
}

function renderCookies(cookies) {
  const listDiv = document.getElementById("cookieList");
  const count = document.getElementById("cookieCount");

  if (!cookies.length) {
    listDiv.innerHTML = `<div class="empty-state">🍪 No cookies found for this site</div>`;
    count.textContent = "0 cookies";
    return;
  }

  count.textContent = `${cookies.length} cookie${cookies.length !== 1 ? "s" : ""} found`;

  listDiv.innerHTML = cookies.map((c, i) => `
    <div class="cookie-item" data-idx="${i}">
      <div class="cookie-header">
        <div class="cookie-name">🔑 ${escapeHtml(c.name)}</div>
        <div class="cookie-flags">${buildCookieFlags(c)}</div>
      </div>
      <div class="cookie-value" title="Click to expand" data-expanded="false">
        ${escapeHtml(c.value || "(empty)")}
      </div>
      <div class="cookie-meta">${buildCookieMeta(c)}</div>
      <div class="cookie-actions">
        <button class="btn-sm btn-primary edit-btn" data-idx="${i}">✏️ EDIT</button>
        <button class="btn-sm btn-danger delete-btn" data-idx="${i}">🗑 DEL</button>
        <button class="btn-sm btn-primary copy-btn" data-idx="${i}">📋 COPY</button>
      </div>
      <div class="edit-overlay" id="edit-${i}">
        <input type="text" id="edit-val-${i}" placeholder="New value..." value="${escapeHtml(c.value)}">
        <div class="btn-group">
          <button class="btn-sm btn-success save-edit-btn" data-idx="${i}">✅ SAVE</button>
          <button class="btn-sm btn-danger cancel-edit-btn" data-idx="${i}">✖ CANCEL</button>
        </div>
      </div>
    </div>
  `).join("");

  listDiv.querySelectorAll(".cookie-value").forEach(el => {
    el.addEventListener("click", () => {
      const expanded = el.dataset.expanded === "true";
      el.dataset.expanded = String(!expanded);
      el.classList.toggle("expanded", !expanded);
    });
  });

  listDiv.querySelectorAll(".edit-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      const i = btn.dataset.idx;
      const overlay = document.getElementById(`edit-${i}`);
      overlay.classList.toggle("open");
    });
  });

  listDiv.querySelectorAll(".cancel-edit-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      document.getElementById(`edit-${btn.dataset.idx}`).classList.remove("open");
    });
  });

  listDiv.querySelectorAll(".save-edit-btn").forEach(btn => {
    btn.addEventListener("click", async () => {
      const i = parseInt(btn.dataset.idx);
      const newVal = document.getElementById(`edit-val-${i}`).value;
      await editCookie(cookies[i], newVal);
      await displayCookies();
    });
  });

  listDiv.querySelectorAll(".delete-btn").forEach(btn => {
    btn.addEventListener("click", async () => {
      const i = parseInt(btn.dataset.idx);
      await deleteCookie(cookies[i]);
      await displayCookies();
    });
  });

  listDiv.querySelectorAll(".copy-btn").forEach(btn => {
    btn.addEventListener("click", async () => {
      const i = parseInt(btn.dataset.idx);
      await navigator.clipboard.writeText(`${cookies[i].name}=${cookies[i].value}`);
      notify("📋 Cookie copied!", "success");
    });
  });
}

async function displayCookies() {
  allCookies = await getCookiesForCurrentSite();
  const search = document.getElementById("cookieSearch").value.toLowerCase().trim();
  const filtered = search
    ? allCookies.filter(c => c.name.toLowerCase().includes(search) || c.value.toLowerCase().includes(search))
    : allCookies;
  renderCookies(filtered);
}

async function deleteCookie(cookie) {
  const tab = await getCurrentTab();
  if (!tab?.url) return;
  const url = new URL(tab.url);
  const domain = cookie.domain.replace(/^\./, "");
  const cookieUrl = `${url.protocol}//${domain}${cookie.path || "/"}`;
  await chrome.cookies.remove({ url: cookieUrl, name: cookie.name });
}

async function editCookie(cookie, newValue) {
  const tab = await getCurrentTab();
  if (!tab?.url) return;
  const url = new URL(tab.url);
  const domain = cookie.domain.replace(/^\./, "");
  const cookieUrl = `${url.protocol}//${domain}${cookie.path || "/"}`;

  const details = {
    url: cookieUrl,
    name: cookie.name,
    value: newValue,
    path: cookie.path || "/",
    secure: cookie.secure,
    httpOnly: cookie.httpOnly,
    sameSite: cookie.sameSite || "lax",
  };
  if (cookie.expirationDate) details.expirationDate = cookie.expirationDate;

  try {
    await chrome.cookies.set(details);
    notify("✅ Cookie updated", "success");
  } catch (e) {
    notify("❌ Failed: " + e.message, "error");
  }
}

async function clearAllCookies() {
  if (!confirm("Delete ALL cookies for this site?")) return;
  for (const c of allCookies) await deleteCookie(c);
  await displayCookies();
  notify("🗑 All cookies cleared", "success");
}

function formatCookiesAsHeader(cookies) {
  return cookies.map(c => `${c.name}=${c.value}`).join("; ");
}

function formatCookiesAsJSON(cookies) {
  return JSON.stringify(cookies, null, 2);
}

function formatCookiesAsNetscape(cookies) {
  const lines = [
    "# Netscape HTTP Cookie File",
    "# Generated by HAD Proxy v2.0"
  ];
  for (const c of cookies) {
    const domain = c.domain.startsWith(".") ? c.domain : "." + c.domain;
    const sub = c.domain.startsWith(".") ? "TRUE" : "FALSE";
    const secure = c.secure ? "TRUE" : "FALSE";
    const exp = c.expirationDate ? Math.floor(c.expirationDate) : 0;
    lines.push(`${domain}\t${sub}\t${c.path || "/"}\t${secure}\t${exp}\t${c.name}\t${c.value}`);
  }
  return lines.join("\n");
}

async function copyAllCookies() {
  const cookies = await getCookiesForCurrentSite();
  const format = document.getElementById("exportFormat").value;
  let text = format === "json" ? formatCookiesAsJSON(cookies)
    : format === "netscape" ? formatCookiesAsNetscape(cookies)
    : formatCookiesAsHeader(cookies);
  await navigator.clipboard.writeText(text);
  notify("📋 Copied " + cookies.length + " cookies!", "success");
}

async function exportCookies() {
  const cookies = await getCookiesForCurrentSite();
  const format = document.getElementById("exportFormat").value;
  let content, filename, mime;

  if (format === "json") {
    content = formatCookiesAsJSON(cookies);
    filename = "cookies.json";
    mime = "application/json";
  } else if (format === "netscape") {
    content = formatCookiesAsNetscape(cookies);
    filename = "cookies.txt";
    mime = "text/plain";
  } else {
    content = formatCookiesAsHeader(cookies);
    filename = "cookies_header.txt";
    mime = "text/plain";
  }

  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
  notify(`💾 Exported as ${filename}`, "success");
}

function parseNetscapeCookies(text) {
  return text.split("\n")
    .filter(l => l.trim() && !l.trim().startsWith("#"))
    .map(l => {
      const p = l.split("\t");
      if (p.length < 7) return null;
      return {
        domain: p[0].replace(/^\./, ""),
        path: p[2],
        secure: p[3] === "TRUE",
        expirationDate: parseInt(p[4]) || undefined,
        name: p[5],
        value: p[6],
        httpOnly: false,
        sameSite: "lax"
      };
    }).filter(Boolean);
}

function parseHeaderCookies(text) {
  return text.split(";").map(s => s.trim()).filter(s => s.includes("=")).map(s => {
    const i = s.indexOf("=");
    return { name: s.slice(0, i).trim(), value: s.slice(i + 1).trim(), path: "/", secure: false, httpOnly: false, sameSite: "lax", domain: "" };
  });
}

async function importCookies() {
  const raw = document.getElementById("importData").value.trim();
  if (!raw) { notify("❌ No data to import", "error"); return; }

  const tab = await getCurrentTab();
  if (!tab?.url) { notify("❌ No active tab", "error"); return; }
  const url = new URL(tab.url);

  let cookies = [];
  if (raw.startsWith("{") || raw.startsWith("[")) {
    try {
      const p = JSON.parse(raw);
      cookies = Array.isArray(p) ? p : [p];
    } catch { notify("❌ Invalid JSON", "error"); return; }
  } else if (raw.includes("\t")) {
    cookies = parseNetscapeCookies(raw);
  } else if (raw.includes("=")) {
    cookies = parseHeaderCookies(raw);
  } else {
    notify("❌ Unknown format", "error");
    return;
  }

  let imported = 0;
  for (const c of cookies) {
    if (!c.name) continue;
    try {
      const details = {
        url: url.origin,
        name: c.name,
        value: c.value || "",
        path: c.path || "/",
        secure: c.secure || false,
        httpOnly: c.httpOnly || false,
        sameSite: c.sameSite || "lax"
      };
      if (c.expirationDate) details.expirationDate = c.expirationDate;
      if (c.domain && c.domain !== url.hostname) details.domain = c.domain;
      await chrome.cookies.set(details);
      imported++;
    } catch {}
  }

  notify(`✅ Imported ${imported} cookie${imported !== 1 ? "s" : ""}`, "success");
  document.getElementById("importData").value = "";
  await displayCookies();
}

async function testProxy() {
  const host = document.getElementById("host").value.trim();
  const port = document.getElementById("port").value.trim();
  const resultDiv = document.getElementById("testResult");

  if (!host || !port) { notify("❌ Enter host and port first", "error"); return; }

  resultDiv.style.display = "block";
  resultDiv.className = "test-result";
  resultDiv.textContent = "⏳ Testing connection...";

  const r = await chrome.runtime.sendMessage({ action: "testProxy", host, port });

  if (r.reachable) {
    resultDiv.className = "test-result test-ok";
    resultDiv.textContent = `✅ REACHABLE — ${r.latency}ms`;
  } else {
    resultDiv.className = "test-result test-fail";
    resultDiv.textContent = "❌ UNREACHABLE — Check host & port";
  }

  setTimeout(() => { resultDiv.style.display = "none"; }, 5000);
}

async function loadBypassList() {
  const r = await chrome.runtime.sendMessage({ action: "getBypassList" });
  const defaults = ["localhost", "127.0.0.1", "::1"];
  const extra = r.list.filter(h => !defaults.includes(h));
  document.getElementById("bypassList").value = [...defaults, ...extra].join("\n");
}

async function saveBypassList() {
  const raw = document.getElementById("bypassList").value;
  const list = raw.split("\n").map(l => l.trim()).filter(Boolean);
  await chrome.runtime.sendMessage({ action: "setBypassList", list });
  notify("💾 Bypass list saved", "success");
}

function notify(msg, type = "") {
  const el = document.getElementById("notification");
  el.textContent = msg;
  el.className = "notification" + (type ? " " + type : "");

  void el.offsetWidth;
  el.classList.add("show");
  setTimeout(() => el.classList.remove("show"), 2200);
}

function setupTabs() {
  document.querySelectorAll(".tab-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".tab-btn").forEach(b => b.classList.remove("active"));
      document.querySelectorAll(".tab-content").forEach(c => c.classList.remove("active"));
      btn.classList.add("active");
      const tab = btn.dataset.tab;
      document.getElementById(tab + "-tab").classList.add("active");
      if (tab === "cookies") displayCookies();
      if (tab === "settings") loadBypassList();
    });
  });
}

function setupPresets() {
  document.querySelectorAll(".preset-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      document.getElementById("host").value = btn.dataset.host;
      document.getElementById("port").value = btn.dataset.port;
      document.getElementById("scheme").value = btn.dataset.scheme;
    });
  });
}

document.getElementById("enableBtn").addEventListener("click", async () => {
  const host = document.getElementById("host").value.trim();
  const port = document.getElementById("port").value.trim();
  const scheme = document.getElementById("scheme").value;
  if (!host || !port) { notify("❌ Host & Port required", "error"); return; }
  await saveSettings(host, port, scheme);
  const r = await chrome.runtime.sendMessage({ action: "setProxy", host, port, scheme });
  await updateStatus();
  notify("⚡ Proxy activated!", "success");
});

document.getElementById("disableBtn").addEventListener("click", async () => {
  await chrome.runtime.sendMessage({ action: "disableProxy" });
  await updateStatus();
  notify("Proxy deactivated");
});

document.getElementById("testBtn").addEventListener("click", testProxy);
document.getElementById("refreshCookiesBtn").addEventListener("click", displayCookies);
document.getElementById("copyCookiesBtn").addEventListener("click", copyAllCookies);
document.getElementById("clearCookiesBtn").addEventListener("click", clearAllCookies);
document.getElementById("exportBtn").addEventListener("click", exportCookies);
document.getElementById("importBtn").addEventListener("click", importCookies);
document.getElementById("saveBypassBtn").addEventListener("click", saveBypassList);

document.getElementById("autoRestore").addEventListener("change", async (e) => {
  await chrome.storage.local.set({ autoRestore: e.target.checked });
});

document.getElementById("cookieSearch").addEventListener("input", () => {
  const search = document.getElementById("cookieSearch").value.toLowerCase().trim();
  const filtered = search
    ? allCookies.filter(c => c.name.toLowerCase().includes(search) || c.value.toLowerCase().includes(search))
    : allCookies;
  renderCookies(filtered);
});

loadSavedSettings();
updateStatus();
setupTabs();
setupPresets();