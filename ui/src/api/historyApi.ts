import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { normalizeQueryError, type ProviderError } from './queryErrors';
import { apiProvider } from './codebaseProvider';

export interface CommitRow {
  Hash: string;
  ShortHash: string;
  Message: string;
  AuthorName: string;
  AuthorEmail: string;
  AuthorTime: number;
  IndexedAt: number;
  Branch: string;
  Error: string;
}

export interface SymbolAtCommit {
  id: string;
  kind: string;
  name: string;
  packageId: string;
  fileId: string;
  startLine: number;
  endLine: number;
  signature: string;
  exported: boolean;
  bodyHash: string;
}

export interface SymbolHistoryEntry {
  commitHash: string;
  shortHash: string;
  message: string;
  authorTime: number;
  bodyHash: string;
  startLine: number;
  endLine: number;
  signature: string;
  kind: string;
}

export interface FileDiff {
  FileID: string;
  Path: string;
  ChangeType: string;
  OldSHA256: string;
  NewSHA256: string;
}

export interface SymbolDiff {
  SymbolID: string;
  Name: string;
  Kind: string;
  PackageID: string;
  ChangeType: string;
  OldStartLine: number;
  OldEndLine: number;
  NewStartLine: number;
  NewEndLine: number;
  OldSignature: string;
  NewSignature: string;
  OldBodyHash: string;
  NewBodyHash: string;
}

export interface DiffStats {
  FilesAdded: number;
  FilesRemoved: number;
  FilesModified: number;
  SymbolsAdded: number;
  SymbolsRemoved: number;
  SymbolsModified: number;
  SymbolsMoved: number;
  SymbolsUnchanged: number;
}

export interface CommitDiff {
  OldHash: string;
  NewHash: string;
  Files: FileDiff[];
  Symbols: SymbolDiff[];
  Stats: DiffStats;
}

export interface BodyDiffResult {
  symbolId: string;
  name: string;
  oldCommit: string;
  newCommit: string;
  oldBody: string;
  newBody: string;
  unifiedDiff: string;
  oldRange: string;
  newRange: string;
}

export interface ImpactEdge {
  fromSymbolId: string;
  toSymbolId: string;
  kind: string;
  fileId: string;
}

export interface ImpactNode {
  symbolId: string;
  name: string;
  kind: string;
  depth: number;
  edges: ImpactEdge[];
  compatibility: string;
  local: boolean;
}

export interface ImpactResponse {
  root: string;
  direction: string;
  depth: number;
  commit: string;
  nodes: ImpactNode[];
}

const noopBaseQuery: BaseQueryFn<void, unknown, ProviderError> = async () => ({ data: undefined });

async function providerResult<T>(fn: () => Promise<T>): Promise<{ data: T } | { error: ProviderError }> {
  try {
    return { data: await fn() };
  } catch (err) {
    return { error: normalizeQueryError(err) };
  }
}

export const historyApi = createApi({
  reducerPath: 'historyApi',
  baseQuery: noopBaseQuery,
  endpoints: (builder) => ({
    listCommits: builder.query<CommitRow[], void>({
      queryFn: () => providerResult(() => apiProvider().listCommits()),
    }),
    getCommit: builder.query<CommitRow, string>({
      queryFn: (hash) => providerResult(() => apiProvider().getCommit(hash)),
    }),
    getCommitSymbols: builder.query<SymbolAtCommit[], string>({
      queryFn: async () => ({ data: [] }),
    }),
    getDiff: builder.query<CommitDiff, { from: string; to: string }>({
      queryFn: ({ from, to }) => providerResult(() => apiProvider().getCommitDiff(from, to)),
    }),
    getSymbolHistory: builder.query<SymbolHistoryEntry[], { symbolId: string; limit?: number }>({
      queryFn: ({ symbolId, limit }) => providerResult(() => apiProvider().getSymbolHistory(symbolId, limit)),
    }),
    getSymbolBodyDiff: builder.query<BodyDiffResult, { from: string; to: string; symbolId: string }>({
      queryFn: ({ from, to, symbolId }) => providerResult(() => apiProvider().getSymbolBodyDiff(from, to, symbolId)),
    }),
    getImpact: builder.query<ImpactResponse, { sym: string; dir?: 'usedby' | 'uses'; depth?: number; commit?: string }>({
      queryFn: ({ sym, dir = 'usedby', depth = 2, commit }) =>
        providerResult(() => apiProvider().getImpact({ symbolId: sym, direction: dir, depth, commit })),
    }),
  }),
});

export const {
  useListCommitsQuery,
  useGetCommitQuery,
  useGetCommitSymbolsQuery,
  useGetDiffQuery,
  useGetSymbolHistoryQuery,
  useGetSymbolBodyDiffQuery,
  useGetImpactQuery,
} = historyApi;
