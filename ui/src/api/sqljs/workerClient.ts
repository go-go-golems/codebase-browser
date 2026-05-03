import { deserializeProviderError, type SqlJsWorkerRequest, type SqlJsWorkerResponse } from './workerProtocol';

interface PendingRequest<T> {
  resolve: (value: T) => void;
  reject: (error: unknown) => void;
}

let worker: Worker | null = null;
let nextID = 1;
const pending = new Map<number, PendingRequest<unknown>>();

function workerTimeoutMs(): number {
  if (typeof window === 'undefined') return 60000;
  const raw = new URLSearchParams(window.location.search).get('sqlWorkerTimeoutMs');
  if (!raw) return 60000;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 60000;
}

function debugSqlEnabled(): boolean {
  return typeof window !== 'undefined' && new URLSearchParams(window.location.search).has('debugSql');
}

function workerBaseUrl(): string {
  return typeof document !== 'undefined' ? document.baseURI : self.location.href;
}

function getWorker(): Worker {
  if (!worker) {
    worker = new Worker(new URL('./sqlJsQueryWorker.ts', import.meta.url), { type: 'module' });
    worker.onmessage = (event: MessageEvent<SqlJsWorkerResponse>) => {
      const response = event.data;
      const request = pending.get(response.id);
      if (!request) return;
      pending.delete(response.id);

      if (response.ok) {
        if (debugSqlEnabled()) {
          console.warn('[sql.js-worker:done]', {
            method: response.timing.method,
            elapsedMs: Math.round(response.timing.elapsedMs),
          });
        }
        request.resolve(response.result);
        return;
      }

      request.reject(deserializeProviderError(response.error));
    };
    worker.onerror = (event) => {
      resetSqlJsWorker(event.message || 'sql.js worker error');
    };
  }
  return worker;
}

export function callSqlJsWorker<T>(method: string, args: unknown[]): Promise<T> {
  const id = nextID++;
  const request: SqlJsWorkerRequest = {
    id,
    method,
    args,
    baseUrl: workerBaseUrl(),
    debugSql: debugSqlEnabled(),
  };
  const timeoutMs = workerTimeoutMs();
  return new Promise<T>((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      resetSqlJsWorker(`sql.js worker request timed out after ${timeoutMs} ms while running ${method}`);
    }, timeoutMs);
    pending.set(id, {
      resolve: (value) => {
        window.clearTimeout(timeout);
        resolve(value as T);
      },
      reject: (error) => {
        window.clearTimeout(timeout);
        reject(error);
      },
    });
    getWorker().postMessage(request);
  });
}

export function resetSqlJsWorker(reason = 'sql.js worker reset'): void {
  worker?.terminate();
  worker = null;
  nextID = 1;
  const error = new Error(reason);
  for (const request of pending.values()) {
    request.reject(error);
  }
  pending.clear();
}

export function resetSqlJsWorkerForTests(): void {
  resetSqlJsWorker('sql.js worker reset');
}
