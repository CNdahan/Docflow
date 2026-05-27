<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import dayjs from 'dayjs';
import * as api from '@/api/documents';
import type { DocumentItem } from '@/types';

const router = useRouter();
const list = ref<DocumentItem[]>([]);
const total = ref(0);
const loading = ref(false);
const page = ref(1);
const size = 20;

async function reload() {
  loading.value = true;
  try {
    const resp = await api.listDocuments({ role_view: 'publish', page: page.value, size });
    list.value = resp.items;
    total.value = resp.total;
  } finally {
    loading.value = false;
  }
}

function fmt(ts?: string | null) {
  return ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '-';
}

function scopeLabel(s: string) {
  return { DEPARTMENT: '指定部门', ALL_USERS: '全员', OWN_DEPARTMENT: '本部门' }[s] || s;
}

function progress(d: DocumentItem) {
  const s = d.stats;
  if (!s || !s.total) return 0;
  return Math.round(((s.submitted + s.late) * 100) / s.total);
}

onMounted(reload);
</script>

<template>
  <el-card>
    <div class="toolbar">
      <h3 style="margin: 0">我发布的公文</h3>
      <el-button type="primary" @click="router.push('/super/documents/new')">+ 发布公文</el-button>
    </div>

    <el-table :data="list" v-loading="loading" border style="margin-top: 16px"
      @row-click="(row: DocumentItem) => router.push(`/super/documents/${row.id}`)">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="title" label="标题" min-width="200" />
      <el-table-column label="范围" width="120">
        <template #default="{ row }">{{ scopeLabel(row.target_scope) }}</template>
      </el-table-column>
      <el-table-column label="截止时间" width="160">
        <template #default="{ row }">{{ fmt(row.deadline) }}</template>
      </el-table-column>
      <el-table-column label="发布时间" width="160">
        <template #default="{ row }">{{ fmt(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="上报进度" width="200">
        <template #default="{ row }">
          <el-progress :percentage="progress(row)" />
          <span style="color: #909399; font-size: 12px">
            {{ (row.stats?.submitted || 0) + (row.stats?.late || 0) }}/{{ row.stats?.total || 0 }} 已交
          </span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'ACTIVE' ? 'success' : 'info'">
            {{ row.status === 'ACTIVE' ? '生效中' : '已撤回' }}
          </el-tag>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination layout="prev, pager, next" :total="total" :page-size="size" :current-page="page"
      style="margin-top: 16px; justify-content: flex-end; display: flex" @current-change="(v: number) => { page = v; reload(); }" />
  </el-card>
</template>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
:deep(.el-table__row) {
  cursor: pointer;
}
</style>
