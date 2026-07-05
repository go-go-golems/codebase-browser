import { QueryError } from './queryErrors';
import type { DocPage, ReviewDocMeta } from './docApi';
import type { BodyDiffResult, CommitDiff, CommitRow, ImpactResponse, SymbolHistoryEntry } from './historyApi';
import type { FileXrefResponse, SnippetKind, SnippetRefView, SourceRefView } from './sourceApi';
import type { IndexSummary, PackageLite, Symbol } from './types';
import type { XrefResponse } from './xrefApi';

let liveAvailablePromise: Promise<boolean> | null = null;

export function resetLiveApiAvailabilityForTests(): void {
  liveAvailablePromise = null;
}

export async function isLiveApiAvailable(): Promise<boolean> {
  if (!liveAvailablePromise) {
    liveAvailablePromise = (async () => {
      try {
        const response = await fetch('/api/health', { cache: 'no-store' });
        if (!response.ok) return false;
        const body = await response.json().catch(() => ({}));
        return body?.mode === 'live-go' || body?.ok === true;
      } catch {
        return false;
      }
    })();
  }
  return liveAvailablePromise;
}

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { cache: 'no-store' });
  if (!response.ok) {
    const text = await response.text().catch(() => '');
    throw new QueryError('SQL_ERROR', text || response.statusText, { status: response.status });
  }
  return (await response.json()) as T;
}

async function fetchText(path: string): Promise<string> {
  const response = await fetch(path, { cache: 'no-store' });
  if (!response.ok) {
    const text = await response.text().catch(() => '');
    throw new QueryError('SQL_ERROR', text || response.statusText, { status: response.status });
  }
  return response.text();
}

function normalizeSymbol(row: any): Symbol {
  return {
    id: row.id,
    kind: row.kind,
    name: row.name,
    packageId: row.packageId,
    fileId: row.fileId,
    range: {
      startLine: Number(row.startLine ?? row.range?.startLine ?? 0),
      startCol: Number(row.startCol ?? row.range?.startCol ?? 0),
      endLine: Number(row.endLine ?? row.range?.endLine ?? 0),
      endCol: Number(row.endCol ?? row.range?.endCol ?? 0),
      startOffset: Number(row.startOffset ?? row.range?.startOffset ?? 0),
      endOffset: Number(row.endOffset ?? row.range?.endOffset ?? 0),
    },
    doc: row.doc ?? '',
    signature: row.signature ?? '',
    receiver: row.receiver ?? (row.receiverType ? { typeName: row.receiverType, pointer: !!row.receiverPointer } : undefined),
    typeParams: row.typeParams ?? [],
    exported: !!row.exported,
    children: row.children,
    tags: row.tags ?? [],
    language: row.language,
  };
}

function normalizeIndex(row: any): IndexSummary {
  return {
    version: row.version ?? 'live-go',
    generatedAt: row.generatedAt ?? '',
    module: row.module ?? '',
    goVersion: row.goVersion ?? '',
    packages: row.packages ?? [],
    files: row.files ?? [],
    symbols: (row.symbols ?? []).map(normalizeSymbol),
  };
}

function normalizeDoc(row: any): DocPage {
  return {
    slug: row.slug,
    title: row.title,
    html: row.html ?? row.markdown ?? '',
    snippets: typeof row.snippetsJson === 'string' ? JSON.parse(row.snippetsJson || '[]') : (row.snippets ?? []),
    errors: typeof row.errorsJson === 'string' ? JSON.parse(row.errorsJson || '[]') : (row.errors ?? []),
  };
}

export class LiveApiProvider {
  async getIndex(): Promise<IndexSummary> {
    return normalizeIndex(await fetchJSON('/api/index'));
  }

  async getPackageLites(): Promise<PackageLite[]> {
    const index = await this.getIndex();
    return index.packages.map((pkg) => ({
      id: pkg.id,
      importPath: pkg.importPath,
      name: pkg.name,
      files: pkg.fileIds.length,
      symbols: pkg.symbolIds.length,
    }));
  }

  async getSymbol(id: string): Promise<Symbol> {
    return normalizeSymbol(await fetchJSON(`/api/symbol?id=${encodeURIComponent(id)}`));
  }

  async searchSymbols(q: string, kind = ''): Promise<Symbol[]> {
    const params = new URLSearchParams({ q });
    if (kind) params.set('kind', kind);
    return (await fetchJSON<any[]>(`/api/search?${params}`)).map(normalizeSymbol);
  }

  async getSource(path: string, commit?: string): Promise<string> {
    const params = new URLSearchParams({ path });
    if (commit) params.set('commit', commit);
    return fetchText(`/api/source?${params}`);
  }

  async getSnippet(symbolId: string, kind: SnippetKind = 'declaration', commit?: string): Promise<string> {
    const params = new URLSearchParams({ symbol: symbolId, kind });
    if (commit) params.set('commit', commit);
    return fetchText(`/api/snippet?${params}`);
  }

  async getSnippetRefs(symbolId: string, commit?: string): Promise<SnippetRefView[]> {
    const params = new URLSearchParams({ symbol: symbolId });
    if (commit) params.set('commit', commit);
    return fetchJSON(`/api/snippet-refs?${params}`);
  }

  async getSourceRefs(path: string, commit?: string): Promise<SourceRefView[]> {
    const params = new URLSearchParams({ path });
    if (commit) params.set('commit', commit);
    return fetchJSON(`/api/source-refs?${params}`);
  }

  async getFileXref(path: string, commit?: string): Promise<FileXrefResponse> {
    const params = new URLSearchParams({ path });
    if (commit) params.set('commit', commit);
    return fetchJSON(`/api/file-xref?${params}`);
  }

  async getXref(symbolId: string, commit?: string): Promise<XrefResponse> {
    const params = new URLSearchParams({ id: symbolId });
    if (commit) params.set('commit', commit);
    return fetchJSON(`/api/xref?${params}`);
  }

  async listReviewDocs(): Promise<ReviewDocMeta[]> {
    return fetchJSON('/api/review-docs');
  }

  async getReviewDoc(slug: string): Promise<DocPage> {
    return normalizeDoc(await fetchJSON(`/api/review-docs/${encodeURIComponent(slug)}`));
  }

  async listCommits(): Promise<CommitRow[]> {
    return fetchJSON('/api/history/commits');
  }

  async getCommit(ref: string): Promise<CommitRow> {
    const commits = await this.listCommits();
    const matches = commits.filter((commit) => commit.Hash === ref || commit.ShortHash === ref || commit.Hash.startsWith(ref));
    if (matches.length === 1) return matches[0];
    if (matches.length > 1) throw new QueryError('AMBIGUOUS_REF', `ambiguous commit ref: ${ref}`);
    throw new QueryError('NOT_FOUND', `commit not found: ${ref}`);
  }

  async getSymbolHistory(symbolId: string, limit?: number): Promise<SymbolHistoryEntry[]> {
    const params = new URLSearchParams({ symbol: symbolId });
    if (limit && limit > 0) params.set('limit', String(limit));
    return fetchJSON(`/api/history/symbol?${params}`);
  }

  async getCommitDiff(from: string, to: string): Promise<CommitDiff> {
    const params = new URLSearchParams({ from, to });
    return fetchJSON(`/api/history/diff?${params}`);
  }

  async getSymbolBodyDiff(from: string, to: string, symbolId: string): Promise<BodyDiffResult> {
    const params = new URLSearchParams({ from, to, symbol: symbolId });
    return fetchJSON(`/api/history/symbol-body-diff?${params}`);
  }

  async getImpact(options: { symbolId: string; direction: 'usedby' | 'uses'; depth: number; commit?: string }): Promise<ImpactResponse> {
    const params = new URLSearchParams({ symbol: options.symbolId, direction: options.direction, depth: String(options.depth) });
    if (options.commit) params.set('commit', options.commit);
    return fetchJSON(`/api/history/impact?${params}`);
  }
}

let liveProvider: LiveApiProvider | null = null;

export function getLiveApiProvider(): LiveApiProvider {
  if (!liveProvider) liveProvider = new LiveApiProvider();
  return liveProvider;
}
