const attached = new Set<number>();

function isDebuggableUrl(url?: string): boolean {
  if (!url) return true;
  return url.startsWith('http://') || url.startsWith('https://') || url === 'about:blank';
}

export async function ensureAttached(tabId: number): Promise<void> {
  try {
    const tab = await chrome.tabs.get(tabId);
    if (!isDebuggableUrl(tab.url)) {
      attached.delete(tabId);
      throw new Error(`Cannot debug tab ${tabId}: ${tab.url ?? 'unknown'}`);
    }
  } catch {
    attached.delete(tabId);
    throw new Error(`Tab ${tabId} no longer exists`);
  }

  if (attached.has(tabId)) return;
  await chrome.debugger.attach({ tabId }, '1.3');
  attached.add(tabId);
  try {
    await chrome.debugger.sendCommand({ tabId }, 'Runtime.enable');
  } catch {
    // no-op
  }
}

export async function evaluate(tabId: number, expression: string): Promise<unknown> {
  await ensureAttached(tabId);
  const result = await chrome.debugger.sendCommand({ tabId }, 'Runtime.evaluate', {
    expression,
    returnByValue: true,
    awaitPromise: true,
  }) as {
    result?: { value?: unknown };
    exceptionDetails?: { exception?: { description?: string }; text?: string };
  };

  if (result.exceptionDetails) {
    const msg = result.exceptionDetails.exception?.description || result.exceptionDetails.text || 'Eval error';
    throw new Error(msg);
  }
  return result.result?.value;
}

export async function detach(tabId: number): Promise<void> {
  if (!attached.has(tabId)) return;
  attached.delete(tabId);
  try {
    await chrome.debugger.detach({ tabId });
  } catch {
    // no-op
  }
}

export function registerListeners(): void {
  chrome.tabs.onRemoved.addListener((tabId) => {
    void detach(tabId);
  });
  chrome.debugger.onDetach.addListener((source) => {
    if (source.tabId) attached.delete(source.tabId);
  });
}
