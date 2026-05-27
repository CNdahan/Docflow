import client, { setAccessToken, setRefreshToken, clearTokens } from './client';
import type { User } from '@/types';

export interface LoginResp {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user: User;
}

export async function login(username: string, password: string): Promise<LoginResp> {
  const { data } = await client.post<LoginResp>('/auth/login', { username, password });
  setAccessToken(data.access_token);
  setRefreshToken(data.refresh_token);
  return data;
}

export async function logout(): Promise<void> {
  try {
    await client.post('/auth/logout');
  } finally {
    clearTokens();
  }
}
