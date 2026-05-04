type NormalizedError = {
  status?: unknown;
  data?: unknown;
};

function errorText(error: unknown): string {
  if (!error || typeof error !== 'object') return String(error ?? 'unknown error');
  const normalized = error as NormalizedError;
  const data = normalized.data;
  if (typeof data === 'string') return data;
  if (data && typeof data === 'object') {
    const payload = data as { message?: unknown; error?: unknown };
    if (typeof payload.message === 'string') return payload.message;
    if (typeof payload.error === 'string') return payload.error;
  }
  try {
    return JSON.stringify(error);
  } catch {
    return String(error);
  }
}

function errorDetails(error: unknown): Record<string, unknown> | undefined {
  if (!error || typeof error !== 'object') return undefined;
  const data = (error as NormalizedError).data;
  if (!data || typeof data !== 'object') return undefined;
  const details = (data as { details?: unknown }).details;
  if (!details || typeof details !== 'object') return undefined;
  return details as Record<string, unknown>;
}

export function WidgetError({ title, error }: { title: string; error: unknown }) {
  const details = errorDetails(error);
  return (
    <section data-part="error" data-role="widget-error" style={{ border: '1px solid var(--cb-color-border)', borderRadius: 8, padding: 10 }}>
      <strong>{title}</strong>
      <p style={{ margin: '6px 0', color: 'var(--cb-color-muted)' }}>{errorText(error)}</p>
      {details ? (
        <details>
          <summary>Diagnostic details</summary>
          <pre style={{ whiteSpace: 'pre-wrap', margin: '8px 0 0' }}>{JSON.stringify(details, null, 2)}</pre>
        </details>
      ) : null}
    </section>
  );
}
