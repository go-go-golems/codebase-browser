import { createApi, type BaseQueryFn } from '@reduxjs/toolkit/query/react';
import { normalizeQueryError } from './queryErrors';
import { liveOrSql, liveProvider, sqlProvider } from './codebaseProvider';

export interface SnippetRef {
  stubId: string;
  directive: string;
  symbolId?: string;
  filePath?: string;
  kind?: string;
  language?: string;
  text: string;
  startLine?: number;
  endLine?: number;
}

export interface DocPage {
  slug: string;
  title: string;
  html: string;
  snippets: SnippetRef[];
  errors?: string[];
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

type ProviderError = { status: string; data?: string };

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
      queryFn: (slug) => providerResult(() => liveOrSql(() => liveProvider().getReviewDoc(slug), () => sqlProvider().getReviewDoc(slug))),
    }),
    listReviewDocs: b.query<ReviewDocMeta[], void>({
      queryFn: () => providerResult(() => liveOrSql(() => liveProvider().listReviewDocs(), () => sqlProvider().listReviewDocs())),
    }),
    getReviewDoc: b.query<DocPage, string>({
      queryFn: (slug) => providerResult(() => liveOrSql(() => liveProvider().getReviewDoc(slug), () => sqlProvider().getReviewDoc(slug))),
    }),
  }),
});

export const { useListDocsQuery, useGetDocQuery, useListReviewDocsQuery, useGetReviewDocQuery } = docApi;
