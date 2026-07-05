import { useGetSnippetQuery } from '../../../api/sourceApi';

interface AnnotationWidgetProps {
  sym: string;
  language?: string;
  commit?: string;
  lines?: string;
  note?: string;
}

export function AnnotationWidget({ sym, commit, lines, note }: AnnotationWidgetProps) {
  const { data: text, isLoading, error } = useGetSnippetQuery({ sym, kind: 'declaration', commit: commit ?? 'HEAD' }, { skip: !sym });

  if (!sym) return <div data-part="error">Missing annotation symbol.</div>;
  if (error) return <div data-part="error">Failed to load annotation snippet: {JSON.stringify(error)}</div>;
  if (isLoading || text === undefined) return <pre data-part="code-block"><code>Loading annotation…</code></pre>;

  const highlight = parseLineSpec(lines);
  const sourceLines = text.split('\n');
  const lineLabel = lines ? `lines ${lines}` : 'selected lines';
  const cleanNote = normalizeNote(note);

  return (
    <section
      data-part="doc-snippet"
      data-role="annotation"
      style={{ border: '1px solid var(--cb-color-border)', borderRadius: 12, overflow: 'hidden' }}
    >
      <div style={{ padding: 12, borderBottom: '1px solid var(--cb-color-border)', background: 'rgba(255, 193, 7, 0.10)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', alignItems: 'baseline' }}>
          <strong>Review note: <code>{displayName(sym)}</code></strong>
          <span style={{ fontSize: 12, color: 'var(--cb-color-muted)' }}>
            {commit && <>at <code>{commit.slice(0, 7)}</code> · </>}
            highlighting <code>{lineLabel}</code>
          </span>
        </div>
        <p style={{ margin: '8px 0 0', color: 'var(--cb-color-text)' }}>
          {cleanNote || 'Read the highlighted lines below; they are the part of this snippet the guide is calling out.'}
        </p>
      </div>

      <pre
        data-part="code-block"
        data-role="annotation-code"
        style={{
          margin: 0,
          padding: 0,
          maxHeight: '60vh',
          overflow: 'auto',
          background: 'var(--cb-color-surface, #f8f8f8)',
          border: 0,
          borderRadius: 0,
          lineHeight: 1.55,
          fontSize: 13,
        }}
      >
        <code>
          {sourceLines.map((line, index) => {
            const lineNo = index + 1;
            const active = highlight.has(lineNo);
            return (
              <span
                key={lineNo}
                data-highlighted={active ? 'true' : undefined}
                style={{
                  display: 'grid',
                  gridTemplateColumns: '3.5rem minmax(0, 1fr)',
                  background: active ? 'rgba(255, 193, 7, 0.22)' : 'transparent',
                  borderLeft: active ? '4px solid #ffb300' : '4px solid transparent',
                }}
              >
                <span style={{ color: active ? '#8a5a00' : 'var(--cb-color-muted)', textAlign: 'right', padding: '0 10px 0 6px', userSelect: 'none' }}>
                  {lineNo}
                </span>
                <span style={{ whiteSpace: 'pre-wrap', paddingRight: 12 }}>{line || ' '}</span>
              </span>
            );
          })}
        </code>
      </pre>
    </section>
  );
}

function normalizeNote(note?: string): string {
  if (!note) return '';
  return note.split('_').join(' ');
}

function parseLineSpec(spec?: string): Set<number> {
  const result = new Set<number>();
  if (!spec) return result;
  for (const part of spec.split(',')) {
    const [startRaw, endRaw] = part.split('-');
    const start = Number.parseInt(startRaw, 10);
    const end = endRaw ? Number.parseInt(endRaw, 10) : start;
    if (!Number.isFinite(start) || !Number.isFinite(end)) continue;
    for (let n = Math.min(start, end); n <= Math.max(start, end); n++) result.add(n);
  }
  return result;
}

function displayName(symbolId: string): string {
  const trimmed = symbolId.startsWith('sym:') ? symbolId.slice(4) : symbolId;
  const dot = trimmed.lastIndexOf('.');
  return dot >= 0 ? trimmed.slice(dot + 1) : trimmed;
}
