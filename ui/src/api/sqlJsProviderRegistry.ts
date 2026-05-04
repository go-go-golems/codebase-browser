import type { CodebaseQueryProvider } from './queryProvider';
import { SqlJsQueryProvider } from './sqlJsQueryProvider';
import { WorkerSqlJsQueryProvider } from './workerSqlJsQueryProvider';
import { resetSqlJsWorkerForTests } from './sqljs/workerClient';

let provider: CodebaseQueryProvider | null = null;

function shouldUseSqlWorker(): boolean {
  if (typeof window === 'undefined') return false;
  if (typeof Worker === 'undefined') return false;
  const params = new URLSearchParams(window.location.search);
  return !params.has('noSqlWorker');
}

export function getSqlJsProvider(): CodebaseQueryProvider {
  if (!provider) {
    provider = shouldUseSqlWorker() ? new WorkerSqlJsQueryProvider() : new SqlJsQueryProvider();
  }
  return provider;
}

export function resetSqlJsProviderForTests(): void {
  provider = null;
  resetSqlJsWorkerForTests();
}
