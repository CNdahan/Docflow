<script setup lang="ts">
import type { SubmissionStatus } from '@/types';
defineProps<{ status: SubmissionStatus }>();

const COLOR: Record<SubmissionStatus, string> = {
  PENDING: 'info',
  SUBMITTED: 'success',
  SUBMITTED_LATE: 'warning',
  OVERDUE: 'danger',
  RETURNED: 'danger',
};
const LABEL: Record<SubmissionStatus, string> = {
  PENDING: '待上报',
  SUBMITTED: '准时已交',
  SUBMITTED_LATE: '逾期已交',
  OVERDUE: '逾期未交',
  RETURNED: '已退回',
};
</script>

<template>
  <el-tag :type="COLOR[status] as any">{{ LABEL[status] || status }}</el-tag>
</template>
