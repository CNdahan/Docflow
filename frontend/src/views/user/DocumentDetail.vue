<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import dayjs from 'dayjs';
import { ElMessage } from 'element-plus';
import * as docsApi from '@/api/documents';
import * as attApi from '@/api/attachments';
import AttachmentList from '@/components/AttachmentList.vue';
import StatusTag from '@/components/StatusTag.vue';
import type { DocumentDetail, SubmissionStatus } from '@/types';

const route = useRoute();
const router = useRouter();
const docId = Number(route.params.id);

const detail = ref<DocumentDetail | null>(null);
const loading = ref(false);

interface PendingAtt {
  id: number;
  file_name: string;
  size_bytes: number;
}
const pendingAtts = ref<PendingAtt[]>([]);
const note = ref('');
const uploading = ref(false);
const submitting = ref(false);

async function reload() {
  loading.value = true;
  try {
    detail.value = await docsApi.getDocument(docId);
    note.value = detail.value.my_submission?.note || '';
  } finally {
    loading.value = false;
  }
}

async function doUpload(file: File) {
  uploading.value = true;
  try {
    const r = await attApi.uploadAttachment({ ownerType: 'DOCUMENT_DRAFT', file });
    pendingAtts.value.push({ id: r.id, file_name: r.file_name, size_bytes: r.size_bytes });
    ElMessage.success('上传成功: ' + r.file_name);
  } catch { /* 拦截器已处理 */ } finally {
    uploading.value = false;
  }
  return false;
}

async function removePending(id: number) {
  await attApi.deleteAttachment(id);
  pendingAtts.value = pendingAtts.value.filter((a) => a.id !== id);
}

const status = computed<SubmissionStatus>(() =>
  detail.value?.my_submission?.display_status || detail.value?.my_submission?.current_status || 'PENDING',
);

const canSubmit = computed(() => {
  if (!detail.value) return false;
  if (status.value === 'SUBMITTED' || status.value === 'SUBMITTED_LATE') return false;
  return pendingAtts.value.length > 0;
});

async function onSubmit() {
  if (!canSubmit.value) {
    ElMessage.warning('请至少上传一个附件');
    return;
  }
  submitting.value = true;
  try {
    await attApi.submitSubmission(docId, pendingAtts.value.map((a) => a.id), note.value);
    ElMessage.success('已提交');
    pendingAtts.value = [];
    await reload();
  } catch { /* */ } finally {
    submitting.value = false;
  }
}

function fmt(ts?: string | null) {
  return ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '-';
}
function fmtSize(n: number) {
  if (n < 1024) return n + 'B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + 'KB';
  return (n / 1024 / 1024).toFixed(1) + 'MB';
}

onMounted(reload);
</script>

<template>
  <div v-loading="loading">
    <el-card v-if="detail">
      <div class="header">
        <div>
          <h2 style="margin: 0">{{ detail.title }}</h2>
          <div class="meta">
            <span>发布人: {{ detail.publisher?.real_name || detail.publisher?.username }}</span>
            <el-divider direction="vertical" />
            <span>截止: {{ fmt(detail.deadline) }}</span>
            <el-divider direction="vertical" />
            <span>发布: {{ fmt(detail.created_at) }}</span>
          </div>
        </div>
        <div>
          <el-button @click="router.back()">返回</el-button>
          <StatusTag :status="status" />
        </div>
      </div>
    </el-card>

    <el-alert v-if="status === 'RETURNED' && detail?.my_submission?.return_reason" type="warning" :closable="false"
      style="margin-top: 16px">
      <strong>您的上报已被退回</strong>
      <div>退回原因: {{ detail.my_submission.return_reason }}</div>
      <div>已退回 {{ detail.my_submission.return_count }} 次</div>
    </el-alert>

    <el-card v-if="detail" style="margin-top: 16px">
      <h3 style="margin-top: 0">公文正文</h3>
      <div class="content-html" v-html="detail.content_html"></div>

      <div v-if="detail.reading_attachments.length" style="margin-top: 16px">
        <AttachmentList :attachments="detail.reading_attachments" title="阅读附件" />
      </div>
      <div v-if="detail.template_attachments.length" style="margin-top: 16px">
        <AttachmentList :attachments="detail.template_attachments" title="上报模板 (请下载填写)" />
      </div>
    </el-card>

    <el-card v-if="detail" style="margin-top: 16px">
      <h3 style="margin-top: 0">我的上报</h3>

      <div v-if="detail.my_submission && (status === 'SUBMITTED' || status === 'SUBMITTED_LATE')">
        <p style="color: #67c23a">您已提交,提交时间: {{ fmt(detail.my_submission.submitted_at) }}</p>
        <AttachmentList :attachments="detail.my_submission.attachments || []" title="已上传附件" />
        <p v-if="detail.my_submission.note" style="margin-top: 12px">
          <strong>备注:</strong> {{ detail.my_submission.note }}
        </p>
        <el-alert :closable="false" type="info" style="margin-top: 12px">
          如需修改,请联系上级退回。
        </el-alert>
      </div>

      <div v-else>
        <el-form label-position="top">
          <el-form-item label="上传上报附件">
            <el-upload :auto-upload="true" :show-file-list="false" :http-request="(opt: any) => doUpload(opt.file)"
              :disabled="uploading">
              <el-button type="primary" :loading="uploading">+ 上传附件</el-button>
            </el-upload>
            <div style="margin-top: 8px; width: 100%">
              <div v-for="a in pendingAtts" :key="a.id" class="att-row">
                <span>📎 {{ a.file_name }} <small style="color: #909399">{{ fmtSize(a.size_bytes) }}</small></span>
                <el-button size="small" text type="danger" @click="removePending(a.id)">删除</el-button>
              </div>
            </div>
          </el-form-item>

          <el-form-item label="备注 (可选)">
            <el-input v-model="note" type="textarea" :rows="3" placeholder="补充说明..." />
          </el-form-item>

          <el-button type="primary" :loading="submitting" :disabled="!canSubmit" @click="onSubmit">
            {{ status === 'RETURNED' ? '重新提交' : '提交上报' }}
          </el-button>
        </el-form>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}
.meta {
  margin-top: 8px;
  color: #606266;
  font-size: 13px;
}
.content-html :deep(p),
.content-html :deep(li) {
  line-height: 1.7;
}
.content-html :deep(img) {
  max-width: 100%;
}
.att-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  border-bottom: 1px dashed #ebeef5;
}
.att-row:last-child {
  border-bottom: none;
}
</style>
