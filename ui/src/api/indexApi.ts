import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { normalizeQueryError, type ProviderError } from './queryErrors';
import { getSqlJsProvider } from './sqlJsProviderRegistry';
import type { IndexSummary, PackageLite, Symbol } from './types';

const noopBaseQuery: BaseQueryFn<void, unknown, ProviderError> = async () => ({ data: undefined });

async function providerResult<T>(fn: () => Promise<T>): Promise<{ data: T } | { error: ProviderError }> {
  try {
    return { data: await fn() };
  } catch (err) {
    return { error: normalizeQueryError(err) };
  }
}

export const indexApi = createApi({
  reducerPath: 'indexApi',
  baseQuery: noopBaseQuery,
  tagTypes: ['Index', 'Package', 'Symbol'],
  keepUnusedDataFor: 3600,
  endpoints: (b) => ({
    getIndex: b.query<IndexSummary, void>({
      queryFn: () => providerResult(() => getSqlJsProvider().getIndex()),
      providesTags: ['Index'],
    }),
    getPackages: b.query<PackageLite[], void>({
      queryFn: () => providerResult(() => getSqlJsProvider().getPackageLites()),
      providesTags: ['Package'],
    }),
    getSymbol: b.query<Symbol, string>({
      queryFn: (id) => providerResult(() => getSqlJsProvider().getSymbol(id)),
      providesTags: (_r, _e, id) => [{ type: 'Symbol', id }],
    }),
    searchSymbols: b.query<Symbol[], { q: string; kind?: string }>({
      queryFn: ({ q, kind }) => providerResult(() => getSqlJsProvider().searchSymbols(q, kind ?? '')),
    }),
  }),
});

export const {
  useGetIndexQuery,
  useGetPackagesQuery,
  useGetSymbolQuery,
  useSearchSymbolsQuery,
} = indexApi;
