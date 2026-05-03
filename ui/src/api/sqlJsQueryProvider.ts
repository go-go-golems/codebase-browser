import type { DocPage, ReviewDiagnostic, ReviewDocMeta, ReviewPageBlock, SnippetRef } from './docApi';
import type { FileXrefResponse, FileXrefUseTarget, SnippetRefView, SourceRefView } from './sourceApi';
import type { File, IndexSummary, Package, PackageLite, Symbol } from './types';
import type { RefRecord, XrefResponse, XrefUseTarget } from './xrefApi';
import type {
  BodyDiffResult,
  CommitDiff,
  CommitRow,
  DiffStats,
  FileDiff,
  ImpactEdge,
  ImpactNode,
  ImpactResponse,
  SymbolDiff,
  SymbolHistoryEntry,
} from './historyApi';
import type initSqlJs from 'sql.js';

import { QueryError } from './queryErrors';
import { getStaticDb } from './sqljs/sqlJsDb';
import { extractUtf8Range, queryAll, queryOne, sqlBlobToBytes, type SqlRow } from './sqljs/sqlRows';

type Database = initSqlJs.Database;
type DbLoader = () => Promise<Database>;

type CommitRowSQL = SqlRow & {
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
};

type SymbolHistorySQL = SqlRow & {
  symbolId: string;
  name: string;
  kind: string;
  packageId: string;
  commitHash: string;
  shortHash: string;
  message: string;
  authorTime: number;
  bodyHash: string;
  startLine: number;
  endLine: number;
  signature: string;
  fileId: string;
};

type BodyMetaSQL = SqlRow & {
  symbolId: string;
  name: string;
  fileId: string;
  signature: string;
  startOffset: number;
  endOffset: number;
  startLine: number;
  endLine: number;
  filePath: string;
  contentHash: string;
};

type ContentSQL = SqlRow & {
  content: Uint8Array;
};

type FileContentMetaSQL = SqlRow & {
  fileId: string;
  contentHash: string;
};

type FileDiffSQL = SqlRow & FileDiff;

type SymbolDiffSQL = SqlRow & SymbolDiff;

type RefRecordSQL = SqlRow & {
  fromSymbolId: string;
  toSymbolId: string;
  kind: string;
  fileId: string;
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
  startOffset: number;
  endOffset: number;
};

type SymbolMetaSQL = SqlRow & {
  id: string;
  name: string;
  kind: string;
};

type ReviewDocMetaSQL = SqlRow & ReviewDocMeta;

type ReviewPageSQL = SqlRow & {
  slug: string;
  title: string;
  blocksJson: string;
  diagnosticsJson: string;
};

type ReviewSnippetSQL = SqlRow & {
  stubId: string;
  directive: string;
  symbolId: string | null;
  filePath: string | null;
  kind: string | null;
  language: string | null;
  text: string;
  paramsJson: string;
  startLine: number;
  endLine: number;
  commitHash: string | null;
};

type PackageSQL = SqlRow & {
  id: string;
  importPath: string;
  name: string;
  doc: string;
  language: string;
};

type FileSQL = SqlRow & {
  id: string;
  path: string;
  packageId: string;
  size: number;
  lineCount: number;
  sha256: string;
  language: string;
  buildTagsJson: string;
};

type SymbolSQL = SqlRow & {
  id: string;
  kind: string;
  name: string;
  packageId: string;
  fileId: string;
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
  startOffset: number;
  endOffset: number;
  doc: string;
  signature: string;
  receiverType: string;
  receiverPointer: number;
  exported: number;
  language: string;
  typeParamsJson: string;
  tagsJson: string;
};

let provider: SqlJsQueryProvider | null = null;

export function getSqlJsProvider(): SqlJsQueryProvider {
  if (!provider) provider = new SqlJsQueryProvider();
  return provider;
}

export function resetSqlJsProviderForTests(): void {
  provider = null;
}

export class SqlJsQueryProvider {
  constructor(private readonly loadDb: DbLoader = getStaticDb) {}

  private async getDb(): Promise<Database> {
    return this.loadDb();
  }

  async getIndex(): Promise<IndexSummary> {
    const commit = await this.resolveCommitRef('HEAD');
    const [packages, files, symbols] = await Promise.all([
      this.getPackagesAtCommit(commit),
      this.getFilesAtCommit(commit),
      this.getSymbolsAtCommit(commit),
    ]);
    return {
      version: 'sqljs-static',
      generatedAt: '',
      module: '',
      goVersion: '',
      packages: packages.map((pkg) => ({
        ...pkg,
        fileIds: files.filter((file) => file.packageId === pkg.id).map((file) => file.id),
        symbolIds: symbols.filter((symbol) => symbol.packageId === pkg.id).map((symbol) => symbol.id),
      })),
      files,
      symbols,
    };
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
    const commit = await this.resolveCommitRef('HEAD');
    const db = await this.getDb();
    const row = queryOne<SymbolSQL>(db, symbolSelectSQL + ' WHERE commit_hash = ? AND id = ?', [commit, id]);
    if (!row) throw new QueryError('NOT_FOUND', `symbol not found: ${id}`);
    return toSymbol(row);
  }

  async searchSymbols(query: string, kind = ''): Promise<Symbol[]> {
    const commit = await this.resolveCommitRef('HEAD');
    const db = await this.getDb();
    const like = `%${query}%`;
    return queryAll<SymbolSQL>(db, symbolSelectSQL + `
      WHERE commit_hash = ?
        AND (? = '' OR kind = ?)
        AND (? = '' OR name LIKE ? OR id LIKE ? OR signature LIKE ?)
      ORDER BY name
      LIMIT 100
    `, [commit, kind, kind, query, like, like, like]).map(toSymbol);
  }

  async getSource(path: string, commitRef = 'HEAD'): Promise<string> {
    const commit = await this.resolveCommitRef(commitRef);
    const meta = await this.getFileContentMeta(path, commit);
    const bytes = await this.getContentBytes(meta.contentHash);
    return new TextDecoder('utf-8').decode(bytes);
  }

  async getSnippet(symbolId: string, kind = 'declaration', commitRef = 'HEAD'): Promise<string> {
    const commit = await this.resolveCommitRef(commitRef);
    const meta = await this.getBodyMeta(commit, symbolId);
    if (kind === 'signature') return meta.signature || meta.name;
    const bytes = await this.getContentBytes(meta.contentHash);
    return extractUtf8Range(bytes, meta.startOffset, meta.endOffset);
  }

  async getSnippetRefs(symbolId: string, commitRef = 'HEAD'): Promise<SnippetRefView[]> {
    const commit = await this.resolveCommitRef(commitRef);
    const meta = await this.getBodyMeta(commit, symbolId);
    const refs = await this.getRefRecordsInFileRange(commit, meta.fileId, meta.startOffset, meta.endOffset);
    return refs.map((ref) => ({
      toSymbolId: ref.toSymbolId,
      kind: ref.kind,
      offsetInSnippet: Math.max(0, ref.range.startOffset - meta.startOffset),
      length: Math.max(0, ref.range.endOffset - ref.range.startOffset),
    }));
  }

  async getSourceRefs(path: string, commitRef = 'HEAD'): Promise<SourceRefView[]> {
    const commit = await this.resolveCommitRef(commitRef);
    const meta = await this.getFileContentMeta(path, commit);
    const refs = await this.getRefRecordsInFile(commit, meta.fileId);
    return refs.map((ref) => ({
      toSymbolId: ref.toSymbolId,
      kind: ref.kind,
      offset: ref.range.startOffset,
      length: Math.max(0, ref.range.endOffset - ref.range.startOffset),
    }));
  }

  async getFileXref(path: string, commitRef = 'HEAD'): Promise<FileXrefResponse> {
    const commit = await this.resolveCommitRef(commitRef);
    const meta = await this.getFileContentMeta(path, commit);
    const [usedBy, usesFlat] = await Promise.all([
      this.getRefRecordsToFileSymbols(commit, meta.fileId),
      this.getRefRecordsFromFileSymbols(commit, meta.fileId),
    ]);
    const usesByTarget = new Map<string, FileXrefUseTarget>();
    for (const ref of usesFlat) {
      const key = `${ref.toSymbolId}|${ref.kind}`;
      const target = usesByTarget.get(key) ?? {
        toSymbolId: ref.toSymbolId,
        kind: ref.kind,
        count: 0,
        occurrences: [],
      };
      target.count++;
      target.occurrences.push(ref);
      usesByTarget.set(key, target);
    }
    return { path, usedBy, uses: [...usesByTarget.values()] };
  }

  async getXref(symbolId: string, commitRef = 'HEAD'): Promise<XrefResponse> {
    const commit = await this.resolveCommitRef(commitRef);
    const [usedBy, usesFlat] = await Promise.all([
      this.getRefRecordsTo(symbolId, commit),
      this.getRefRecordsFrom(symbolId, commit),
    ]);
    const usesByTarget = new Map<string, XrefUseTarget>();
    for (const ref of usesFlat) {
      const key = `${ref.toSymbolId}|${ref.kind}`;
      const target = usesByTarget.get(key) ?? {
        toSymbolId: ref.toSymbolId,
        kind: ref.kind,
        count: 0,
        occurrences: [],
      };
      target.count++;
      target.occurrences.push(ref);
      usesByTarget.set(key, target);
    }
    return { id: symbolId, usedBy, uses: [...usesByTarget.values()] };
  }

  async listCommits(): Promise<CommitRow[]> {
    const db = await this.getDb();
    return queryAll<CommitRowSQL>(db, `
      SELECT hash AS Hash,
             short_hash AS ShortHash,
             message AS Message,
             author_name AS AuthorName,
             author_email AS AuthorEmail,
             author_time AS AuthorTime,
             indexed_at AS IndexedAt,
             sequence AS Sequence,
             branch AS Branch,
             error AS Error
      FROM commits
      WHERE error = ''
      ORDER BY sequence DESC, author_time DESC
    `).map((row) => ({ ...row }));
  }

  async resolveCommitRef(ref: string): Promise<string> {
    const commits = await this.listCommits();
    if (commits.length === 0) throw new QueryError('NOT_FOUND', 'no indexed commits in database');

    const ordered = [...commits].sort((a, b) => (a.Sequence - b.Sequence) || (a.AuthorTime - b.AuthorTime));
    const newestIndex = ordered.length - 1;
    const newest = ordered[newestIndex];
    const oldest = ordered[0];
    const commitContext = {
      requestedRef: ref,
      indexedCommitCount: ordered.length,
      supportedHeadRefs: ordered.length === 1 ? ['HEAD'] : [`HEAD..HEAD~${ordered.length - 1}`],
      newest: newest ? { hash: newest.Hash, shortHash: newest.ShortHash, message: newest.Message } : undefined,
      oldest: oldest ? { hash: oldest.Hash, shortHash: oldest.ShortHash, message: oldest.Message } : undefined,
    };
    if (!ref || ref === 'HEAD') return newest.Hash;

    const headOffset = ref.match(/^HEAD~(\d+)$/);
    if (headOffset) {
      const offset = Number.parseInt(headOffset[1], 10);
      const index = newestIndex - offset;
      const commit = ordered[index];
      if (!commit) {
        throw new QueryError(
          'NOT_FOUND',
          `commit ref ${ref} is outside this export's indexed range (${ordered.length} commit${ordered.length === 1 ? '' : 's'} available; deepest supported ref is HEAD~${ordered.length - 1})`,
          { ...commitContext, requestedOffset: offset },
        );
      }
      return commit.Hash;
    }

    const exact = ordered.find((commit) => commit.Hash === ref || commit.ShortHash === ref);
    if (exact) return exact.Hash;

    const prefixMatches = ordered.filter((commit) => commit.Hash.startsWith(ref));
    if (prefixMatches.length === 1) return prefixMatches[0].Hash;
    if (prefixMatches.length > 1) {
      throw new QueryError('AMBIGUOUS_REF', `ambiguous commit ref: ${ref}`, {
        ...commitContext,
        matches: prefixMatches.map((commit) => ({ hash: commit.Hash, shortHash: commit.ShortHash, message: commit.Message })),
      });
    }

    throw new QueryError(
      'NOT_FOUND',
      `commit ref ${ref} was not found in this static export (${ordered.length} indexed commit${ordered.length === 1 ? '' : 's'})`,
      commitContext,
    );
  }

  async getCommit(ref: string): Promise<CommitRow> {
    const hash = await this.resolveCommitRef(ref);
    const commits = await this.listCommits();
    const commit = commits.find((row) => row.Hash === hash);
    if (!commit) throw new QueryError('NOT_FOUND', `commit not found: ${ref}`);
    return commit;
  }

  async getSymbolHistory(symbolId: string): Promise<SymbolHistoryEntry[]> {
    const db = await this.getDb();
    return queryAll<SymbolHistorySQL>(db, `
      SELECT symbol_id AS symbolId,
             name,
             kind,
             package_id AS packageId,
             commit_hash AS commitHash,
             short_hash AS shortHash,
             commit_message AS message,
             author_time AS authorTime,
             body_hash AS bodyHash,
             start_line AS startLine,
             end_line AS endLine,
             signature,
             file_id AS fileId
      FROM symbol_history
      WHERE symbol_id = ?
      ORDER BY author_time DESC
    `, [symbolId]).map((row) => ({
      commitHash: row.commitHash,
      shortHash: row.shortHash,
      message: row.message,
      authorTime: row.authorTime,
      bodyHash: row.bodyHash,
      startLine: row.startLine,
      endLine: row.endLine,
      signature: row.signature,
      kind: row.kind,
    }));
  }

  async getSymbolBodyDiff(from: string, to: string, symbolId: string): Promise<BodyDiffResult> {
    const oldHash = await this.resolveCommitRef(from);
    const newHash = await this.resolveCommitRef(to);
    const [oldMeta, newMeta] = await Promise.all([
      this.getBodyMeta(oldHash, symbolId),
      this.getBodyMeta(newHash, symbolId),
    ]);
    const [oldBytes, newBytes] = await Promise.all([
      this.getContentBytes(oldMeta.contentHash),
      this.getContentBytes(newMeta.contentHash),
    ]);
    const oldBody = extractUtf8Range(oldBytes, oldMeta.startOffset, oldMeta.endOffset);
    const newBody = extractUtf8Range(newBytes, newMeta.startOffset, newMeta.endOffset);

    return {
      symbolId,
      name: newMeta.name || oldMeta.name,
      oldCommit: oldHash,
      newCommit: newHash,
      oldBody,
      newBody,
      unifiedDiff: simpleUnifiedDiff(oldBody, newBody),
      oldRange: `lines ${oldMeta.startLine}-${oldMeta.endLine}`,
      newRange: `lines ${newMeta.startLine}-${newMeta.endLine}`,
    };
  }

  async getCommitDiff(from: string, to: string): Promise<CommitDiff> {
    const oldHash = await this.resolveCommitRef(from);
    const newHash = await this.resolveCommitRef(to);
    const db = await this.getDb();
    const files = queryAll<FileDiffSQL>(db, fileDiffSQL, [oldHash, newHash, newHash, oldHash, oldHash, newHash]);
    const symbols = queryAll<SymbolDiffSQL>(db, symbolDiffSQL, [oldHash, newHash, newHash, oldHash, oldHash, newHash]);
    return {
      OldHash: oldHash,
      NewHash: newHash,
      Files: files.map((row) => ({ ...row })),
      Symbols: symbols.map((row) => ({ ...row })),
      Stats: diffStats(files, symbols),
    };
  }

  async getImpact(options: {
    symbolId: string;
    direction: 'usedby' | 'uses';
    depth: number;
    commit?: string;
  }): Promise<ImpactResponse> {
    const commit = await this.resolveCommitRef(options.commit ?? 'HEAD');
    const direction = options.direction;
    const maxDepth = Math.max(1, Math.min(options.depth, 5));
    const visited = new Set<string>([options.symbolId]);
    const queue: Array<{ symbolId: string; depth: number }> = [{ symbolId: options.symbolId, depth: 0 }];
    const nodeByID = new Map<string, ImpactNode>();

    while (queue.length > 0) {
      const item = queue.shift();
      if (!item || item.depth >= maxDepth) continue;
      const edges = direction === 'uses'
        ? await this.getRefsFrom(item.symbolId, commit)
        : await this.getRefsTo(item.symbolId, commit);
      for (const edge of edges) {
        const nextID = direction === 'uses' ? edge.toSymbolId : edge.fromSymbolId;
        const nextDepth = item.depth + 1;
        let node = nodeByID.get(nextID);
        if (!node) {
          const meta = await this.getSymbolMeta(nextID, commit);
          node = {
            symbolId: nextID,
            name: meta?.name ?? fallbackName(nextID),
            kind: meta?.kind ?? 'external',
            depth: nextDepth,
            edges: [],
            compatibility: 'unknown',
            local: !!meta,
          };
          nodeByID.set(nextID, node);
        }
        node.edges.push(edge);
        if (!visited.has(nextID)) {
          visited.add(nextID);
          queue.push({ symbolId: nextID, depth: nextDepth });
        }
      }
    }

    return {
      root: options.symbolId,
      direction,
      depth: maxDepth,
      commit,
      nodes: [...nodeByID.values()],
    };
  }

  async listReviewDocs(): Promise<ReviewDocMeta[]> {
    const db = await this.getDb();
    return queryAll<ReviewDocMetaSQL>(db, `
      SELECT slug, title
      FROM static_review_pages
      ORDER BY slug
    `).map((row) => ({ slug: row.slug, title: row.title }));
  }

  async getReviewDoc(slug: string): Promise<DocPage> {
    const db = await this.getDb();
    const row = queryOne<ReviewPageSQL>(db, `
      SELECT slug,
             title,
             blocks_json AS blocksJson,
             diagnostics_json AS diagnosticsJson
      FROM static_review_pages
      WHERE slug = ?
    `, [slug]);
    if (!row) throw new QueryError('NOT_FOUND', `review doc not found: ${slug}`);
    const snippets = queryAll<ReviewSnippetSQL>(db, `
      SELECT s.stub_id AS stubId,
             s.directive,
             s.symbol_id AS symbolId,
             s.file_path AS filePath,
             s.kind,
             s.language,
             s.text,
             s.params_json AS paramsJson,
             s.start_line AS startLine,
             s.end_line AS endLine,
             s.commit_hash AS commitHash
      FROM review_docs d
      JOIN review_doc_snippets s ON s.doc_id = d.id
      WHERE d.slug = ?
      ORDER BY s.id
    `, [slug]).map(toSnippetRef);
    const diagnostics = parseJSON<ReviewDiagnostic[]>(row.diagnosticsJson, []);
    return {
      slug: row.slug,
      title: row.title,
      html: '',
      blocks: parseJSON<ReviewPageBlock[]>(row.blocksJson, []),
      snippets,
      diagnostics,
      errors: diagnostics.map((diagnostic) => diagnostic.message),
    };
  }

  private async getPackagesAtCommit(commit: string): Promise<Package[]> {
    const db = await this.getDb();
    return queryAll<PackageSQL>(db, `
      SELECT id,
             import_path AS importPath,
             name,
             doc,
             language
      FROM snapshot_packages
      WHERE commit_hash = ?
      ORDER BY import_path
    `, [commit]).map((row) => ({
      id: row.id,
      importPath: row.importPath,
      name: row.name,
      doc: row.doc,
      language: row.language,
      fileIds: [],
      symbolIds: [],
    }));
  }

  private async getFilesAtCommit(commit: string): Promise<File[]> {
    const db = await this.getDb();
    return queryAll<FileSQL>(db, `
      SELECT id,
             path,
             package_id AS packageId,
             size,
             line_count AS lineCount,
             sha256,
             language,
             build_tags_json AS buildTagsJson
      FROM snapshot_files
      WHERE commit_hash = ?
      ORDER BY path
    `, [commit]).map((row) => ({
      id: row.id,
      path: row.path,
      packageId: row.packageId,
      size: row.size,
      lineCount: row.lineCount,
      sha256: row.sha256,
      language: row.language,
      buildTags: parseJSON<string[]>(row.buildTagsJson, []),
    }));
  }

  private async getSymbolsAtCommit(commit: string): Promise<Symbol[]> {
    const db = await this.getDb();
    return queryAll<SymbolSQL>(db, symbolSelectSQL + `
      WHERE commit_hash = ?
      ORDER BY package_id, file_id, start_line, name
    `, [commit]).map(toSymbol);
  }

  private async getFileContentMeta(path: string, commit: string): Promise<FileContentMetaSQL> {
    const db = await this.getDb();
    const row = queryOne<FileContentMetaSQL>(db, `
      SELECT id AS fileId,
             sha256 AS contentHash
      FROM snapshot_files
      WHERE commit_hash = ? AND path = ?
    `, [commit, path]);
    if (!row) throw new QueryError('NOT_FOUND', `source file not found: ${path}`);
    return row;
  }

  private async getBodyMeta(commitHash: string, symbolId: string): Promise<BodyMetaSQL> {
    const db = await this.getDb();
    const row = queryOne<BodyMetaSQL>(db, `
      SELECT s.id AS symbolId,
             s.name AS name,
             s.start_offset AS startOffset,
             s.end_offset AS endOffset,
             s.start_line AS startLine,
             s.end_line AS endLine,
             s.file_id AS fileId,
             s.signature AS signature,
             f.path AS filePath,
             f.sha256 AS contentHash
      FROM snapshot_symbols s
      JOIN snapshot_files f
        ON f.commit_hash = s.commit_hash
       AND f.id = s.file_id
      WHERE s.commit_hash = ? AND s.id = ?
    `, [commitHash, symbolId]);
    if (!row) throw new QueryError('NOT_FOUND', `symbol ${symbolId} not found at ${commitHash.slice(0, 7)}`);
    return row;
  }

  private async getContentBytes(contentHash: string): Promise<Uint8Array> {
    const db = await this.getDb();
    const row = queryOne<ContentSQL>(db, `
      SELECT content
      FROM file_contents
      WHERE content_hash = ?
    `, [contentHash]);
    if (!row) throw new QueryError('NOT_FOUND', `file content not found: ${contentHash}`);
    return sqlBlobToBytes(row.content);
  }

  private async getRefsFrom(symbolId: string, commit: string): Promise<ImpactEdge[]> {
    return (await this.getRefRecordsFrom(symbolId, commit)).map(({ fromSymbolId, toSymbolId, kind, fileId }) => ({
      fromSymbolId,
      toSymbolId,
      kind,
      fileId,
    }));
  }

  private async getRefsTo(symbolId: string, commit: string): Promise<ImpactEdge[]> {
    return (await this.getRefRecordsTo(symbolId, commit)).map(({ fromSymbolId, toSymbolId, kind, fileId }) => ({
      fromSymbolId,
      toSymbolId,
      kind,
      fileId,
    }));
  }

  private async getRefRecordsFrom(symbolId: string, commit: string): Promise<RefRecord[]> {
    const db = await this.getDb();
    return queryAll<RefRecordSQL>(db, refRecordSelectSQL + `
      WHERE commit_hash = ? AND from_symbol_id = ?
      ORDER BY to_symbol_id, kind
    `, [commit, symbolId]).map(toRefRecord);
  }

  private async getRefRecordsTo(symbolId: string, commit: string): Promise<RefRecord[]> {
    const db = await this.getDb();
    return queryAll<RefRecordSQL>(db, refRecordSelectSQL + `
      WHERE commit_hash = ? AND to_symbol_id = ?
      ORDER BY from_symbol_id, kind
    `, [commit, symbolId]).map(toRefRecord);
  }

  private async getRefRecordsInFile(commit: string, fileId: string): Promise<RefRecord[]> {
    const db = await this.getDb();
    return queryAll<RefRecordSQL>(db, refRecordSelectSQL + `
      WHERE commit_hash = ? AND file_id = ?
      ORDER BY start_offset, end_offset
    `, [commit, fileId]).map(toRefRecord);
  }

  private async getRefRecordsInFileRange(commit: string, fileId: string, startOffset: number, endOffset: number): Promise<RefRecord[]> {
    const db = await this.getDb();
    return queryAll<RefRecordSQL>(db, refRecordSelectSQL + `
      WHERE commit_hash = ?
        AND file_id = ?
        AND start_offset >= ?
        AND end_offset <= ?
      ORDER BY start_offset, end_offset
    `, [commit, fileId, startOffset, endOffset]).map(toRefRecord);
  }

  private async getRefRecordsToFileSymbols(commit: string, fileId: string): Promise<RefRecord[]> {
    const db = await this.getDb();
    return queryAll<RefRecordSQL>(db, refRecordSelectSQLWithAlias + `
      JOIN snapshot_symbols target
        ON target.commit_hash = r.commit_hash
       AND target.id = r.to_symbol_id
      LEFT JOIN snapshot_symbols source
        ON source.commit_hash = r.commit_hash
       AND source.id = r.from_symbol_id
      WHERE r.commit_hash = ?
        AND target.file_id = ?
        AND COALESCE(source.file_id, '') != ?
      ORDER BY r.from_symbol_id, r.kind
    `, [commit, fileId, fileId]).map(toRefRecord);
  }

  private async getRefRecordsFromFileSymbols(commit: string, fileId: string): Promise<RefRecord[]> {
    const db = await this.getDb();
    return queryAll<RefRecordSQL>(db, refRecordSelectSQLWithAlias + `
      JOIN snapshot_symbols source
        ON source.commit_hash = r.commit_hash
       AND source.id = r.from_symbol_id
      LEFT JOIN snapshot_symbols target
        ON target.commit_hash = r.commit_hash
       AND target.id = r.to_symbol_id
      WHERE r.commit_hash = ?
        AND source.file_id = ?
        AND COALESCE(target.file_id, '') != ?
      ORDER BY r.to_symbol_id, r.kind
    `, [commit, fileId, fileId]).map(toRefRecord);
  }

  private async getSymbolMeta(symbolId: string, commit: string): Promise<SymbolMetaSQL | null> {
    const db = await this.getDb();
    return queryOne<SymbolMetaSQL>(db, `
      SELECT id, name, kind
      FROM snapshot_symbols
      WHERE commit_hash = ? AND id = ?
    `, [commit, symbolId]);
  }
}

const refRecordColumnsSQL = `
  from_symbol_id AS fromSymbolId,
  to_symbol_id AS toSymbolId,
  kind,
  file_id AS fileId,
  start_line AS startLine,
  start_col AS startCol,
  end_line AS endLine,
  end_col AS endCol,
  start_offset AS startOffset,
  end_offset AS endOffset
`;

const refRecordSelectSQL = `
  SELECT ${refRecordColumnsSQL}
  FROM snapshot_refs
`;

const refRecordSelectSQLWithAlias = `
  SELECT r.from_symbol_id AS fromSymbolId,
         r.to_symbol_id AS toSymbolId,
         r.kind,
         r.file_id AS fileId,
         r.start_line AS startLine,
         r.start_col AS startCol,
         r.end_line AS endLine,
         r.end_col AS endCol,
         r.start_offset AS startOffset,
         r.end_offset AS endOffset
  FROM snapshot_refs r
`;

function toSnippetRef(row: ReviewSnippetSQL): SnippetRef {
  return {
    stubId: row.stubId,
    directive: row.directive,
    symbolId: row.symbolId ?? undefined,
    filePath: row.filePath ?? undefined,
    kind: row.kind ?? undefined,
    language: row.language ?? undefined,
    text: row.text,
    params: parseJSON<Record<string, string>>(row.paramsJson, {}),
    startLine: row.startLine,
    endLine: row.endLine,
    commitHash: row.commitHash ?? undefined,
  };
}

function toRefRecord(row: RefRecordSQL): RefRecord {
  return {
    fromSymbolId: row.fromSymbolId,
    toSymbolId: row.toSymbolId,
    kind: row.kind,
    fileId: row.fileId,
    range: {
      startLine: row.startLine,
      startCol: row.startCol,
      endLine: row.endLine,
      endCol: row.endCol,
      startOffset: row.startOffset,
      endOffset: row.endOffset,
    },
  };
}

const symbolSelectSQL = `
  SELECT id,
         kind,
         name,
         package_id AS packageId,
         file_id AS fileId,
         start_line AS startLine,
         start_col AS startCol,
         end_line AS endLine,
         end_col AS endCol,
         start_offset AS startOffset,
         end_offset AS endOffset,
         doc,
         signature,
         receiver_type AS receiverType,
         receiver_pointer AS receiverPointer,
         exported,
         language,
         type_params_json AS typeParamsJson,
         tags_json AS tagsJson
  FROM snapshot_symbols
`;

function toSymbol(row: SymbolSQL): Symbol {
  return {
    id: row.id,
    kind: row.kind,
    name: row.name,
    packageId: row.packageId,
    fileId: row.fileId,
    range: {
      startLine: row.startLine,
      startCol: row.startCol,
      endLine: row.endLine,
      endCol: row.endCol,
      startOffset: row.startOffset,
      endOffset: row.endOffset,
    },
    doc: row.doc,
    signature: row.signature,
    receiver: row.receiverType ? { typeName: row.receiverType, pointer: row.receiverPointer !== 0 } : undefined,
    typeParams: parseJSON<string[]>(row.typeParamsJson, []),
    exported: row.exported !== 0,
    tags: parseJSON<string[]>(row.tagsJson, []),
    language: row.language,
  };
}

const fileDiffSQL = `
  SELECT b.id AS FileID,
         b.path AS Path,
         'added' AS ChangeType,
         '' AS OldSHA256,
         b.sha256 AS NewSHA256
  FROM snapshot_files b
  LEFT JOIN snapshot_files a
    ON a.commit_hash = ? AND a.id = b.id
  WHERE b.commit_hash = ? AND a.id IS NULL

  UNION ALL

  SELECT a.id AS FileID,
         a.path AS Path,
         'removed' AS ChangeType,
         a.sha256 AS OldSHA256,
         '' AS NewSHA256
  FROM snapshot_files a
  LEFT JOIN snapshot_files b
    ON b.commit_hash = ? AND b.id = a.id
  WHERE a.commit_hash = ? AND b.id IS NULL

  UNION ALL

  SELECT b.id AS FileID,
         b.path AS Path,
         'modified' AS ChangeType,
         a.sha256 AS OldSHA256,
         b.sha256 AS NewSHA256
  FROM snapshot_files a
  JOIN snapshot_files b
    ON b.id = a.id
  WHERE a.commit_hash = ?
    AND b.commit_hash = ?
    AND a.sha256 != b.sha256
  ORDER BY Path
`;

const symbolDiffSQL = `
  SELECT b.id AS SymbolID,
         b.name AS Name,
         b.kind AS Kind,
         b.package_id AS PackageID,
         'added' AS ChangeType,
         0 AS OldStartLine,
         0 AS OldEndLine,
         b.start_line AS NewStartLine,
         b.end_line AS NewEndLine,
         '' AS OldSignature,
         b.signature AS NewSignature,
         '' AS OldBodyHash,
         b.body_hash AS NewBodyHash
  FROM snapshot_symbols b
  LEFT JOIN snapshot_symbols a
    ON a.commit_hash = ? AND a.id = b.id
  WHERE b.commit_hash = ? AND a.id IS NULL

  UNION ALL

  SELECT a.id AS SymbolID,
         a.name AS Name,
         a.kind AS Kind,
         a.package_id AS PackageID,
         'removed' AS ChangeType,
         a.start_line AS OldStartLine,
         a.end_line AS OldEndLine,
         0 AS NewStartLine,
         0 AS NewEndLine,
         a.signature AS OldSignature,
         '' AS NewSignature,
         a.body_hash AS OldBodyHash,
         '' AS NewBodyHash
  FROM snapshot_symbols a
  LEFT JOIN snapshot_symbols b
    ON b.commit_hash = ? AND b.id = a.id
  WHERE a.commit_hash = ? AND b.id IS NULL

  UNION ALL

  SELECT b.id AS SymbolID,
         b.name AS Name,
         b.kind AS Kind,
         b.package_id AS PackageID,
         CASE
           WHEN a.body_hash != b.body_hash AND a.body_hash != '' AND b.body_hash != '' THEN 'modified'
           WHEN a.signature != b.signature THEN 'signature-changed'
           WHEN a.start_line != b.start_line OR a.end_line != b.end_line THEN 'moved'
           ELSE 'unchanged'
         END AS ChangeType,
         a.start_line AS OldStartLine,
         a.end_line AS OldEndLine,
         b.start_line AS NewStartLine,
         b.end_line AS NewEndLine,
         a.signature AS OldSignature,
         b.signature AS NewSignature,
         a.body_hash AS OldBodyHash,
         b.body_hash AS NewBodyHash
  FROM snapshot_symbols a
  JOIN snapshot_symbols b
    ON b.id = a.id
  WHERE a.commit_hash = ?
    AND b.commit_hash = ?
    AND (
      a.body_hash != b.body_hash
      OR a.signature != b.signature
      OR a.start_line != b.start_line
      OR a.end_line != b.end_line
    )
  ORDER BY Name
`;

function diffStats(files: FileDiff[], symbols: SymbolDiff[]): DiffStats {
  const stats: DiffStats = {
    FilesAdded: 0,
    FilesRemoved: 0,
    FilesModified: 0,
    SymbolsAdded: 0,
    SymbolsRemoved: 0,
    SymbolsModified: 0,
    SymbolsMoved: 0,
    SymbolsUnchanged: 0,
  };
  for (const file of files) {
    if (file.ChangeType === 'added') stats.FilesAdded++;
    else if (file.ChangeType === 'removed') stats.FilesRemoved++;
    else if (file.ChangeType === 'modified') stats.FilesModified++;
  }
  for (const symbol of symbols) {
    if (symbol.ChangeType === 'added') stats.SymbolsAdded++;
    else if (symbol.ChangeType === 'removed') stats.SymbolsRemoved++;
    else if (symbol.ChangeType === 'modified') stats.SymbolsModified++;
    else if (symbol.ChangeType === 'moved') stats.SymbolsMoved++;
    else if (symbol.ChangeType === 'unchanged') stats.SymbolsUnchanged++;
  }
  return stats;
}

function fallbackName(symbolId: string): string {
  const trimmed = symbolId.startsWith('sym:') ? symbolId.slice(4) : symbolId;
  const lastDot = trimmed.lastIndexOf('.');
  return lastDot >= 0 ? trimmed.slice(lastDot + 1) : trimmed;
}

function parseJSON<T>(raw: string, fallback: T): T {
  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

function simpleUnifiedDiff(oldText: string, newText: string): string {
  const oldLines = splitLines(oldText);
  const newLines = splitLines(newText);
  let prefix = 0;
  while (prefix < oldLines.length && prefix < newLines.length && oldLines[prefix] === newLines[prefix]) {
    prefix++;
  }

  let suffix = 0;
  while (
    suffix < oldLines.length - prefix &&
    suffix < newLines.length - prefix &&
    oldLines[oldLines.length - 1 - suffix] === newLines[newLines.length - 1 - suffix]
  ) {
    suffix++;
  }

  const out: string[] = [];
  for (let i = 0; i < prefix; i++) out.push(`  ${oldLines[i]}`);
  for (let i = prefix; i < oldLines.length - suffix; i++) out.push(`- ${oldLines[i]}`);
  for (let i = prefix; i < newLines.length - suffix; i++) out.push(`+ ${newLines[i]}`);
  for (let i = oldLines.length - suffix; i < oldLines.length; i++) out.push(`  ${oldLines[i]}`);
  return out.join('\n') + (out.length > 0 ? '\n' : '');
}

function splitLines(text: string): string[] {
  const lines = text.split('\n');
  if (lines.length > 0 && lines[lines.length - 1] === '') lines.pop();
  return lines;
}
