import { getLiveApiProvider, isLiveApiAvailable } from './liveApiProvider';
import { getSqlJsProvider } from './sqlJsQueryProvider';

export async function liveOrSql<T>(liveFn: () => Promise<T>, sqlFn: () => Promise<T>): Promise<T> {
  if (await isLiveApiAvailable()) {
    return liveFn();
  }
  return sqlFn();
}

export async function liveWithSqlFallback<T>(liveFn: () => Promise<T>, sqlFn: () => Promise<T>): Promise<T> {
  if (await isLiveApiAvailable()) {
    try {
      return await liveFn();
    } catch {
      return sqlFn();
    }
  }
  return sqlFn();
}

export function liveProvider() {
  return getLiveApiProvider();
}

export function sqlProvider() {
  return getSqlJsProvider();
}
