<script setup lang="ts">
import { ref } from 'vue';
import { previewUrl, downloadAttachment } from '@/api/attachments';
import PdfPreview from './PdfPreview.vue';
import type { Attachment } from '@/types';

defineProps<{ attachments: Attachment[]; title?: string; showDelete?: boolean }>();
const emit = defineEmits<{ delete: [id: number] }>();

const dialog = ref({ visible: false, src: '', title: '' });

function isPdf(a: Attachment) {
  return /\.pdf$/i.test(a.file_name) || a.mime_type === 'application/pdf';
}

function openPreview(a: Attachment) {
  dialog.value = {
    visible: true,
    src: previewUrl(a.id),
    title: a.file_name,
  };
}

async function onDownload(a: Attachment) {
  await downloadAttachment(a.id, a.file_name);
}

function fmtSize(n: number) {
  if (n < 1024) return n + 'B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + 'KB';
  return (n / 1024 / 1024).toFixed(1) + 'MB';
}
</script>

<template>
  <div v-if="title" class="att-title">{{ title }}</div>
  <div v-if="!attachments.length" style="color: #909399; padding: 8px 0">(无)</div>
  <div v-else class="att-list">
    <div v-for="a in attachments" :key="a.id" class="att-item">
      <span class="filename">
        <el-icon v-if="isPdf(a)" style="color: #f56c6c"><Document /></el-icon>
        <el-icon v-else style="color: #409eff"><Document /></el-icon>
        {{ a.file_name }}
        <small style="color: #909399; margin-left: 8px">{{ fmtSize(a.size_bytes) }}</small>
      </span>
      <span class="actions">
        <el-button v-if="isPdf(a)" size="small" text type="primary" @click="openPreview(a)">预览</el-button>
        <el-button size="small" text @click="onDownload(a)">下载</el-button>
        <el-button v-if="showDelete" size="small" text type="danger" @click="emit('delete', a.id)">删除</el-button>
      </span>
    </div>
  </div>

  <el-dialog v-model="dialog.visible" :title="dialog.title" width="80%" top="5vh">
    <PdfPreview v-if="dialog.visible" :src="dialog.src" />
  </el-dialog>
</template>

<script lang="ts">
import { Document } from '@element-plus/icons-vue';
export default { components: { Document } };
</script>

<style scoped>
.att-title {
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}
.att-list {
  border: 1px solid #ebeef5;
  border-radius: 4px;
}
.att-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  border-bottom: 1px solid #ebeef5;
}
.att-item:last-child {
  border-bottom: none;
}
.filename {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
