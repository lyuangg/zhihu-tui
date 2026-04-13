const DAEMON_PORT = 19860;
const DAEMON_HOST = "localhost";
const DAEMON_WS_URL = `ws://${DAEMON_HOST}:${DAEMON_PORT}/ext`;
const DAEMON_PING_URL = `http://${DAEMON_HOST}:${DAEMON_PORT}/ping`;

const attached = /* @__PURE__ */ new Set();
function isDebuggableUrl(url) {
  if (!url) return true;
  return url.startsWith("http://") || url.startsWith("https://") || url === "about:blank";
}
async function ensureAttached(tabId) {
  try {
    const tab = await chrome.tabs.get(tabId);
    if (!isDebuggableUrl(tab.url)) {
      attached.delete(tabId);
      throw new Error(`Cannot debug tab ${tabId}: ${tab.url ?? "unknown"}`);
    }
  } catch {
    attached.delete(tabId);
    throw new Error(`Tab ${tabId} no longer exists`);
  }
  if (attached.has(tabId)) return;
  await chrome.debugger.attach({ tabId }, "1.3");
  attached.add(tabId);
  try {
    await chrome.debugger.sendCommand({ tabId }, "Runtime.enable");
  } catch {
  }
}
async function evaluate(tabId, expression) {
  await ensureAttached(tabId);
  const result = await chrome.debugger.sendCommand({ tabId }, "Runtime.evaluate", {
    expression,
    returnByValue: true,
    awaitPromise: true
  });
  if (result.exceptionDetails) {
    const msg = result.exceptionDetails.exception?.description || result.exceptionDetails.text || "Eval error";
    throw new Error(msg);
  }
  return result.result?.value;
}
async function detach(tabId) {
  if (!attached.has(tabId)) return;
  attached.delete(tabId);
  try {
    await chrome.debugger.detach({ tabId });
  } catch {
  }
}
function registerListeners() {
  chrome.tabs.onRemoved.addListener((tabId) => {
    void detach(tabId);
  });
  chrome.debugger.onDetach.addListener((source) => {
    if (source.tabId) attached.delete(source.tabId);
  });
}

const sessions = /* @__PURE__ */ new Map();
let ws = null;
let reconnectTimer = null;
let reconnectAttempts = 0;
const RECONNECT_BASE_DELAY = 2e3;
const RECONNECT_MAX_DELAY = 5e3;
function getWorkspaceKey(workspace) {
  return workspace?.trim() || "default";
}
function isSafeNavigationUrl(url) {
  return url.startsWith("http://") || url.startsWith("https://");
}
async function connect() {
  if (ws?.readyState === WebSocket.OPEN || ws?.readyState === WebSocket.CONNECTING) return;
  try {
    const ping = await fetch(DAEMON_PING_URL, { signal: AbortSignal.timeout(1e3) });
    if (!ping.ok) return;
  } catch {
    return;
  }
  try {
    ws = new WebSocket(DAEMON_WS_URL);
  } catch {
    scheduleReconnect();
    return;
  }
  ws.onopen = () => {
    reconnectAttempts = 0;
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    ws?.send(JSON.stringify({
      type: "hello",
      version: chrome.runtime.getManifest().version,
      compatRange: ">=0.1.0"
    }));
  };
  ws.onmessage = async (event) => {
    try {
      const cmd = JSON.parse(event.data);
      const result = await handleCommand(cmd);
      ws?.send(JSON.stringify(result));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ws?.send(JSON.stringify({ id: "unknown", ok: false, error: message }));
    }
  };
  ws.onclose = () => {
    ws = null;
    scheduleReconnect();
  };
  ws.onerror = () => {
    ws?.close();
  };
}
function scheduleReconnect() {
  if (reconnectTimer) return;
  reconnectAttempts += 1;
  const delay = Math.min(RECONNECT_BASE_DELAY * Math.pow(2, reconnectAttempts - 1), RECONNECT_MAX_DELAY);
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    void connect();
  }, delay);
}
async function getOrCreateSession(workspace, initialUrl) {
  const existing = sessions.get(workspace);
  if (existing) {
    try {
      await chrome.tabs.get(existing.tabId);
      return existing;
    } catch {
      sessions.delete(workspace);
    }
  }
  const startUrl = initialUrl && isSafeNavigationUrl(initialUrl) ? initialUrl : "about:blank";
  const win = await chrome.windows.create({
    url: startUrl,
    focused: false,
    width: 1280,
    height: 900,
    type: "normal"
  });
  const tabs = await chrome.tabs.query({ windowId: win.id });
  const tabId = tabs[0]?.id;
  if (!tabId) throw new Error("Failed to create automation tab");
  const session = { windowId: win.id, tabId };
  sessions.set(workspace, session);
  return session;
}
async function resolveTabId(cmd) {
  if (cmd.tabId) return cmd.tabId;
  const workspace = getWorkspaceKey(cmd.workspace);
  const session = await getOrCreateSession(workspace, cmd.url);
  return session.tabId;
}
async function handleNavigate(cmd) {
  if (!cmd.url) return { id: cmd.id, ok: false, error: "Missing url" };
  if (!isSafeNavigationUrl(cmd.url)) {
    return { id: cmd.id, ok: false, error: "Blocked URL scheme -- only http:// and https:// are allowed" };
  }
  const workspace = getWorkspaceKey(cmd.workspace);
  const session = await getOrCreateSession(workspace, cmd.url);
  const tabId = session.tabId;
  await detach(tabId);
  await chrome.tabs.update(tabId, { url: cmd.url });
  let timedOut = false;
  await new Promise((resolve) => {
    const listener = (id, info) => {
      if (id !== tabId) return;
      if (info.status === "complete") {
        chrome.tabs.onUpdated.removeListener(listener);
        clearTimeout(timeout);
        resolve();
      }
    };
    chrome.tabs.onUpdated.addListener(listener);
    const timeout = setTimeout(() => {
      timedOut = true;
      chrome.tabs.onUpdated.removeListener(listener);
      resolve();
    }, 15e3);
  });
  const tab = await chrome.tabs.get(tabId);
  return {
    id: cmd.id,
    ok: true,
    data: {
      tabId,
      title: tab.title,
      url: tab.url,
      timedOut
    }
  };
}
async function handleExec(cmd) {
  if (!cmd.code) return { id: cmd.id, ok: false, error: "Missing code" };
  const tabId = await resolveTabId(cmd);
  try {
    const data = await evaluate(tabId, cmd.code);
    return { id: cmd.id, ok: true, data };
  } catch (err) {
    return { id: cmd.id, ok: false, error: err instanceof Error ? err.message : String(err) };
  }
}
async function handleCommand(cmd) {
  try {
    switch (cmd.action) {
      case "navigate":
        return await handleNavigate(cmd);
      case "exec":
        return await handleExec(cmd);
      default:
        return { id: cmd.id, ok: false, error: `Unsupported action: ${cmd.action}` };
    }
  } catch (err) {
    return { id: cmd.id, ok: false, error: err instanceof Error ? err.message : String(err) };
  }
}
chrome.windows.onRemoved.addListener((windowId) => {
  for (const [workspace, session] of sessions.entries()) {
    if (session.windowId === windowId) {
      sessions.delete(workspace);
    }
  }
});
function initialize() {
  chrome.alarms.create("keepalive", { periodInMinutes: 0.4 });
  registerListeners();
  void connect();
}
chrome.runtime.onInstalled.addListener(initialize);
chrome.runtime.onStartup.addListener(initialize);
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "keepalive") void connect();
});
