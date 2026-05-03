import { deserializeProviderError, type SqlJsWorkerRequest, type SqlJsWorkerResponse } from './workerProtocol';

interface PendingRequest<T> {
  resolve: (value: T) => void;
  reject: (error: unknown) => void;
}

let worker: Worker | null = null;
let nextID = 1;
const pending = new Map<number, PendingRequest<unknown>>();

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
      const error = new Error(event.message || 'sql.js worker error');
      for (const request of pending.values()) {
        request.reject(error);
      }
      pending.clear();
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
  return new Promise<T>((resolve, reject) => {
    pending.set(id, { resolve: resolve as (value: unknown) => void, reject });
    getWorker().postMessage(request);
  });
}

export function resetSqlJsWorkerForTests(): void {
  worker?.terminate();
  worker = null;
  nextID = 1;
  for (const request of pending.values()) {
    request.reject(new Error('sql.js worker reset'));
  }
  pending.clear();
}
