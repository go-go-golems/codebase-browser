import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { normalizeQueryError } from './queryErrors';
import { liveOrSql, liveProvider, sqlProvider } from './codebaseProvider';

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

type ProviderError = { status: string; data?: string };

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
      queryFn: (path) => providerResult(() => liveOrSql(() => liveProvider().getSource(path), () => sqlProvider().getSource(path))),
    }),
    getSnippet: b.query<string, { sym: string; kind?: SnippetKind }>({
      queryFn: ({ sym, kind = 'declaration' }) => providerResult(() => liveOrSql(() => liveProvider().getSnippet(sym, kind), () => sqlProvider().getSnippet(sym, kind))),
    }),
    getSnippetRefs: b.query<SnippetRefView[], string>({
      queryFn: (sym) => providerResult(() => sqlProvider().getSnippetRefs(sym)),
    }),
    getSourceRefs: b.query<SourceRefView[], string>({
      queryFn: (path) => providerResult(() => sqlProvider().getSourceRefs(path)),
    }),
    getFileXref: b.query<FileXrefResponse, string>({
      queryFn: (path) => providerResult(() => sqlProvider().getFileXref(path)),
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
