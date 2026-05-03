/// <reference lib="webworker" />

import { SqlJsQueryProvider } from '../sqlJsQueryProvider';
import type { CodebaseQueryProvider } from '../queryProvider';
import { createStaticDbLoader } from './sqlJsDb';
import { serializeProviderError, type SqlJsWorkerRequest, type SqlJsWorkerResponse } from './workerProtocol';

let provider: CodebaseQueryProvider | null = null;
let providerBaseUrl = '';

function getProvider(baseUrl: string): CodebaseQueryProvider {
  if (!provider || providerBaseUrl !== baseUrl) {
    providerBaseUrl = baseUrl;
    provider = new SqlJsQueryProvider(createStaticDbLoader({ baseUrl }));
  }
  return provider;
}

self.onmessage = async (event: MessageEvent<SqlJsWorkerRequest>) => {
  const { id, method, args, baseUrl = self.location.href, debugSql = false } = event.data;
  const started = performance.now();

  try {
    const queryProvider = getProvider(baseUrl);
    const fn = queryProvider[method as keyof CodebaseQueryProvider];
    if (typeof fn !== 'function') {
      throw new Error(`unknown sql.js provider method: ${method}`);
    }

    if (debugSql) {
      console.warn('[sql.js-worker:start]', { method, args });
    }
    const result = await (fn as (...methodArgs: unknown[]) => Promise<unknown>).apply(queryProvider, args);
    const elapsedMs = performance.now() - started;
    const response: SqlJsWorkerResponse = {
      id,
      ok: true,
      result,
      timing: { method, elapsedMs },
    };
    self.postMessage(response);
  } catch (error) {
    const response: SqlJsWorkerResponse = {
      id,
      ok: false,
      error: serializeProviderError(error),
    };
    self.postMessage(response);
  }
};

export {};
