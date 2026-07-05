import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { apiProvider } from './codebaseProvider';
import { normalizeQueryError, type ProviderError } from './queryErrors';
import type { Range } from './types';

export interface RefRecord {
  fromSymbolId: string;
  toSymbolId: string;
  kind: string;
  fileId: string;
  range: Range;
}

export interface XrefUseTarget {
  toSymbolId: string;
  kind: string;
  count: number;
  occurrences: RefRecord[];
}

export interface XrefResponse {
  id: string;
  usedBy: RefRecord[];
  uses: XrefUseTarget[];
}

const noopBaseQuery: BaseQueryFn<void, unknown, ProviderError> = async () => ({ data: undefined });

async function providerResult<T>(fn: () => Promise<T>): Promise<{ data: T } | { error: ProviderError }> {
  try {
    return { data: await fn() };
  } catch (err) {
    return { error: normalizeQueryError(err) };
  }
}

export const xrefApi = createApi({
  reducerPath: 'xrefApi',
  baseQuery: noopBaseQuery,
  keepUnusedDataFor: 3600,
  endpoints: (b) => ({
    getXref: b.query<XrefResponse, string>({
      queryFn: (id) => providerResult(() => apiProvider().getXref(id)),
    }),
  }),
});

export const { useGetXrefQuery } = xrefApi;
