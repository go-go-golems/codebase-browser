import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { normalizeQueryError } from './queryErrors';
import { getSqlJsProvider } from './sqlJsQueryProvider';

export interface CommitRow {
  Hash: string;
  ShortHash: string;
  Message: string;
  AuthorName: string;
  AuthorEmail: string;
  AuthorTime: number;
  IndexedAt: number;
  Sequence: number;
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

type ProviderError = { status: string; data?: string };

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
      queryFn: () => providerResult(() => getSqlJsProvider().listCommits()),
    }),
    getCommit: builder.query<CommitRow, string>({
      queryFn: (hash) => providerResult(() => getSqlJsProvider().getCommit(hash)),
    }),
    getCommitSymbols: builder.query<SymbolAtCommit[], string>({
      queryFn: async () => ({ data: [] }),
    }),
    getDiff: builder.query<CommitDiff, { from: string; to: string }>({
      queryFn: ({ from, to }) => providerResult(() => getSqlJsProvider().getCommitDiff(from, to)),
    }),
    getSymbolHistory: builder.query<SymbolHistoryEntry[], { symbolId: string; limit?: number }>({
      queryFn: ({ symbolId, limit }) =>
        providerResult(async () => {
          const entries = await getSqlJsProvider().getSymbolHistory(symbolId);
          return limit && limit > 0 ? entries.slice(0, limit) : entries;
        }),
    }),
    getSymbolBodyDiff: builder.query<BodyDiffResult, { from: string; to: string; symbolId: string }>({
      queryFn: ({ from, to, symbolId }) =>
        providerResult(() => getSqlJsProvider().getSymbolBodyDiff(from, to, symbolId)),
    }),
    getImpact: builder.query<ImpactResponse, { sym: string; dir?: 'usedby' | 'uses'; depth?: number; commit?: string }>({
      queryFn: ({ sym, dir = 'usedby', depth = 2, commit }) =>
        providerResult(() => getSqlJsProvider().getImpact({ symbolId: sym, direction: dir, depth, commit })),
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
