<script setup lang="ts">
import { onMounted, ref } from 'vue';
import dayjs from 'dayjs';
import { useRouter } from 'vue-router';
import * as api from '@/api/documents';
import type { DocumentItem } from '@/types';

const router = useRouter();
const list = ref<DocumentItem[]>([]);
const loading = ref(false);

async function reload() {
  loading.value = true;
  try {
    const r = await api.listDocuments();
    list.value = r.items;
  } finally {
    loading.value = false;
  }
}

function fmt(ts?: string | null) {
  return ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '-';
}

onMounted(reload);
</script>

<template>
  <el-card>
    <h3 style="margin-top: 0">部门用户视图 (M2 实现)</h3>
    <el-alert type="info" :closable="false" style="margin-bottom: 16px">
      部门用户的发布、本部门管理、退回审核等功能将在 M2 阶段实现。当前页面仅展示相关公文清单。
    </el-alert>
    <el-table :data="list" v-loading="loading" border>
      <el-table-column prop="title" label="标题" min-width="200" />
      <el-table-column label="截止" width="160">
        <template #default="{ row }">{{ fmt(row.deadline) }}</template>
      </el-table-column>
      <el-table-column label="发布" width="160">
        <template #default="{ row }">{{ fmt(row.created_at) }}</template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
