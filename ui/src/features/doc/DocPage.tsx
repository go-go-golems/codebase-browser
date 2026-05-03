// React namespace provided by jsx: react-jsx
import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Link, useParams } from 'react-router-dom';
import { useGetDocQuery, type SnippetRef } from '../../api/docApi';
import { DocSnippet } from './DocSnippet';

interface StubHandle {
  el: HTMLElement;
  sym: string;
  directive: string;
  kind: string;
  lang: string;
  commit?: string;
  params?: Record<string, string>;
  text?: string;
}

export function DocPage() {
  const { slug: rawSlug } = useParams<{ slug: string }>();
  const slug = rawSlug ?? '';
  const { data, isLoading, error } = useGetDocQuery(slug, { skip: !slug });
  const articleRef = useRef<HTMLElement>(null);
  const [stubs, setStubs] = useState<StubHandle[]>([]);

  // After the server-rendered HTML is mounted (via dangerouslySetInnerHTML),
  // walk the article for stub divs, clear their plaintext fallback, and
  // stash them in state so we can portal rich widgets into each one. Each
  // portal keeps its React subtree under the outer <Provider> so the RTK-
  // Query hooks inside <DocSnippet> just work.
  useEffect(() => {
    if (!data || !articleRef.current) {
      setStubs([]);
      return;
    }
    const snippetsByStub = new Map<string, SnippetRef>((data.snippets ?? []).map((snippet) => [snippet.stubId, snippet]));
    const found: StubHandle[] = [];
    articleRef.current
      .querySelectorAll<HTMLElement>('[data-codebase-snippet]')
      .forEach((el) => {
        const stubId = el.getAttribute('data-stub-id') ?? '';
        const snippet = snippetsByStub.get(stubId);
        const sym = el.getAttribute('data-sym') ?? snippet?.symbolId ?? '';
        const directive = el.getAttribute('data-directive') ?? '';
        const kind = el.getAttribute('data-kind') ?? '';
        const lang = el.getAttribute('data-lang') ?? 'go';
        const commit = el.getAttribute('data-commit') ?? undefined;
        const rawParams = el.getAttribute('data-params') ?? '';
        let params: Record<string, string> | undefined;
        if (rawParams) {
          try {
            params = JSON.parse(rawParams) as Record<string, string>;
          } catch {
            params = undefined;
          }
        }
        // Some document widgets (diff-stats, changed-files) are not tied to a
        // single symbol, so only directive is required here.
        if (!directive) return;
        el.innerHTML = ''; // drop the plaintext fallback before React mounts
        found.push({ el, sym, directive, kind, lang, commit, params: params ?? snippet?.params, text: snippet?.text });
      });
    setStubs(found);
  }, [data?.html]);

  if (isLoading) return <div data-part="loading">Loading…</div>;
  if (error) return <div data-part="error">Failed to load doc</div>;
  if (!data) return null;

  return (
    <article data-part="doc-page" ref={articleRef}>
      {data.errors && data.errors.length > 0 && (
        <div data-part="error">
          {data.errors.map((e, i) => (
            <div key={i}>{e}</div>
          ))}
        </div>
      )}
      <div dangerouslySetInnerHTML={{ __html: data.html ?? '' }} />
      {stubs.map((s, i) =>
        createPortal(
          <DocSnippet
            sym={s.sym}
            directive={s.directive}
            kind={s.kind}
            lang={s.lang}
            commit={s.commit}
            params={s.params}
            text={s.text}
          />,
          s.el,
          `${slug}-${i}`,
        ),
      )}
      <footer data-part="symbol-doc" style={{ marginTop: 32, fontSize: 12 }}>
        Resolved {(data.snippets ?? []).length} snippet(s) from the live index.
      </footer>
    </article>
  );
}

export function DocList() {
  const { data } = useListDocs();
  if (!data?.length) return <div data-part="empty">No docs yet</div>;
  return (
    <ul data-part="tree-nav">
      {data.map((d) => (
        <li key={d.slug}>
          <Link data-part="tree-node" to={`/doc/${encodeURIComponent(d.slug)}`}>
            {d.title}
          </Link>
        </li>
      ))}
    </ul>
  );
}

// Narrow re-export to avoid an unused-import warning in this file.
import { useListDocsQuery } from '../../api/docApi';
const useListDocs = useListDocsQuery;
