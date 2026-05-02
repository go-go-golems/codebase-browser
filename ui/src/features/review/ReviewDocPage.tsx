// React namespace provided by jsx: react-jsx
import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Link, useParams } from 'react-router-dom';
import { useGetReviewDocQuery, useListReviewDocsQuery, type SnippetRef } from '../../api/docApi';
import { DocSnippet } from '../doc/DocSnippet';

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

export function ReviewDocPage() {
  const { slug: rawSlug } = useParams<{ slug: string }>();
  const slug = rawSlug ?? '';
  const { data, isLoading, error } = useGetReviewDocQuery(slug, { skip: !slug });
  const articleRef = useRef<HTMLElement>(null);
  const [stubs, setStubs] = useState<StubHandle[]>([]);

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
        if (!directive) return;
        el.innerHTML = '';
        found.push({ el, sym, directive, kind, lang, commit, params: params ?? snippet?.params, text: snippet?.text });
      });
    setStubs(found);
  }, [data?.html]);

  if (isLoading) return <div data-part="loading">Loading review doc…</div>;
  if (error) return <div data-part="error">Failed to load review doc: {JSON.stringify(error)}</div>;
  if (!data) return <div data-part="empty">No review doc data for slug: {slug}</div>;

  return (
    <article data-part="doc-page" ref={articleRef}>
      {data.errors && data.errors.length > 0 && (
        <div data-part="error">
          {data.errors.map((e: string, i: number) => (
            <div key={i}>{e}</div>
          ))}
        </div>
      )}
      <div dangerouslySetInnerHTML={{ __html: data.html }} />
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
        Resolved {(data.snippets ?? []).length} snippet(s) from the review index.
      </footer>
    </article>
  );
}

export function ReviewDocList() {
  const { data } = useListReviewDocsQuery();
  if (!data?.length) return null;
  return (
    <details open style={{ marginBottom: 12 }}>
      <summary style={{ cursor: 'pointer', fontWeight: 600, padding: '4px 0' }}>Review docs</summary>
      <ul data-part="tree-nav">
        {data.map((d) => (
          <li key={d.slug}>
            <Link data-part="tree-node" to={`/review/${encodeURIComponent(d.slug)}`}>
              {d.title}
            </Link>
          </li>
        ))}
      </ul>
    </details>
  );
}
