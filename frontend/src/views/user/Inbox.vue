<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import dayjs from 'dayjs';
import { listMySubmissions, type MySubmissionItem } from '@/api/attachments';
import StatusTag from '@/components/StatusTag.vue';

const router = useRouter();
const list = ref<MySubmissionItem[]>([]);
const total = ref(0);
const loading = ref(false);
const status = ref('');
const page = ref(1);
const size = 20;

async function reload() {
  loading.value = true;
  try {
    const r = await listMySubmissions({ status: status.value || undefined, page: page.value, size });
    list.value = r.items;
    total.value = r.total;
  } finally {
    loading.value = false;
  }
}

function fmt(ts?: string | null) {
  return ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '-';
}

function openDoc(docId: number) {
  router.push(`/normal/documents/${docId}`);
}

onMounted(reload);
</script>

<template>
  <el-card>
    <div class="toolbar">
      <h3 style="margin: 0">我的公文</h3>
      <el-radio-group v-model="status" @change="reload">
        <el-radio-button value="">全部</el-radio-button>
        <el-radio-button value="PENDING">待上报</el-radio-button>
        <el-radio-button value="SUBMITTED">已上报</el-radio-button>
        <el-radio-button value="RETURNED">已退回</el-radio-button>
      </el-radio-group>
    </div>

    <el-table :data="list" v-loading="loading" border style="margin-top: 16px"
      @row-click="(row: MySubmissionItem) => openDoc(row.document.id)">
      <el-table-column label="标题" min-width="220">
        <template #default="{ row }">{{ row.document.title }}</template>
      </el-table-column>
      <el-table-column label="发布人" width="120">
        <template #default="{ row }">{{ row.document.publisher?.real_name || '-' }}</template>
      </el-table-column>
      <el-table-column label="截止" width="160">
        <template #default="{ row }">{{ fmt(row.document.deadline) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <StatusTag :status="row.display_status" />
        </template>
      </el-table-column>
      <el-table-column label="提交时间" width="160">
        <template #default="{ row }">{{ fmt(row.submission.submitted_at) }}</template>
      </el-table-column>
    </el-table>
    <el-pagination layout="prev, pager, next" :total="total" :page-size="size" :current-page="page"
      style="margin-top: 16px; justify-content: flex-end; display: flex"
      @current-change="(v: number) => { page = v; reload(); }" />
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
