<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue';
import '@wangeditor/editor/dist/css/style.css';
import { Editor, Toolbar } from '@wangeditor/editor-for-vue';
import type { IDomEditor, IEditorConfig, IToolbarConfig } from '@wangeditor/editor';
import { ElMessage } from 'element-plus';
import { useRouter } from 'vue-router';
import dayjs from 'dayjs';
import * as docsApi from '@/api/documents';
import * as attApi from '@/api/attachments';
import { listDepartments } from '@/api/admin';
import type { Department } from '@/types';

const router = useRouter();

const title = ref('');
const targetScope = ref<'DEPARTMENT' | 'ALL_USERS'>('DEPARTMENT');
const targetDeptIds = ref<number[]>([]);
const deadline = ref<string>('');
const noDeadline = ref(false);
const depts = ref<Department[]>([]);

// 富文本
const editorRef = shallowRef<IDomEditor>();
const content = ref('');
const toolbarConfig: Partial<IToolbarConfig> = {};
const editorConfig: Partial<IEditorConfig> = {
  placeholder: '请输入公文正文...',
  MENU_CONF: {
    uploadImage: {
      async customUpload(file: File, insertFn: (url: string) => void) {
        try {
          const r = await attApi.uploadAttachment({
            ownerType: 'INLINE',
            file,
          });
          insertFn(`/api/v1/attachments/${r.id}/preview`);
        } catch {
          ElMessage.error('图片上传失败');
        }
      },
    },
  },
};

onBeforeUnmount(() => {
  editorRef.value?.destroy();
});

function onCreated(e: IDomEditor) {
  editorRef.value = e;
}

// 附件
interface UploadedAtt {
  id: number;
  file_name: string;
  size_bytes: number;
}
const readingAtts = ref<UploadedAtt[]>([]);
const templateAtts = ref<UploadedAtt[]>([]);
const uploading = ref(false);

async function doUpload(file: File, purpose: 'READING' | 'TEMPLATE') {
  uploading.value = true;
  try {
    const r = await attApi.uploadAttachment({
      ownerType: 'DOCUMENT_DRAFT',
      purpose,
      file,
    });
    const item = { id: r.id, file_name: r.file_name, size_bytes: r.size_bytes };
    if (purpose === 'READING') readingAtts.value.push(item);
    else templateAtts.value.push(item);
    ElMessage.success('上传成功: ' + r.file_name);
  } catch {
    // 拦截器已弹错
  } finally {
    uploading.value = false;
  }
  return false; // 阻止 el-upload 默认行为
}

async function removeAtt(id: number, type: 'reading' | 'template') {
  await attApi.deleteAttachment(id);
  if (type === 'reading') {
    readingAtts.value = readingAtts.value.filter((a) => a.id !== id);
  } else {
    templateAtts.value = templateAtts.value.filter((a) => a.id !== id);
  }
}

function fmtSize(n: number) {
  if (n < 1024) return n + 'B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + 'KB';
  return (n / 1024 / 1024).toFixed(1) + 'MB';
}

const canSubmit = computed(() => {
  if (!title.value.trim()) return false;
  if (!content.value || content.value === '<p><br></p>') return false;
  if (targetScope.value === 'DEPARTMENT' && targetDeptIds.value.length === 0) return false;
  return true;
});

const submitting = ref(false);

async function onSubmit() {
  if (!canSubmit.value) {
    ElMessage.warning('请完整填写表单');
    return;
  }
  submitting.value = true;
  try {
    const doc = await docsApi.publishDocument({
      title: title.value.trim(),
      content_html: content.value,
      target_scope: targetScope.value,
      target_department_ids: targetScope.value === 'DEPARTMENT' ? targetDeptIds.value : undefined,
      deadline: noDeadline.value || !deadline.value ? null : dayjs(deadline.value).toISOString(),
      reading_attachment_ids: readingAtts.value.map((a) => a.id),
      template_attachment_ids: templateAtts.value.map((a) => a.id),
    });
    ElMessage.success('公文已发布');
    router.push(`/super/documents/${doc.id}`);
  } catch {
    // 拦截器
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  depts.value = (await listDepartments()).filter((d) => !d.disabled);
});
</script>

<template>
  <el-card>
    <div class="toolbar">
      <h3 style="margin: 0">发布公文</h3>
      <div>
        <el-button @click="router.back()">取消</el-button>
        <el-button type="primary" :loading="submitting" :disabled="!canSubmit" @click="onSubmit">发布</el-button>
      </div>
    </div>

    <el-form label-position="top" style="margin-top: 16px">
      <el-form-item label="标题">
        <el-input v-model="title" placeholder="公文标题" maxlength="200" show-word-limit />
      </el-form-item>

      <el-form-item label="发送范围">
        <el-radio-group v-model="targetScope">
          <el-radio value="DEPARTMENT">指定部门</el-radio>
          <el-radio value="ALL_USERS">全员</el-radio>
        </el-radio-group>
      </el-form-item>

      <el-form-item v-if="targetScope === 'DEPARTMENT'" label="目标部门 (可多选)">
        <el-select v-model="targetDeptIds" multiple placeholder="请选择部门" style="width: 100%">
          <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
        </el-select>
      </el-form-item>

      <el-form-item label="截止时间">
        <el-date-picker v-model="deadline" type="datetime" placeholder="选择截止日期时间" :disabled="noDeadline"
          format="YYYY-MM-DD HH:mm" value-format="YYYY-MM-DD HH:mm:00" />
        <el-checkbox v-model="noDeadline" style="margin-left: 12px">不设截止</el-checkbox>
      </el-form-item>

      <el-form-item label="正文">
        <div style="border: 1px solid #dcdfe6; border-radius: 4px; width: 100%">
          <Toolbar :editor="editorRef" :defaultConfig="toolbarConfig" mode="default" style="border-bottom: 1px solid #dcdfe6" />
          <Editor v-model="content" :defaultConfig="editorConfig" mode="default" style="height: 400px; overflow-y: auto"
            @onCreated="onCreated" />
        </div>
      </el-form-item>

      <el-form-item label="阅读附件 (供下级查看)">
        <el-upload :auto-upload="true" :show-file-list="false"
          :http-request="(opt: any) => doUpload(opt.file, 'READING')" :disabled="uploading">
          <el-button type="primary" plain :loading="uploading">+ 上传阅读附件</el-button>
        </el-upload>
        <div style="margin-top: 8px; width: 100%">
          <div v-for="a in readingAtts" :key="a.id" class="att-row">
            <span>📄 {{ a.file_name }} <small style="color: #909399">{{ fmtSize(a.size_bytes) }}</small></span>
            <el-button size="small" text type="danger" @click="removeAtt(a.id, 'reading')">删除</el-button>
          </div>
        </div>
      </el-form-item>

      <el-form-item label="上报模板 (供下级下载填写)">
        <el-upload :auto-upload="true" :show-file-list="false"
          :http-request="(opt: any) => doUpload(opt.file, 'TEMPLATE')" :disabled="uploading">
          <el-button type="success" plain :loading="uploading">+ 上传模板</el-button>
        </el-upload>
        <div style="margin-top: 8px; width: 100%">
          <div v-for="a in templateAtts" :key="a.id" class="att-row">
            <span>📋 {{ a.file_name }} <small style="color: #909399">{{ fmtSize(a.size_bytes) }}</small></span>
            <el-button size="small" text type="danger" @click="removeAtt(a.id, 'template')">删除</el-button>
          </div>
        </div>
      </el-form-item>
    </el-form>
  </el-card>
</template>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.att-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
  border-bottom: 1px dashed #ebeef5;
}
.att-row:last-child {
  border-bottom: none;
}
</style>
