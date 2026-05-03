import type { DocPage, ReviewDocMeta } from './docApi';
import type { FileXrefResponse, SnippetRefView, SourceRefView } from './sourceApi';
import type { IndexSummary, PackageLite, Symbol } from './types';
import type { XrefResponse } from './xrefApi';
import type {
  BodyDiffResult,
  CommitDiff,
  CommitRow,
  ImpactResponse,
  SymbolHistoryEntry,
} from './historyApi';

export interface ImpactQueryOptions {
  symbolId: string;
  direction: 'usedby' | 'uses';
  depth: number;
  commit?: string;
}

export interface CodebaseQueryProvider {
  getIndex(): Promise<IndexSummary>;
  getPackageLites(): Promise<PackageLite[]>;
  getSymbol(id: string): Promise<Symbol>;
  searchSymbols(query: string, kind?: string): Promise<Symbol[]>;

  getSource(path: string, commitRef?: string): Promise<string>;
  getSnippet(symbolId: string, kind?: string, commitRef?: string): Promise<string>;
  getSnippetRefs(symbolId: string, commitRef?: string): Promise<SnippetRefView[]>;
  getSourceRefs(path: string, commitRef?: string): Promise<SourceRefView[]>;
  getFileXref(path: string, commitRef?: string): Promise<FileXrefResponse>;
  getXref(symbolId: string, commitRef?: string): Promise<XrefResponse>;

  listCommits(): Promise<CommitRow[]>;
  resolveCommitRef(ref: string): Promise<string>;
  getCommit(ref: string): Promise<CommitRow>;
  getSymbolHistory(symbolId: string): Promise<SymbolHistoryEntry[]>;
  getSymbolBodyDiff(from: string, to: string, symbolId: string): Promise<BodyDiffResult>;
  getCommitDiff(from: string, to: string): Promise<CommitDiff>;
  getImpact(options: ImpactQueryOptions): Promise<ImpactResponse>;

  listReviewDocs(): Promise<ReviewDocMeta[]>;
  getReviewDoc(slug: string): Promise<DocPage>;
}
