import { QueryError, type QueryErrorCode } from '../queryErrors';

export interface SqlJsWorkerRequest {
  id: number;
  method: string;
  args: unknown[];
  baseUrl?: string;
  debugSql?: boolean;
}

export interface SqlJsWorkerTiming {
  method: string;
  elapsedMs: number;
}

export interface SerializedProviderError {
  name?: string;
  message: string;
  code?: QueryErrorCode;
  details?: Record<string, unknown>;
  stack?: string;
}

export type SqlJsWorkerResponse =
  | { id: number; ok: true; result: unknown; timing: SqlJsWorkerTiming }
  | { id: number; ok: false; error: SerializedProviderError };

export function serializeProviderError(error: unknown): SerializedProviderError {
  if (error instanceof QueryError) {
    return {
      name: error.name,
      message: error.message,
      code: error.code,
      details: error.details,
      stack: error.stack,
    };
  }
  if (error instanceof Error) {
    return {
      name: error.name,
      message: error.message,
      stack: error.stack,
    };
  }
  return { message: String(error) };
}

export function deserializeProviderError(error: SerializedProviderError): Error {
  if (error.code) {
    const queryError = new QueryError(error.code, error.message, error.details ?? {});
    queryError.stack = error.stack;
    return queryError;
  }
  const generic = new Error(error.message);
  generic.name = error.name ?? 'Error';
  generic.stack = error.stack;
  return generic;
}
