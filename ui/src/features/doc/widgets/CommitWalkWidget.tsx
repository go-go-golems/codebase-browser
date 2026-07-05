import React from 'react';
import { AnnotationWidget } from './AnnotationWidget';
import { ChangedFilesWidget } from './ChangedFilesWidget';
import { DiffStatsWidget } from './DiffStatsWidget';
import { ImpactInlineWidget } from './ImpactInlineWidget';
import { SymbolDiffInlineWidget } from './SymbolDiffInlineWidget';
import { SymbolHistoryInlineWidget } from './SymbolHistoryInlineWidget';

export interface CommitWalkStep {
  kind: string;
  title?: string;
  body?: string;
  symbolId?: string;
  language?: string;
  params?: Record<string, string>;
}

interface CommitWalkWidgetProps {
  title?: string;
  stepsJSON?: string;
  commit?: string;
  from?: string;
  to?: string;
}

export function CommitWalkWidget({ title = 'Commit walk', stepsJSON, commit, from, to }: CommitWalkWidgetProps) {
  const steps = React.useMemo(() => parseSteps(stepsJSON), [stepsJSON]);
  const [index, setIndex] = React.useState(0);
  const current = steps[index];

  React.useEffect(() => {
    if (index >= steps.length) setIndex(Math.max(steps.length - 1, 0));
  }, [index, steps.length]);

  if (steps.length === 0) {
    return <div data-part="error">Commit walk has no valid steps.</div>;
  }

  return (
    <section
      data-part="doc-snippet"
      data-role="commit-walk"
      style={{ border: '1px solid var(--cb-color-border)', borderRadius: 12, padding: 16 }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'baseline', flexWrap: 'wrap', marginBottom: 12 }}>
        <div>
          <h3 style={{ margin: 0 }}>{title}</h3>
          <div style={{ fontSize: 12, color: 'var(--cb-color-muted)', marginTop: 4 }}>
            Step {index + 1} of {steps.length}: <code>{current.kind}</code>
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <button type="button" onClick={() => setIndex((i) => Math.max(i - 1, 0))} disabled={index === 0} style={navButtonStyle}>
            ← Prev
          </button>
          <button type="button" onClick={() => setIndex((i) => Math.min(i + 1, steps.length - 1))} disabled={index === steps.length - 1} style={navButtonStyle}>
            Next →
          </button>
        </div>
      </div>

      <ol style={{ display: 'flex', gap: 6, listStyle: 'none', padding: 0, margin: '0 0 16px', flexWrap: 'wrap' }} aria-label="Commit walk steps">
        {steps.map((step, i) => (
          <li key={i}>
            <button
              type="button"
              onClick={() => setIndex(i)}
              aria-current={i === index ? 'step' : undefined}
              title={step.title || step.kind}
              style={{
                border: i === index ? '2px solid var(--cb-color-accent, #2196f3)' : '1px solid var(--cb-color-border)',
                borderRadius: 999,
                background: i === index ? 'rgba(33, 150, 243, 0.12)' : 'transparent',
                color: 'var(--cb-color-text)',
                padding: '3px 8px',
                cursor: 'pointer',
                fontSize: 12,
              }}
            >
              {i + 1}
            </button>
          </li>
        ))}
      </ol>

      <article style={{ display: 'grid', gap: 12 }}>
        <header>
          <h4 style={{ margin: '0 0 4px' }}>{current.title || defaultStepTitle(current)}</h4>
        </header>
        {current.body && <StepNote body={current.body} />}
        <CommitWalkStepView step={current} defaultCommit={commit} defaultFrom={from} defaultTo={to} />
      </article>
    </section>
  );
}

function StepNote({ body }: { body: string }) {
  return (
    <aside
      data-part="commit-walk-note"
      style={{
        border: '1px solid rgba(33, 150, 243, 0.35)',
        borderLeft: '4px solid var(--cb-color-accent, #2196f3)',
        borderRadius: 8,
        padding: '10px 12px',
        background: 'rgba(33, 150, 243, 0.08)',
      }}
    >
      <div style={{ fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: 0.04, color: 'var(--cb-color-muted)', marginBottom: 4 }}>
        Guide note
      </div>
      <div>{body}</div>
    </aside>
  );
}

function CommitWalkStepView({
  step,
  defaultCommit,
  defaultFrom,
  defaultTo,
}: {
  step: CommitWalkStep;
  defaultCommit?: string;
  defaultFrom?: string;
  defaultTo?: string;
}) {
  const p = { ...(step.params ?? {}) };
  if (defaultCommit && !p.commit) p.commit = defaultCommit;
  if (defaultFrom && !p.from) p.from = defaultFrom;
  if (defaultTo && !p.to) p.to = defaultTo;

  switch (step.kind) {
    case 'overview':
    case 'note':
      return step.body ? null : <div style={{ color: 'var(--cb-color-muted)' }}>Continue to the next step.</div>;
    case 'diff-stats':
    case 'stats':
      return <DiffStatsWidget from={p.from ?? ''} to={p.to ?? ''} />;
    case 'changed-files':
    case 'files':
      return <ChangedFilesWidget from={p.from ?? ''} to={p.to ?? ''} />;
    case 'symbol':
    case 'annotation':
    case 'snippet':
      return (
        <AnnotationWidget
          sym={step.symbolId ?? ''}
          language={step.language}
          commit={p.commit ?? p.to}
          lines={p.lines}
          note={p.note}
        />
      );
    case 'diff':
      return <SymbolDiffInlineWidget sym={step.symbolId ?? ''} from={p.from ?? ''} to={p.to ?? ''} />;
    case 'history': {
      const limit = p.limit ? Number.parseInt(p.limit, 10) : undefined;
      return <SymbolHistoryInlineWidget sym={step.symbolId ?? ''} limit={Number.isFinite(limit) ? limit : undefined} />;
    }
    case 'impact': {
      const depth = p.depth ? Number.parseInt(p.depth, 10) : undefined;
      const dir = p.dir === 'uses' ? 'uses' : 'usedby';
      return <ImpactInlineWidget sym={step.symbolId ?? ''} dir={dir} depth={Number.isFinite(depth) ? depth : undefined} commit={p.commit} />;
    }
    default:
      return <div data-part="error">Unknown commit walk step kind: <code>{step.kind}</code></div>;
  }
}

function parseSteps(raw?: string): CommitWalkStep[] {
  if (!raw) return [];
  try {
    const value = JSON.parse(raw);
    return Array.isArray(value) ? value : [];
  } catch {
    return [];
  }
}

function defaultStepTitle(step: CommitWalkStep): string {
  if (step.kind === 'overview') return 'Review scope';
  if (step.kind === 'note') return 'Note';
  if (step.kind === 'stats' || step.kind === 'diff-stats') return 'Review the size of the change';
  if (step.kind === 'files' || step.kind === 'changed-files') return 'Review changed files';
  if (step.kind === 'symbol') return 'Inspect symbol';
  if (step.kind === 'diff') return 'Review the symbol diff';
  if (step.kind === 'history') return 'Review symbol history';
  if (step.kind === 'impact') return 'Review impact';
  if (step.kind === 'annotation' || step.kind === 'snippet') return 'Review annotated code';
  return step.kind;
}

const navButtonStyle: React.CSSProperties = {
  border: '1px solid var(--cb-color-border)',
  borderRadius: 6,
  background: 'transparent',
  color: 'var(--cb-color-text)',
  padding: '4px 8px',
  cursor: 'pointer',
};
