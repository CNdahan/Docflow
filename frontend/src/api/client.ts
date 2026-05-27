import axios, { type AxiosInstance } from 'axios';
import { ElMessage } from 'element-plus';

const ACCESS_KEY = 'docflow.access_token';
const REFRESH_KEY = 'docflow.refresh_token';

export function getAccessToken(): string {
  return localStorage.getItem(ACCESS_KEY) || '';
}
export function setAccessToken(t: string) {
  localStorage.setItem(ACCESS_KEY, t);
}
export function getRefreshToken(): string {
  return localStorage.getItem(REFRESH_KEY) || '';
}
export function setRefreshToken(t: string) {
  localStorage.setItem(REFRESH_KEY, t);
}
export function clearTokens() {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

const client: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
});

client.interceptors.request.use((cfg) => {
  const tok = getAccessToken();
  if (tok) {
    cfg.headers = cfg.headers || {};
    cfg.headers.Authorization = `Bearer ${tok}`;
  }
  return cfg;
});

client.interceptors.response.use(
  (resp) => resp,
  (err) => {
    const status = err.response?.status;
    const data = err.response?.data;
    const msg = data?.message || err.message || '请求失败';
    if (status === 401) {
      clearTokens();
      if (!location.pathname.startsWith('/login')) {
        location.href = '/login';
      }
    } else if (status === 403) {
      ElMessage.error(`无权限: ${msg}`);
    } else if (status && status >= 500) {
      ElMessage.error(`服务异常: ${msg}`);
    } else {
      ElMessage.error(msg);
    }
    return Promise.reject(err);
  },
);

export default client;
