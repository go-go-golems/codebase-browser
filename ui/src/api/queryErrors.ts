export type QueryErrorCode =
  | 'NOT_FOUND'
  | 'AMBIGUOUS_REF'
  | 'SQL_ERROR'
  | 'DB_LOAD_ERROR'
  | 'FEATURE_UNAVAILABLE';

export class QueryError extends Error {
  constructor(
    public code: QueryErrorCode,
    message: string,
    public details: Record<string, unknown> = {},
  ) {
    super(message);
    this.name = 'QueryError';
  }
}

export type QueryErrorPayload = {
  message: string;
  details?: Record<string, unknown>;
};

export type ProviderError = { status: string; data?: QueryErrorPayload };

export function normalizeQueryError(err: unknown): { status: string; data?: QueryErrorPayload } {
  if (err instanceof QueryError) {
    return { status: err.code, data: { message: err.message, details: err.details } };
  }
  if (err instanceof Error) {
    return { status: 'SQL_ERROR', data: { message: err.message } };
  }
  return { status: 'SQL_ERROR', data: { message: String(err) } };
}
