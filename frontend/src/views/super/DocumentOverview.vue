<script setup lang="ts">
import { onMounted, ref, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import dayjs from 'dayjs';
import { ElMessage, ElMessageBox } from 'element-plus';
import * as api from '@/api/documents';
import * as subApi from '@/api/attachments';
import AttachmentList from '@/components/AttachmentList.vue';
import StatusTag from '@/components/StatusTag.vue';
import type { DocumentDetail, DocumentOverview, UserSubmissionRow } from '@/types';

const route = useRoute();
const router = useRouter();
const docId = Number(route.params.id);

const detail = ref<DocumentDetail | null>(null);
const overview = ref<DocumentOverview | null>(null);
const loading = ref(false);

// 上报详情对话框
const detailDialog = ref({
  visible: false,
  loading: false,
  data: null as subApi.SubmissionDetail | null,
});

// 退回对话框
const returnDialog = ref({
  visible: false,
  submitting: false,
  submissionId: 0,
  reason: '',
});

async function reload() {
  loading.value = true;
  try {
    [detail.value, overview.value] = await Promise.all([
      api.getDocument(docId),
      api.getOverview(docId),
    ]);
  } finally {
    loading.value = false;
  }
}

function fmt(ts?: string | null) {
  return ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '-';
}

function scopeLabel(s?: string) {
  return s ? ({ DEPARTMENT: '指定部门', ALL_USERS: '全员', OWN_DEPARTMENT: '本部门' } as any)[s] || s : '-';
}

const progress = computed(() => {
  const s = overview.value?.summary;
  if (!s || !s.total) return 0;
  return Math.round(((s.submitted + s.late) * 100) / s.total);
});

async function onRecall() {
  await ElMessageBox.confirm('确定撤回这条公文?撤回后所有未上报记录将作废', '撤回确认', {
    type: 'warning',
  });
  await api.recallDocument(docId);
  ElMessage.success('已撤回');
  router.push('/super/documents');
}

async function openDetail(row: UserSubmissionRow) {
  if (!row.submission_id) {
    ElMessage.info('该用户尚未上报');
    return;
  }
  detailDialog.value = { visible: true, loading: true, data: null };
  try {
    detailDialog.value.data = await subApi.getSubmissionDetail(row.submission_id);
  } finally {
    detailDialog.value.loading = false;
  }
}

function openReturn(row: UserSubmissionRow) {
  if (!row.submission_id) {
    ElMessage.info('该用户尚未上报');
    return;
  }
  if (row.display_status !== 'SUBMITTED' && row.display_status !== 'SUBMITTED_LATE') {
    ElMessage.warning('仅"已上报"的记录可被退回');
    return;
  }
  returnDialog.value = { visible: true, submitting: false, submissionId: row.submission_id, reason: '' };
}

async function confirmReturn() {
  if (returnDialog.value.reason.trim().length < 5) {
    ElMessage.warning('退回原因至少 5 字');
    return;
  }
  returnDialog.value.submitting = true;
  try {
    await subApi.returnSubmission(returnDialog.value.submissionId, returnDialog.value.reason.trim());
    ElMessage.success('已退回');
    returnDialog.value.visible = false;
    await reload();
  } finally {
    returnDialog.value.submitting = false;
  }
}

function actionLabel(t: string) {
  return { SUBMIT: '提交', RETURN: '退回', RESUBMIT: '重新提交' }[t] || t;
}

onMounted(reload);
</script>

<template>
  <div v-loading="loading">
    <el-card v-if="detail" class="meta-card">
      <div class="header">
        <div>
          <h2 style="margin: 0">{{ detail.title }}</h2>
          <div class="meta">
            <span>发布人: {{ detail.publisher?.real_name || detail.publisher?.username }}</span>
            <el-divider direction="vertical" />
            <span>范围: {{ scopeLabel(detail.target_scope) }}</span>
            <el-divider direction="vertical" />
            <span>截止: {{ fmt(detail.deadline) }}</span>
            <el-divider direction="vertical" />
            <span>发布: {{ fmt(detail.created_at) }}</span>
          </div>
        </div>
        <div>
          <el-button @click="router.back()">返回</el-button>
          <el-button v-if="detail.status === 'ACTIVE'" type="danger" plain @click="onRecall">撤回</el-button>
          <el-tag v-else type="info">已撤回</el-tag>
        </div>
      </div>
    </el-card>

    <el-card style="margin-top: 16px" v-if="detail">
      <h3 style="margin-top: 0">公文正文</h3>
      <div class="content-html" v-html="detail.content_html"></div>

      <div v-if="detail.reading_attachments.length" style="margin-top: 16px">
        <AttachmentList :attachments="detail.reading_attachments" title="阅读附件" />
      </div>
      <div v-if="detail.template_attachments.length" style="margin-top: 16px">
        <AttachmentList :attachments="detail.template_attachments" title="上报模板" />
      </div>
    </el-card>

    <el-card style="margin-top: 16px" v-if="overview">
      <h3 style="margin-top: 0">上报纵览</h3>
      <el-progress :percentage="progress" :stroke-width="14" />
      <div class="summary">
        <div class="stat"><span class="num">{{ overview.summary.total }}</span> 应交</div>
        <div class="stat success"><span class="num">{{ overview.summary.submitted }}</span> 准时已交</div>
        <div class="stat warn"><span class="num">{{ overview.summary.late }}</span> 逾期已交</div>
        <div class="stat info"><span class="num">{{ overview.summary.pending }}</span> 待上报</div>
        <div class="stat danger"><span class="num">{{ overview.summary.overdue }}</span> 逾期未交</div>
        <div class="stat danger"><span class="num">{{ overview.summary.returned }}</span> 已退回</div>
      </div>

      <el-table v-if="overview.by_department?.length" :data="overview.by_department" border style="margin-top: 16px">
        <el-table-column prop="department_name" label="部门" />
        <el-table-column prop="total" label="应交" width="80" />
        <el-table-column prop="submitted" label="准时" width="80" />
        <el-table-column prop="late" label="逾期已交" width="100" />
        <el-table-column prop="pending" label="待交" width="80" />
        <el-table-column prop="overdue" label="逾期未交" width="100" />
        <el-table-column prop="returned" label="已退回" width="80" />
      </el-table>

      <h4 style="margin-top: 24px">用户明细</h4>
      <el-table :data="overview.by_user" border>
        <el-table-column prop="real_name" label="姓名" width="120" />
        <el-table-column prop="username" label="用户名" width="140" />
        <el-table-column prop="department_name" label="部门" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <StatusTag :status="row.display_status" />
          </template>
        </el-table-column>
        <el-table-column label="提交时间" width="160">
          <template #default="{ row }">{{ fmt(row.submitted_at) }}</template>
        </el-table-column>
        <el-table-column prop="return_count" label="退回次数" width="100" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary"
              :disabled="!row.submission_id || row.display_status === 'PENDING' || row.display_status === 'OVERDUE'"
              @click="openDetail(row)">
              查看
            </el-button>
            <el-button size="small" text type="warning"
              :disabled="row.display_status !== 'SUBMITTED' && row.display_status !== 'SUBMITTED_LATE'"
              @click="openReturn(row)">
              退回
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>

  <!-- 上报详情对话框 -->
  <el-dialog v-model="detailDialog.visible" title="上报详情" width="720px">
    <div v-loading="detailDialog.loading" style="min-height: 120px">
      <div v-if="detailDialog.data">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="状态">
            <StatusTag :status="detailDialog.data.display_status" />
          </el-descriptions-item>
          <el-descriptions-item label="提交时间">
            {{ fmt(detailDialog.data.submitted_at) }}
          </el-descriptions-item>
          <el-descriptions-item label="退回次数">
            {{ detailDialog.data.return_count }}
          </el-descriptions-item>
          <el-descriptions-item label="最后操作">
            {{ fmt(detailDialog.data.last_action_at) }}
          </el-descriptions-item>
          <el-descriptions-item v-if="detailDialog.data.note" label="备注" :span="2">
            {{ detailDialog.data.note }}
          </el-descriptions-item>
          <el-descriptions-item v-if="detailDialog.data.return_reason" label="退回原因" :span="2">
            <span style="color: #f56c6c">{{ detailDialog.data.return_reason }}</span>
          </el-descriptions-item>
        </el-descriptions>

        <div style="margin-top: 16px">
          <AttachmentList :attachments="detailDialog.data.attachments" title="上报附件" />
        </div>

        <div style="margin-top: 16px" v-if="detailDialog.data.actions.length">
          <strong>操作历史</strong>
          <el-timeline style="margin-top: 8px">
            <el-timeline-item v-for="a in detailDialog.data.actions" :key="a.id"
              :timestamp="fmt(a.created_at)"
              :type="a.action_type === 'RETURN' ? 'warning' : 'primary'">
              {{ actionLabel(a.action_type) }}
              <span v-if="a.reason" style="color: #909399; margin-left: 8px">— {{ a.reason }}</span>
            </el-timeline-item>
          </el-timeline>
        </div>
      </div>
    </div>
  </el-dialog>

  <!-- 退回对话框 -->
  <el-dialog v-model="returnDialog.visible" title="退回上报" width="500px">
    <el-form label-position="top">
      <el-form-item label="退回原因 (最少 5 字)">
        <el-input v-model="returnDialog.reason" type="textarea" :rows="4"
          placeholder="请说明退回原因,告知用户如何修改..." maxlength="500" show-word-limit />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="returnDialog.visible = false">取消</el-button>
      <el-button type="warning" :loading="returnDialog.submitting" @click="confirmReturn">确认退回</el-button>
    </template>
  </el-dialog>
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
.summary {
  display: flex;
  gap: 24px;
  margin-top: 16px;
  flex-wrap: wrap;
}
.stat {
  font-size: 14px;
  color: #606266;
}
.stat .num {
  font-size: 22px;
  font-weight: 600;
  color: #303133;
  margin-right: 4px;
}
.stat.success .num { color: #67c23a; }
.stat.warn .num { color: #e6a23c; }
.stat.info .num { color: #909399; }
.stat.danger .num { color: #f56c6c; }
</style>
