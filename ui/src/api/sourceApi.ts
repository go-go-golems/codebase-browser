import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { normalizeQueryError, type ProviderError } from './queryErrors';
import { getSqlJsProvider } from './sqlJsProviderRegistry';

export type SnippetKind = 'declaration' | 'body' | 'signature';

export interface SnippetRefView {
  toSymbolId: string;
  kind: string;
  offsetInSnippet: number;
  length: number;
}

export interface SourceRefView {
  toSymbolId: string;
  kind: string;
  offset: number;
  length: number;
}

export interface FileXrefRef {
  fromSymbolId: string;
  toSymbolId: string;
  kind: string;
  fileId: string;
  range: { startLine: number; startCol: number; endLine: number; endCol: number; startOffset: number; endOffset: number };
}

export interface FileXrefUseTarget {
  toSymbolId: string;
  kind: string;
  count: number;
  occurrences: FileXrefRef[];
}

export interface FileXrefResponse {
  path: string;
  usedBy: FileXrefRef[];
  uses: FileXrefUseTarget[];
}

const noopBaseQuery: BaseQueryFn<void, unknown, ProviderError> = async () => ({ data: undefined });

async function providerResult<T>(fn: () => Promise<T>): Promise<{ data: T } | { error: ProviderError }> {
  try {
    return { data: await fn() };
  } catch (err) {
    return { error: normalizeQueryError(err) };
  }
}

export const sourceApi = createApi({
  reducerPath: 'sourceApi',
  baseQuery: noopBaseQuery,
  keepUnusedDataFor: 3600,
  endpoints: (b) => ({
    getSource: b.query<string, string>({
      queryFn: (path) => providerResult(() => getSqlJsProvider().getSource(path)),
    }),
    getSnippet: b.query<string, { sym: string; kind?: SnippetKind }>({
      queryFn: ({ sym, kind = 'declaration' }) => providerResult(() => getSqlJsProvider().getSnippet(sym, kind)),
    }),
    getSnippetRefs: b.query<SnippetRefView[], string>({
      queryFn: (sym) => providerResult(() => getSqlJsProvider().getSnippetRefs(sym)),
    }),
    getSourceRefs: b.query<SourceRefView[], string>({
      queryFn: (path) => providerResult(() => getSqlJsProvider().getSourceRefs(path)),
    }),
    getFileXref: b.query<FileXrefResponse, string>({
      queryFn: (path) => providerResult(() => getSqlJsProvider().getFileXref(path)),
    }),
  }),
});

export const {
  useGetSourceQuery,
  useGetSnippetQuery,
  useGetSnippetRefsQuery,
  useGetSourceRefsQuery,
  useGetFileXrefQuery,
} = sourceApi;
