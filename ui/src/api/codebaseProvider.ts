import { getLiveApiProvider, isLiveApiAvailable } from './liveApiProvider';

export { isLiveApiAvailable };

export function apiProvider() {
  return getLiveApiProvider();
}

export function liveProvider() {
  return getLiveApiProvider();
}
