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
  real_name?: string;
}): Promise<User> {
  const { data } = await client.post<User>('/users', input);
  return data;
}

export async function updateUser(
  id: number,
  patch: { real_name?: string; disabled?: boolean; department_id?: number },
): Promise<User> {
  const { data } = await client.patch<User>(`/users/${id}`, patch);
  return data;
}

export async function resetPassword(id: number, newPassword: string) {
  await client.post(`/users/${id}/reset-password`, { new_password: newPassword });
}
