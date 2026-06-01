<script setup lang="ts">
import { computed } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useAuthStore } from '@/stores/auth';
import {
  Document,
  Folder,
  User as UserIcon,
  Box,
  Bell,
  DataBoard,
} from '@element-plus/icons-vue';

const router = useRouter();
const route = useRoute();
const auth = useAuthStore();

const menuItems = computed(() => {
  switch (auth.role) {
    case 'super':
      return [
        { path: '/super/dashboard', label: '全局看板', icon: DataBoard },
        { path: '/super/documents', label: '公文管理', icon: Document },
        { path: '/super/documents/new', label: '发布公文', icon: Box },
        { path: '/super/departments', label: '部门管理', icon: Folder },
        { path: '/super/users', label: '用户管理', icon: UserIcon },
      ];
    case 'dept':
      return [
        { path: '/dept/documents', label: '公文管理', icon: Document },
        { path: '/dept/documents/new', label: '发布公文', icon: Box },
        { path: '/dept/users', label: '用户管理', icon: UserIcon },
      ];
    default:
      return [
        { path: '/normal/inbox', label: '我的公文', icon: Bell },
      ];
  }
});

async function onLogout() {
  await auth.logout();
  router.push('/login');
}

function go(path: string) {
  router.push(path);
}
</script>

<template>
  <el-container style="height: 100vh">
    <el-aside width="220px" class="aside">
      <div class="logo">
        <el-icon><Document /></el-icon>
        <span>DocFlow</span>
      </div>
      <el-menu :default-active="route.path" class="menu" router>
        <el-menu-item v-for="m in menuItems" :key="m.path" :index="m.path">
          <el-icon><component :is="m.icon" /></el-icon>
          <span>{{ m.label }}</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="title">公文下发管理系统</div>
        <div class="user">
          <el-tag size="small" type="info">{{ roleLabel(auth.role) }}</el-tag>
          <span style="margin: 0 12px">{{ auth.user?.real_name || auth.user?.username }}</span>
          <el-button size="small" text @click="onLogout">退出</el-button>
        </div>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script lang="ts">
function roleLabel(role: string) {
  return { super: '顶级用户', dept: '部门用户', normal: '普通用户' }[role] || role;
}
</script>

<style scoped>
.aside {
  background: #001529;
  color: #fff;
}
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 600;
  background: #002140;
}
.menu {
  background: #001529;
  border-right: none;
}
:deep(.el-menu) {
  --el-menu-bg-color: #001529;
  --el-menu-text-color: #cfd8dc;
  --el-menu-hover-bg-color: #002140;
  --el-menu-active-color: #fff;
}
.header {
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
}
.title {
  font-size: 18px;
  font-weight: 600;
}
.user {
  display: flex;
  align-items: center;
}
.main {
  background: #f5f7fa;
  padding: 20px;
}
</style>
