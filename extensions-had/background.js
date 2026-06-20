let proxyEnabled = false;
let proxyConfig = { host: "", port: "", scheme: "http" };
let stats = { requests: 0, startTime: null };

async function setProxy(host, port, scheme) {
  const config = {
    mode: "fixed_servers",
    rules: {
      singleProxy: {
        scheme: scheme || "http",
        host: host,
        port: parseInt(port)
      },
      bypassList: await getBypassList()
    }
  };

  return new Promise((resolve) => {
    chrome.proxy.settings.set({ value: config, scope: "regular" }, () => {
      proxyConfig = { host, port, scheme };
      stats.startTime = Date.now();
      stats.requests = 0;
      console.log("[HAD] Proxy activated:", scheme + "://" + host + ":" + port);
      updateBadge(true);
      resolve(true);
    });
  });
}

async function disableProxy() {
  return new Promise((resolve) => {
    chrome.proxy.settings.clear({ scope: "regular" }, () => {
      proxyConfig = { host: "", port: "", scheme: "http" };
      stats = { requests: 0, startTime: null };
      console.log("[HAD] Proxy disabled");
      updateBadge(false);
      resolve(true);
    });
  });
}

async function getBypassList() {
  const result = await chrome.storage.local.get(["bypassList"]);
  const defaults = ["localhost", "127.0.0.1", "::1"];
  if (result.bypassList && Array.isArray(result.bypassList)) {
    return [...new Set([...defaults, ...result.bypassList])];
  }
  return defaults;
}

function updateBadge(enabled) {
  if (enabled) {
    chrome.action.setBadgeText({ text: "ON" });
    chrome.action.setBadgeBackgroundColor({ color: "#8b0000" });
  } else {
    chrome.action.setBadgeText({ text: "" });
  }
}

chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  (async () => {
    switch (request.action) {
      case "setProxy": {
        const ok = await setProxy(request.host, request.port, request.scheme);
        proxyEnabled = true;
        sendResponse({ status: "ok", success: ok });
        break;
      }
      case "disableProxy": {
        await disableProxy();
        proxyEnabled = false;
        sendResponse({ status: "ok" });
        break;
      }
      case "getStatus": {
        const uptime = stats.startTime ? Math.floor((Date.now() - stats.startTime) / 1000) : 0;
        sendResponse({
          enabled: proxyEnabled,
          config: proxyConfig,
          uptime,
          requests: stats.requests
        });
        break;
      }
      case "getBypassList": {
        const list = await getBypassList();
        sendResponse({ list });
        break;
      }
      case "setBypassList": {
        await chrome.storage.local.set({ bypassList: request.list });
        if (proxyEnabled) {
          await setProxy(proxyConfig.host, proxyConfig.port, proxyConfig.scheme);
        }
        sendResponse({ status: "ok" });
        break;
      }
      case "testProxy": {
        const result = await testProxyConnection(request.host, request.port);
        sendResponse(result);
        break;
      }
      default:
        sendResponse({ status: "unknown_action" });
    }
  })();
  return true;
});

async function testProxyConnection(host, port) {
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);
    const start = Date.now();
    const resp = await fetch(`http://${host}:${port}`, { signal: controller.signal }).catch(() => null);
    clearTimeout(timeout);
    const latency = Date.now() - start;
    return { reachable: true, latency };
  } catch (e) {
    return { reachable: false, latency: -1 };
  }
}

chrome.runtime.onInstalled.addListener(async () => {
  const result = await chrome.storage.local.get(["proxyHost", "proxyPort", "proxyScheme", "autoRestore"]);
  if (result.autoRestore && result.proxyHost && result.proxyPort) {
    await setProxy(result.proxyHost, result.proxyPort, result.proxyScheme || "http");
    proxyEnabled = true;
  }
});

chrome.runtime.onStartup.addListener(async () => {
  const result = await chrome.storage.local.get(["proxyHost", "proxyPort", "proxyScheme", "autoRestore"]);
  if (result.autoRestore && result.proxyHost && result.proxyPort) {
    await setProxy(result.proxyHost, result.proxyPort, result.proxyScheme || "http");
    proxyEnabled = true;
  }
});