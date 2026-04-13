export type Action = 'exec' | 'navigate';

export interface Command {
  id: string;
  action: Action;
  workspace?: string;
  tabId?: number;
  url?: string;
  code?: string;
}

export interface Result {
  id: string;
  ok: boolean;
  data?: unknown;
  error?: string;
}

export const DAEMON_PORT = 19860;
export const DAEMON_HOST = 'localhost';
export const DAEMON_WS_URL = `ws://${DAEMON_HOST}:${DAEMON_PORT}/ext`;
export const DAEMON_PING_URL = `http://${DAEMON_HOST}:${DAEMON_PORT}/ping`;
