import type { DocPage, ReviewDocMeta } from './docApi';
import type { BodyDiffResult, CommitDiff, CommitRow, ImpactResponse, SymbolHistoryEntry } from './historyApi';
import type { CodebaseQueryProvider, ImpactQueryOptions } from './queryProvider';
import type { FileXrefResponse, SnippetRefView, SourceRefView } from './sourceApi';
import { callSqlJsWorker } from './sqljs/workerClient';
import type { IndexSummary, PackageLite, Symbol } from './types';
import type { XrefResponse } from './xrefApi';

export class WorkerSqlJsQueryProvider implements CodebaseQueryProvider {
  getIndex(): Promise<IndexSummary> {
    return callSqlJsWorker('getIndex', []);
  }

  getPackageLites(): Promise<PackageLite[]> {
    return callSqlJsWorker('getPackageLites', []);
  }

  getSymbol(id: string): Promise<Symbol> {
    return callSqlJsWorker('getSymbol', [id]);
  }

  searchSymbols(query: string, kind = ''): Promise<Symbol[]> {
    return callSqlJsWorker('searchSymbols', [query, kind]);
  }

  getSource(path: string, commitRef = 'HEAD'): Promise<string> {
    return callSqlJsWorker('getSource', [path, commitRef]);
  }

  getSnippet(symbolId: string, kind = 'declaration', commitRef = 'HEAD'): Promise<string> {
    return callSqlJsWorker('getSnippet', [symbolId, kind, commitRef]);
  }

  getSnippetRefs(symbolId: string, commitRef = 'HEAD'): Promise<SnippetRefView[]> {
    return callSqlJsWorker('getSnippetRefs', [symbolId, commitRef]);
  }

  getSourceRefs(path: string, commitRef = 'HEAD'): Promise<SourceRefView[]> {
    return callSqlJsWorker('getSourceRefs', [path, commitRef]);
  }

  getFileXref(path: string, commitRef = 'HEAD'): Promise<FileXrefResponse> {
    return callSqlJsWorker('getFileXref', [path, commitRef]);
  }

  getXref(symbolId: string, commitRef = 'HEAD'): Promise<XrefResponse> {
    return callSqlJsWorker('getXref', [symbolId, commitRef]);
  }

  listCommits(): Promise<CommitRow[]> {
    return callSqlJsWorker('listCommits', []);
  }

  resolveCommitRef(ref: string): Promise<string> {
    return callSqlJsWorker('resolveCommitRef', [ref]);
  }

  getCommit(ref: string): Promise<CommitRow> {
    return callSqlJsWorker('getCommit', [ref]);
  }

  getSymbolHistory(symbolId: string): Promise<SymbolHistoryEntry[]> {
    return callSqlJsWorker('getSymbolHistory', [symbolId]);
  }

  getSymbolBodyDiff(from: string, to: string, symbolId: string): Promise<BodyDiffResult> {
    return callSqlJsWorker('getSymbolBodyDiff', [from, to, symbolId]);
  }

  getCommitDiff(from: string, to: string): Promise<CommitDiff> {
    return callSqlJsWorker('getCommitDiff', [from, to]);
  }

  getImpact(options: ImpactQueryOptions): Promise<ImpactResponse> {
    return callSqlJsWorker('getImpact', [options]);
  }

  listReviewDocs(): Promise<ReviewDocMeta[]> {
    return callSqlJsWorker('listReviewDocs', []);
  }

  getReviewDoc(slug: string): Promise<DocPage> {
    return callSqlJsWorker('getReviewDoc', [slug]);
  }
}
