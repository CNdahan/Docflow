import client from './client';
import type { Department, User } from '@/types';

export async function listDepartments(): Promise<Department[]> {
  const { data } = await client.get<{ items: Department[] }>('/departments');
  return data.items || [];
}
export async function createDepartment(name: string): Promise<Department> {
  const { data } = await client.post<Department>('/departments', { name });
  return data;
}
export async function updateDepartment(id: number, patch: Partial<Department>): Promise<Department> {
  const { data } = await client.patch<Department>(`/departments/${id}`, patch);
  return data;
}

export interface UserListResp {
  items: User[];
  total: number;
  page: number;
  size: number;
}

export async function listUsers(params: {
  role?: string;
  department_id?: number;
  page?: number;
  size?: number;
}): Promise<UserListResp> {
  const { data } = await client.get<UserListResp>('/users', { params });
  return data;
}

export async function createUser(input: {
  username: string;
  password: string;
  role: string;
  department_id?: number;
  department_ids?: number[];
  real_name?: string;
}): Promise<User> {
  const { data } = await client.post<User>('/users', input);
  return data;
}

export async function updateUser(
  id: number,
  patch: { real_name?: string; disabled?: boolean; department_id?: number; department_ids?: number[] },
): Promise<User> {
  const { data } = await client.patch<User>(`/users/${id}`, patch);
  return data;
}

export async function resetPassword(id: number, newPassword: string) {
  await client.post(`/users/${id}/reset-password`, { new_password: newPassword });
}

export function exportUsersUrl(): string {
  return '/api/v1/users/export';
}

export function exportTemplateUrl(): string {
  return '/api/v1/users/export-template';
}

export interface ImportResult {
  total: number;
  success: number;
  errors: string[];
}

export async function importUsers(file: File, defaultPassword: string): Promise<ImportResult> {
  const fd = new FormData();
  fd.append('file', file);
  fd.append('default_password', defaultPassword);
  const { data } = await client.post<ImportResult>('/users/import', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
  return data;
}

export async function downloadExcel(url: string, filename: string) {
  const resp = await client.get(url, { responseType: 'blob' });
  const blobUrl = URL.createObjectURL(resp.data);
  const a = document.createElement('a');
  a.href = blobUrl;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(blobUrl), 1000);
}
