// React namespace provided by jsx: react-jsx
import { Link, useParams } from 'react-router-dom';
import {
  useGetReviewDocQuery,
  useListReviewDocsQuery,
  type DocPage,
  type ReviewPageBlock,
  type SnippetRef,
} from '../../api/docApi';
import { DocSnippet } from '../doc/DocSnippet';

export function ReviewDocPage() {
  const { slug: rawSlug } = useParams<{ slug: string }>();
  const slug = rawSlug ?? '';
  const { data, isLoading, error } = useGetReviewDocQuery(slug, { skip: !slug });

  if (isLoading) return <div data-part="loading">Loading review doc…</div>;
  if (error) return <div data-part="error">Failed to load review doc: {JSON.stringify(error)}</div>;
  if (!data) return <div data-part="empty">No review doc data for slug: {slug}</div>;

  return (
    <article data-part="doc-page">
      {data.diagnostics && data.diagnostics.length > 0 && (
        <div data-part="error">
          {data.diagnostics.map((diagnostic, i) => (
            <div key={i}>
              {diagnostic.line ? `line ${diagnostic.line}: ` : ''}
              {diagnostic.message}
            </div>
          ))}
        </div>
      )}
      <ReviewBlocks page={data} />
      <footer data-part="symbol-doc" style={{ marginTop: 32, fontSize: 12 }}>
        Resolved {(data.snippets ?? []).length} snippet(s) from the review index.
      </footer>
    </article>
  );
}

function ReviewBlocks({ page }: { page: DocPage }) {
  let widgetIndex = 0;
  return (
    <>
      {(page.blocks ?? []).map((block, index) => {
        if (block.type === 'widget') {
          const snippet = page.snippets?.[widgetIndex];
          widgetIndex += 1;
          return <ReviewWidgetBlock key={block.id ?? index} block={block} snippet={snippet} />;
        }
        return <MarkdownBlock key={block.id ?? index} block={block} />;
      })}
    </>
  );
}

function MarkdownBlock({ block }: { block: ReviewPageBlock }) {
  return <div dangerouslySetInnerHTML={{ __html: block.html ?? '' }} />;
}

function ReviewWidgetBlock({ block, snippet }: { block: ReviewPageBlock; snippet?: SnippetRef }) {
  const props = block.props ?? {};
  const directive = block.directive ?? snippet?.directive ?? '';
  const sym = snippet?.symbolId ?? props.sym ?? '';
  const params = { ...props, ...(snippet?.params ?? {}) };
  return (
    <div className="codebase-snippet" data-directive={directive} data-widget-id={block.id}>
      <DocSnippet
        sym={sym}
        directive={directive}
        kind={snippet?.kind ?? props.kind ?? ''}
        lang={snippet?.language ?? 'go'}
        commit={snippet?.commitHash ?? props.commit}
        params={params}
        text={snippet?.text}
      />
    </div>
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
