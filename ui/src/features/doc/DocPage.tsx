// Generic embedded docs are not exported by the static review browser anymore.
// Keep a tiny placeholder route so old /doc links fail clearly instead of
// carrying the deprecated data-codebase-snippet DOM hydration path forward.
export function DocPage() {
  return <div data-part="empty">Embedded docs are not available in this static export.</div>;
}

export function DocList() {
  return null;
}
