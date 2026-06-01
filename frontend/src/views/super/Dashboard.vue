<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import * as statsApi from '@/api/stats';
import type { GlobalOverview } from '@/types';

const router = useRouter();
const data = ref<GlobalOverview | null>(null);
const loading = ref(false);

async function reload() {
  loading.value = true;
  try { data.value = await statsApi.getGlobalOverview(); }
  finally { loading.value = false; }
}

function completedRate(row: GlobalOverview['by_department'][0]) {
  if (!row.total) return 0;
  return Math.round(((row.submitted + row.late) * 100) / row.total);
}

onMounted(reload);
</script>

<template>
  <div v-loading="loading">
    <el-row :gutter="16" v-if="data">
      <el-col :span="8">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="num">{{ data.total_documents }}</div>
            <div class="label">公文总数</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="num" style="color: #67c23a">{{ data.active_documents }}</div>
            <div class="label">生效中</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="num" style="color: #909399">{{ data.total_documents - data.active_documents }}</div>
            <div class="label">已撤回</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card style="margin-top: 16px" v-if="data">
      <h3 style="margin-top: 0">按部门上报汇总</h3>
      <el-table :data="data.by_department" border>
        <el-table-column prop="department_name" label="部门" min-width="150" />
        <el-table-column prop="total" label="应交" width="80" />
        <el-table-column prop="submitted" label="准时已交" width="100" />
        <el-table-column prop="late" label="逾期已交" width="100" />
        <el-table-column prop="pending" label="待上报" width="80" />
        <el-table-column prop="overdue" label="逾期未交" width="100" />
        <el-table-column prop="returned" label="已退回" width="80" />
        <el-table-column label="完成率" width="200">
          <template #default="{ row }">
            <el-progress :percentage="completedRate(row)" />
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.stat-card { text-align: center; padding: 16px 0; }
.stat-card .num { font-size: 32px; font-weight: 700; }
.stat-card .label { margin-top: 8px; color: #606266; font-size: 14px; }
</style>
