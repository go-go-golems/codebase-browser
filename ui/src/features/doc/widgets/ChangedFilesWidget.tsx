import { Link } from 'react-router-dom';
import { useGetDiffQuery } from '../../../api/historyApi';
import { WidgetError } from './WidgetError';

interface ChangedFilesWidgetProps {
  from: string;
  to: string;
}

export function ChangedFilesWidget({ from, to }: ChangedFilesWidgetProps) {
  const { data, isLoading, error } = useGetDiffQuery({ from, to }, { skip: !from || !to });
  if (isLoading) {
    return <pre data-part="code-block"><code>Loading changed files…</code></pre>;
  }
  if (error) {
    return <WidgetError title="Failed to load changed files" error={error} />;
  }
  const files = data?.Files ?? [];
  return (
    <section data-part="doc-snippet" data-role="changed-files">
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, marginBottom: 8 }}>
        <strong>Changed files</strong>
        <span style={{ fontSize: 12, color: 'var(--cb-color-muted)' }}>
          {files.length} file(s) · <code>{from.slice(0, 7)}</code> → <code>{to.slice(0, 7)}</code>
        </span>
      </div>
      {files.length === 0 ? (
        <div data-part="empty">No changed files.</div>
      ) : (
        <div style={{ border: '1px solid var(--cb-color-border)', borderRadius: 8, overflow: 'hidden' }}>
          {files.map((file, i) => (
            <div
              key={`${file.Path}-${i}`}
              style={{
                display: 'grid',
                gridTemplateColumns: '6rem minmax(0, 1fr)',
                gap: 8,
                borderTop: i === 0 ? 0 : '1px solid var(--cb-color-border)',
                padding: '6px 8px',
                alignItems: 'baseline',
              }}
            >
              <code style={{ fontSize: 12 }}>{file.ChangeType}</code>
              <Link to={`/source/${file.Path}`} style={{ textDecoration: 'none', minWidth: 0 }}>
                <code>{file.Path}</code>
              </Link>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
