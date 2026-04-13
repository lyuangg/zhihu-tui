import type { Command, Result } from './protocol';
import { DAEMON_PING_URL, DAEMON_WS_URL } from './protocol';
import * as cdp from './cdp';

type Session = {
  windowId: number;
  tabId: number;
};

const sessions = new Map<string, Session>();
let ws: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempts = 0;

const RECONNECT_BASE_DELAY = 2000;
const RECONNECT_MAX_DELAY = 5000;

function getWorkspaceKey(workspace?: string): string {
  return workspace?.trim() || 'default';
}

function isSafeNavigationUrl(url: string): boolean {
  return url.startsWith('http://') || url.startsWith('https://');
}

async function connect(): Promise<void> {
  if (ws?.readyState === WebSocket.OPEN || ws?.readyState === WebSocket.CONNECTING) return;

  try {
    const ping = await fetch(DAEMON_PING_URL, { signal: AbortSignal.timeout(1000) });
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
      type: 'hello',
      version: chrome.runtime.getManifest().version,
      compatRange: '>=0.1.0',
    }));
  };

  ws.onmessage = async (event) => {
    try {
      const cmd = JSON.parse(event.data as string) as Command;
      const result = await handleCommand(cmd);
      ws?.send(JSON.stringify(result));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ws?.send(JSON.stringify({ id: 'unknown', ok: false, error: message }));
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

function scheduleReconnect(): void {
  if (reconnectTimer) return;
  reconnectAttempts += 1;
  const delay = Math.min(RECONNECT_BASE_DELAY * Math.pow(2, reconnectAttempts - 1), RECONNECT_MAX_DELAY);
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    void connect();
  }, delay);
}

async function getOrCreateSession(workspace: string, initialUrl?: string): Promise<Session> {
  const existing = sessions.get(workspace);
  if (existing) {
    try {
      await chrome.tabs.get(existing.tabId);
      return existing;
    } catch {
      sessions.delete(workspace);
    }
  }

  const startUrl = (initialUrl && isSafeNavigationUrl(initialUrl)) ? initialUrl : 'about:blank';
  const win = await chrome.windows.create({
    url: startUrl,
    focused: false,
    width: 1280,
    height: 900,
    type: 'normal',
  });
  const tabs = await chrome.tabs.query({ windowId: win.id! });
  const tabId = tabs[0]?.id;
  if (!tabId) throw new Error('Failed to create automation tab');

  const session = { windowId: win.id!, tabId };
  sessions.set(workspace, session);
  return session;
}

async function resolveTabId(cmd: Command): Promise<number> {
  if (cmd.tabId) return cmd.tabId;
  const workspace = getWorkspaceKey(cmd.workspace);
  const session = await getOrCreateSession(workspace, cmd.url);
  return session.tabId;
}

async function handleNavigate(cmd: Command): Promise<Result> {
  if (!cmd.url) return { id: cmd.id, ok: false, error: 'Missing url' };
  if (!isSafeNavigationUrl(cmd.url)) {
    return { id: cmd.id, ok: false, error: 'Blocked URL scheme -- only http:// and https:// are allowed' };
  }

  const workspace = getWorkspaceKey(cmd.workspace);
  const session = await getOrCreateSession(workspace, cmd.url);
  const tabId = session.tabId;

  await cdp.detach(tabId);
  await chrome.tabs.update(tabId, { url: cmd.url });

  let timedOut = false;
  await new Promise<void>((resolve) => {
    const listener = (id: number, info: chrome.tabs.TabChangeInfo) => {
      if (id !== tabId) return;
      if (info.status === 'complete') {
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
    }, 15000);
  });

  const tab = await chrome.tabs.get(tabId);
  return {
    id: cmd.id,
    ok: true,
    data: {
      tabId,
      title: tab.title,
      url: tab.url,
      timedOut,
    },
  };
}

async function handleExec(cmd: Command): Promise<Result> {
  if (!cmd.code) return { id: cmd.id, ok: false, error: 'Missing code' };
  const tabId = await resolveTabId(cmd);
  try {
    const data = await cdp.evaluate(tabId, cmd.code);
    return { id: cmd.id, ok: true, data };
  } catch (err) {
    return { id: cmd.id, ok: false, error: err instanceof Error ? err.message : String(err) };
  }
}

async function handleCommand(cmd: Command): Promise<Result> {
  try {
    switch (cmd.action) {
      case 'navigate':
        return await handleNavigate(cmd);
      case 'exec':
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

function initialize(): void {
  chrome.alarms.create('keepalive', { periodInMinutes: 0.4 });
  cdp.registerListeners();
  void connect();
}

chrome.runtime.onInstalled.addListener(initialize);
chrome.runtime.onStartup.addListener(initialize);
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === 'keepalive') void connect();
});
