import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import type { User } from '@/types';
import { login as apiLogin, logout as apiLogout } from '@/api/auth';
import { getAccessToken } from '@/api/client';

const USER_KEY = 'docflow.user';

function loadUser(): User | null {
  const raw = localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(loadUser());
  const isLoggedIn = computed(() => !!user.value && !!getAccessToken());
  const role = computed(() => user.value?.role || '');

  async function login(username: string, password: string) {
    const resp = await apiLogin(username, password);
    user.value = resp.user;
    localStorage.setItem(USER_KEY, JSON.stringify(resp.user));
    return resp;
  }

  async function logout() {
    try {
      await apiLogout();
    } finally {
      user.value = null;
      localStorage.removeItem(USER_KEY);
    }
  }

  function homeRoute(): string {
    if (!user.value) return '/login';
    switch (user.value.role) {
      case 'super':
        return '/super/documents';
      case 'dept':
        return '/dept/documents';
      default:
        return '/user/inbox';
    }
  }

  return { user, role, isLoggedIn, login, logout, homeRoute };
});
