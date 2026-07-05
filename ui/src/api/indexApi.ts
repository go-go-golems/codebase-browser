import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { normalizeQueryError } from './queryErrors';
import { liveOrSql, liveProvider, sqlProvider } from './codebaseProvider';
import type { IndexSummary, PackageLite, Symbol } from './types';

type ProviderError = { status: string; data?: string };

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
      queryFn: () => providerResult(() => liveOrSql(() => liveProvider().getIndex(), () => sqlProvider().getIndex())),
      providesTags: ['Index'],
    }),
    getPackages: b.query<PackageLite[], void>({
      queryFn: () => providerResult(() => liveOrSql(() => liveProvider().getPackageLites(), () => sqlProvider().getPackageLites())),
      providesTags: ['Package'],
    }),
    getSymbol: b.query<Symbol, string>({
      queryFn: (id) => providerResult(() => liveOrSql(() => liveProvider().getSymbol(id), () => sqlProvider().getSymbol(id))),
      providesTags: (_r, _e, id) => [{ type: 'Symbol', id }],
    }),
    searchSymbols: b.query<Symbol[], { q: string; kind?: string }>({
      queryFn: ({ q, kind }) => providerResult(() => liveOrSql(() => liveProvider().searchSymbols(q, kind ?? ''), () => sqlProvider().searchSymbols(q, kind ?? ''))),
    }),
  }),
});

export const {
  useGetIndexQuery,
  useGetPackagesQuery,
  useGetSymbolQuery,
  useSearchSymbolsQuery,
} = indexApi;
