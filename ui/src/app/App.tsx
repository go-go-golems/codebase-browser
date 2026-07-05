import React from 'react';
import { HashRouter, Link, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { widgetRootAttrs } from '../packages/ui/src/parts';
import '../packages/ui/src/theme/base.css';
import '../packages/ui/src/theme/dark.css';
import { HomePage } from '../features/tree/HomePage';
import { PackagePage } from '../features/tree/PackagePage';
import { SymbolPage } from '../features/symbol/SymbolPage';
import { SourcePage } from '../features/source/SourcePage';
import { SearchPanel } from '../features/tree/SearchPanel';
import { DocPage } from '../features/doc/DocPage';
import { ReviewDocPage, ReviewDocList } from '../features/review/ReviewDocPage';
import { HistoryPage } from '../features/history/HistoryPage';
import { useGetIndexQuery } from '../api/indexApi';
import type { Package } from '../api/types';

export function App() {
  const [dark, setDark] = React.useState(false);
  return (
    <HashRouter>
      <ScrollToTop />
      <div {...widgetRootAttrs} data-theme={dark ? 'dark' : 'light'}>
        <div data-part="layout">
          <aside data-part="sidebar">
            <Header onToggleTheme={() => setDark((d) => !d)} dark={dark} />
            <SearchPanel />
            <div style={{ marginBottom: 12 }}>
              <Link to="/history" style={{ fontWeight: 600, color: 'var(--cb-color-accent)', textDecoration: 'none' }}>
                History
              </Link>
            </div>
            <ReviewDocList />
            <PackageTree />
          </aside>
          <main data-part="main">
            <Routes>
              <Route path="/" element={<HomePage />} />
              <Route path="/packages/:id" element={<PackagePage />} />
              <Route path="/symbol/:id" element={<SymbolPage />} />
              <Route path="/source/*" element={<SourcePage />} />
              <Route path="/doc/:slug" element={<DocPage />} />
              <Route path="/review/:slug" element={<ReviewDocPage />} />
              <Route path="/history" element={<HistoryPage />} />
            </Routes>
          </main>
        </div>
      </div>
    </HashRouter>
  );
}

function ScrollToTop() {
  const { pathname, search, hash } = useLocation();
  React.useEffect(() => {
    window.scrollTo({ top: 0, left: 0, behavior: 'auto' });
    document.querySelector<HTMLElement>('[data-part="main"]')?.scrollTo({ top: 0, left: 0, behavior: 'auto' });
  }, [pathname, search, hash]);
  return null;
}

function Header({ onToggleTheme, dark }: { onToggleTheme: () => void; dark: boolean }) {
  const { data } = useGetIndexQuery();
  return (
    <div style={{ marginBottom: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Link to="/" style={{ fontWeight: 700, color: 'var(--cb-color-accent)', textDecoration: 'none' }}>
          Codebase Browser
        </Link>
        <button
          onClick={onToggleTheme}
          style={{ background: 'transparent', border: '1px solid var(--cb-color-border)', padding: '2px 8px', borderRadius: 4, cursor: 'pointer', color: 'var(--cb-color-text)' }}
        >
          {dark ? '☀' : '☾'}
        </button>
      </div>
      {data && (
        <div data-part="symbol-doc" style={{ fontSize: 12, marginTop: 6 }}>
          {data.module} · {data.packages.length} pkg · {data.symbols.length} sym
        </div>
      )}
    </div>
  );
}

interface PackageTreeNode {
  id: string;
  label: string;
  children: Map<string, PackageTreeNode>;
  pkg?: Package;
}

function PackageTree() {
  const { data, isLoading, error } = useGetIndexQuery();
  const navigate = useNavigate();
  const location = useLocation();
  if (isLoading) return <div data-part="loading">Loading…</div>;
  if (error) return <div data-part="error">Failed to load index</div>;
  if (!data) return null;

  const activePackageId = decodeActivePackageId(location.pathname);
  const { rootLabel, children } = buildPackageTree(data.packages);

  return (
    <details open style={{ marginBottom: 12 }}>
      <summary style={{ cursor: 'pointer', fontWeight: 600, padding: '4px 0' }}>
        Packages <span data-role="hint">({data.packages.length})</span>
      </summary>
      <div data-part="tree-root-label" title={rootLabel}>{rootLabel}</div>
      <ul data-part="tree-nav" data-role="package-tree">
        {children.map((node) => (
          <PackageTreeNodeView
            key={node.id}
            node={node}
            navigate={navigate}
            activePackageId={activePackageId}
            depth={0}
          />
        ))}
      </ul>
    </details>
  );
}

function PackageTreeNodeView({
  node,
  navigate,
  activePackageId,
  depth,
}: {
  node: PackageTreeNode;
  navigate: ReturnType<typeof useNavigate>;
  activePackageId?: string;
  depth: number;
}) {
  const sortedChildren = sortNodes([...node.children.values()]);
  const hasChildren = sortedChildren.length > 0;
  const isActive = node.pkg?.id === activePackageId;
  const startsActivePath = !!activePackageId && packageTreeContains(node, activePackageId);

  if (hasChildren) {
    return (
      <li data-role="tree-branch" style={{ marginLeft: depth ? 10 : 0 }}>
        <details open={startsActivePath}>
          <summary data-part="tree-node" data-state={startsActivePath ? 'active-path' : undefined}>
            {node.label}
            {node.pkg && <span data-role="hint"> · pkg</span>}
          </summary>
          {node.pkg && (
            <a
              data-part="tree-node"
              data-state={isActive ? 'active' : undefined}
              href={`/packages/${encodeURIComponent(node.pkg.id)}`}
              onClick={(e) => {
                e.preventDefault();
                navigate(`/packages/${encodeURIComponent(node.pkg!.id)}`);
              }}
            >
              Open package
            </a>
          )}
          <ul data-part="tree-nav">
            {sortedChildren.map((child) => (
              <PackageTreeNodeView
                key={child.id}
                node={child}
                navigate={navigate}
                activePackageId={activePackageId}
                depth={depth + 1}
              />
            ))}
          </ul>
        </details>
      </li>
    );
  }

  if (!node.pkg) return null;
  return (
    <li style={{ marginLeft: depth ? 10 : 0 }}>
      <a
        data-part="tree-node"
        data-state={isActive ? 'active' : undefined}
        href={`/packages/${encodeURIComponent(node.pkg.id)}`}
        title={node.pkg.importPath}
        onClick={(e) => {
          e.preventDefault();
          navigate(`/packages/${encodeURIComponent(node.pkg!.id)}`);
        }}
      >
        {node.label}
        <span data-role="hint"> ({node.pkg.fileIds.length} files · {node.pkg.symbolIds.length} sym)</span>
      </a>
    </li>
  );
}

function buildPackageTree(packages: Package[]): { rootLabel: string; children: PackageTreeNode[] } {
  const sortedPackages = [...packages].sort((a, b) => a.importPath.localeCompare(b.importPath));
  const prefix = commonPathPrefix(sortedPackages.map((p) => p.importPath.split('/').filter(Boolean)));
  const rootLabel = prefix.length ? prefix.join('/') : 'codebase';
  const root = newNode('root', rootLabel);

  for (const pkg of sortedPackages) {
    const segments = pkg.importPath.split('/').filter(Boolean).slice(prefix.length);
    const pathSegments = segments.length ? segments : [pkg.name || pkg.importPath.split('/').pop() || pkg.importPath];
    let current = root;
    pathSegments.forEach((segment, index) => {
      const id = `${current.id}/${segment}`;
      let child = current.children.get(segment);
      if (!child) {
        child = newNode(id, segment);
        current.children.set(segment, child);
      }
      if (index === pathSegments.length - 1) child.pkg = pkg;
      current = child;
    });
  }

  return { rootLabel, children: sortNodes([...root.children.values()]) };
}

function newNode(id: string, label: string): PackageTreeNode {
  return { id, label, children: new Map() };
}

function commonPathPrefix(paths: string[][]): string[] {
  if (!paths.length) return [];
  const first = paths[0];
  const out: string[] = [];
  for (let i = 0; i < first.length; i += 1) {
    const segment = first[i];
    if (!segment || paths.some((path) => path[i] !== segment)) break;
    out.push(segment);
  }
  return out;
}

function sortNodes(nodes: PackageTreeNode[]): PackageTreeNode[] {
  return nodes.sort((a, b) => {
    const aBranch = a.children.size > 0 ? 0 : 1;
    const bBranch = b.children.size > 0 ? 0 : 1;
    if (aBranch !== bBranch) return aBranch - bBranch;
    return a.label.localeCompare(b.label);
  });
}

function packageTreeContains(node: PackageTreeNode, packageId: string): boolean {
  if (node.pkg?.id === packageId) return true;
  for (const child of node.children.values()) {
    if (packageTreeContains(child, packageId)) return true;
  }
  return false;
}

function decodeActivePackageId(pathname: string): string | undefined {
  if (!pathname.startsWith('/packages/')) return undefined;
  try {
    return decodeURIComponent(pathname.slice('/packages/'.length));
  } catch {
    return pathname.slice('/packages/'.length);
  }
}
