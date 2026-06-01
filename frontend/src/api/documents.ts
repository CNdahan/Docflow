import client from './client';
import type { DocumentDetail, DocumentItem, DocumentOverview, DocumentRevision, TargetScope } from '@/types';

export interface ListDocsParams {
  role_view?: 'publish' | 'inbox';
  page?: number;
  size?: number;
}

export async function listDocuments(params: ListDocsParams = {}) {
  const { data } = await client.get<{ items: DocumentItem[]; total: number }>('/documents', { params });
  return data;
}

export interface PublishInput {
  title: string;
  content_html: string;
  target_scope: TargetScope;
  target_department_ids?: number[];
  deadline?: string | null;
  reading_attachment_ids?: number[];
  template_attachment_ids?: number[];
}

export async function publishDocument(input: PublishInput): Promise<DocumentItem> {
  const { data } = await client.post<DocumentItem>('/documents', input);
  return data;
}

export async function getDocument(id: number): Promise<DocumentDetail> {
  const { data } = await client.get<DocumentDetail>(`/documents/${id}`);
  return data;
}

export async function recallDocument(id: number) {
  await client.post(`/documents/${id}/recall`);
}

export async function getOverview(id: number, params: { page?: number; size?: number; status?: string } = {}): Promise<DocumentOverview> {
  const { data } = await client.get<DocumentOverview>(`/stats/documents/${id}`, { params });
  return data;
}

export interface UpdateDocInput {
  title?: string;
  content_html?: string;
  deadline?: string | null;
  clear_deadline?: boolean;
  add_reading_attachment_ids?: number[];
  add_template_attachment_ids?: number[];
  remove_attachment_ids?: number[];
}

export async function updateDocument(id: number, input: UpdateDocInput): Promise<DocumentItem> {
  const { data } = await client.patch<DocumentItem>(`/documents/${id}`, input);
  return data;
}

export async function listRevisions(id: number): Promise<DocumentRevision[]> {
  const { data } = await client.get<{ items: DocumentRevision[] }>(`/documents/${id}/revisions`);
  return data.items || [];
}
