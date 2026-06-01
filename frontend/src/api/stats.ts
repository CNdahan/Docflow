import client from './client';
import type { GlobalOverview, DepartmentOverview } from '@/types';

export async function getGlobalOverview(): Promise<GlobalOverview> {
  const { data } = await client.get<GlobalOverview>('/stats/global');
  return data;
}

export async function getDepartmentOverview(deptId: number): Promise<DepartmentOverview> {
  const { data } = await client.get<DepartmentOverview>(`/stats/departments/${deptId}`);
  return data;
}

export async function exportDocumentOverview(docId: number, filename: string) {
  const resp = await client.get(`/stats/documents/${docId}/export`, { responseType: 'blob' });
  const url = URL.createObjectURL(resp.data);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
