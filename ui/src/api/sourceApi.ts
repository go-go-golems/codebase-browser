import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { normalizeQueryError } from './queryErrors';
import { apiProvider } from './codebaseProvider';

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
    getSource: b.query<string, string | { path: string; commit?: string }>({
      queryFn: (arg) => {
        const path = typeof arg === 'string' ? arg : arg.path;
        const commit = typeof arg === 'string' ? undefined : arg.commit;
        return providerResult(() => apiProvider().getSource(path, commit));
      },
    }),
    getSnippet: b.query<string, { sym: string; kind?: SnippetKind; commit?: string }>({
      queryFn: ({ sym, kind = 'declaration', commit }) => providerResult(() => apiProvider().getSnippet(sym, kind, commit)),
    }),
    getSnippetRefs: b.query<SnippetRefView[], string>({
      queryFn: (sym) => providerResult(() => apiProvider().getSnippetRefs(sym)),
    }),
    getSourceRefs: b.query<SourceRefView[], string>({
      queryFn: (path) => providerResult(() => apiProvider().getSourceRefs(path)),
    }),
    getFileXref: b.query<FileXrefResponse, string>({
      queryFn: (path) => providerResult(() => apiProvider().getFileXref(path)),
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
