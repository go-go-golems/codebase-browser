import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { apiProvider } from './codebaseProvider';
import { normalizeQueryError, type ProviderError } from './queryErrors';

export interface SnippetRef {
  stubId: string;
  directive: string;
  symbolId?: string;
  filePath?: string;
  kind?: string;
  language?: string;
  text: string;
  commitHash?: string;
  params?: Record<string, string>;
  startLine?: number;
  endLine?: number;
}

export interface ReviewDiagnostic {
  severity: string;
  line?: number;
  directive?: string;
  message: string;
}

export interface ReviewPageBlock {
  type: 'markdown' | 'widget';
  id?: string;
  html?: string;
  directive?: string;
  props?: Record<string, string>;
  body?: string;
  line?: number;
}

export interface DocPage {
  slug: string;
  title: string;
  html?: string;
  snippets: SnippetRef[];
  errors?: string[];
  blocks?: ReviewPageBlock[];
  diagnostics?: ReviewDiagnostic[];
}

export interface PageMeta {
  slug: string;
  title: string;
  path: string;
}

export interface ReviewDocMeta {
  slug: string;
  title: string;
}

const noopBaseQuery: BaseQueryFn<void, unknown, ProviderError> = async () => ({ data: undefined });

async function providerResult<T>(fn: () => Promise<T>): Promise<{ data: T } | { error: ProviderError }> {
  try {
    return { data: await fn() };
  } catch (err) {
    return { error: normalizeQueryError(err) };
  }
}

export const docApi = createApi({
  reducerPath: 'docApi',
  baseQuery: noopBaseQuery,
  keepUnusedDataFor: 3600,
  endpoints: (b) => ({
    listDocs: b.query<PageMeta[], void>({
      queryFn: async () => ({ data: [] }),
    }),
    getDoc: b.query<DocPage, string>({
      queryFn: (slug) => providerResult(() => apiProvider().getReviewDoc(slug)),
    }),
    listReviewDocs: b.query<ReviewDocMeta[], void>({
      queryFn: () => providerResult(() => apiProvider().listReviewDocs()),
    }),
    getReviewDoc: b.query<DocPage, string>({
      queryFn: (slug) => providerResult(() => apiProvider().getReviewDoc(slug)),
    }),
  }),
});

export const { useListDocsQuery, useGetDocQuery, useListReviewDocsQuery, useGetReviewDocQuery } = docApi;
