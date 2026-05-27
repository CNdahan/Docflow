import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';
import { useAuthStore } from '@/stores/auth';

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: () => useAuthStore().homeRoute() },
  { path: '/login', name: 'login', component: () => import('@/views/login/LoginView.vue') },

  // 顶级用户
  {
    path: '/super',
    component: () => import('@/views/layout/AppLayout.vue'),
    meta: { roles: ['super'] },
    children: [
      { path: '', redirect: '/super/documents' },
      { path: 'departments', component: () => import('@/views/super/DepartmentManage.vue') },
      { path: 'users', component: () => import('@/views/super/UserManage.vue') },
      { path: 'documents', component: () => import('@/views/super/DocumentList.vue') },
      { path: 'documents/new', component: () => import('@/views/super/DocumentPublish.vue') },
      { path: 'documents/:id', component: () => import('@/views/super/DocumentOverview.vue') },
    ],
  },

  // 部门用户 (M2 完整实现, M1 仅占位避免 404)
  {
    path: '/dept',
    component: () => import('@/views/layout/AppLayout.vue'),
    meta: { roles: ['dept'] },
    children: [
      { path: '', redirect: '/dept/documents' },
      { path: 'documents', component: () => import('@/views/dept/DocumentList.vue') },
    ],
  },

  // 普通用户
  {
    path: '/user',
    component: () => import('@/views/layout/AppLayout.vue'),
    meta: { roles: ['user'] },
    children: [
      { path: '', redirect: '/user/inbox' },
      { path: 'inbox', component: () => import('@/views/user/Inbox.vue') },
      { path: 'documents/:id', component: () => import('@/views/user/DocumentDetail.vue') },
    ],
  },

  { path: '/:pathMatch(.*)*', component: () => import('@/views/error/NotFound.vue') },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach((to) => {
  const auth = useAuthStore();
  if (to.path === '/login') return true;
  if (!auth.isLoggedIn) return { path: '/login', query: { redirect: to.fullPath } };
  const allowed = to.matched.find((r) => r.meta.roles)?.meta.roles as string[] | undefined;
  if (allowed && !allowed.includes(auth.role)) {
    return auth.homeRoute();
  }
  return true;
});

export default router;
