import client from './client';
import type { Submission } from '@/types';

export interface UploadResp {
  id: number;
  file_name: string;
  size_bytes: number;
  mime_type: string;
  preview_url: string;
  download_url: string;
}

export async function uploadAttachment(opts: {
  ownerType: 'DOCUMENT_DRAFT' | 'SUBMISSION' | 'INLINE';
  purpose?: 'READING' | 'TEMPLATE';
  ownerId?: number;
  file: File;
  onProgress?: (pct: number) => void;
}): Promise<UploadResp> {
  const fd = new FormData();
  fd.append('owner_type', opts.ownerType);
  if (opts.purpose) fd.append('purpose', opts.purpose);
  if (opts.ownerId != null) fd.append('owner_id', String(opts.ownerId));
  fd.append('file', opts.file);
  const { data } = await client.post<UploadResp>('/attachments', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
    onUploadProgress: (e) => {
      if (e.total && opts.onProgress) {
        opts.onProgress(Math.round((e.loaded * 100) / e.total));
      }
    },
  });
  return data;
}

export async function deleteAttachment(id: number) {
  await client.delete(`/attachments/${id}`);
}

export function downloadUrl(id: number): string {
  return `/api/v1/attachments/${id}/download`;
}
export function previewUrl(id: number): string {
  return `/api/v1/attachments/${id}/preview`;
}

// 用 axios 带 token 拉附件,触发浏览器保存
export async function downloadAttachment(id: number, filename: string): Promise<void> {
  const resp = await client.get(`/attachments/${id}/download`, { responseType: 'blob' });
  const url = URL.createObjectURL(resp.data);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename || `attachment-${id}`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export async function submitSubmission(documentId: number, attachmentIds: number[], note: string): Promise<Submission> {
  const { data } = await client.post<Submission>(`/submissions/${documentId}`, {
    attachment_ids: attachmentIds,
    note,
  });
  return data;
}

export interface MySubmissionItem {
  submission: Submission;
  document: import('@/types').DocumentItem;
  display_status: import('@/types').SubmissionStatus;
}

export async function listMySubmissions(params: { status?: string; page?: number; size?: number } = {}) {
  const { data } = await client.get<{ items: MySubmissionItem[]; total: number }>(
    '/submissions/mine',
    { params },
  );
  return data;
}

export interface SubmissionDetail {
  id: number;
  document_id: number;
  user_id: number;
  department_id?: number | null;
  current_status: import('@/types').SubmissionStatus;
  display_status: import('@/types').SubmissionStatus;
  submitted_at?: string | null;
  return_reason: string;
  return_count: number;
  note: string;
  last_action_at: string;
  attachments: import('@/types').Attachment[];
  document: import('@/types').DocumentItem;
  actions: Array<{
    id: number;
    submission_id: number;
    action_type: 'SUBMIT' | 'RETURN' | 'RESUBMIT';
    operator_id: number;
    reason: string;
    created_at: string;
  }>;
}

export async function getSubmissionDetail(id: number): Promise<SubmissionDetail> {
  const { data } = await client.get<SubmissionDetail>(`/submissions/${id}/detail`);
  return data;
}

export async function returnSubmission(id: number, reason: string) {
  await client.post(`/submissions/${id}/return`, { reason });
}
