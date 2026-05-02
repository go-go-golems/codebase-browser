import { useGetDiffQuery } from '../../../api/historyApi';
import { HistoryUnavailableNotice, isHistoryUnavailable } from './historyUnavailable';
import { WidgetError } from './WidgetError';

interface DiffStatsWidgetProps {
  from: string;
  to: string;
}

export function DiffStatsWidget({ from, to }: DiffStatsWidgetProps) {
  const { data, isLoading, error } = useGetDiffQuery({ from, to }, { skip: !from || !to });
  if (isLoading) return <span data-part="loading">Loading diff stats…</span>;
  if (error) {
    if (isHistoryUnavailable(error)) {
      return <HistoryUnavailableNotice widget="Diff stats" />;
    }
    return <WidgetError title="Failed to load diff stats" error={error} />;
  }
  if (!data) return null;
  const s = data.Stats;
  const chips = [
    ['files', `+${s.FilesAdded} -${s.FilesRemoved} ~${s.FilesModified}`],
    ['symbols', `+${s.SymbolsAdded} -${s.SymbolsRemoved} ~${s.SymbolsModified}`],
    ['moved', `${s.SymbolsMoved}`],
  ];
  return (
    <span data-role="diff-stats" style={{ display: 'inline-flex', gap: 6, flexWrap: 'wrap', alignItems: 'center' }}>
      {chips.map(([label, value]) => (
        <span key={label} style={{ border: '1px solid var(--cb-color-border)', borderRadius: 999, padding: '2px 8px', fontSize: 12 }}>
          <strong>{label}</strong> <code>{value}</code>
        </span>
      ))}
      <span style={{ color: 'var(--cb-color-muted)', fontSize: 12 }}>
        <code>{from.slice(0, 7)}</code> → <code>{to.slice(0, 7)}</code>
      </span>
    </span>
  );
}
