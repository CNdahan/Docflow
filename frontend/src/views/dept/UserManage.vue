<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import * as api from '@/api/admin';
import type { User } from '@/types';

const list = ref<User[]>([]);
const total = ref(0);
const loading = ref(false);
const page = ref(1);
const size = 20;

const dialogVisible = ref(false);
const editing = ref<User | null>(null);
const form = reactive({ username: '', password: '', real_name: '' });

const pwdDialogVisible = ref(false);
const pwdUserId = ref<number | null>(null);
const newPassword = ref('');

async function reload() {
  loading.value = true;
  try {
    const resp = await api.listUsers({ page: page.value, size });
    list.value = resp.items;
    total.value = resp.total;
  } finally { loading.value = false; }
}

function openCreate() {
  editing.value = null;
  Object.assign(form, { username: '', password: '', real_name: '' });
  dialogVisible.value = true;
}

function openEdit(u: User) {
  editing.value = u;
  Object.assign(form, { username: u.username, password: '', real_name: u.real_name });
  dialogVisible.value = true;
}

async function onSave() {
  if (editing.value) {
    await api.updateUser(editing.value.id, { real_name: form.real_name });
    ElMessage.success('已更新');
  } else {
    if (!form.username || !form.password) { ElMessage.warning('用户名和密码不能为空'); return; }
    await api.createUser({
      username: form.username,
      password: form.password,
      role: 'normal',
      real_name: form.real_name,
    });
    ElMessage.success('已创建');
  }
  dialogVisible.value = false;
  await reload();
}

async function toggleDisabled(u: User) {
  await ElMessageBox.confirm(`${u.disabled ? '启用' : '禁用'}用户 "${u.username}"?`, '提示');
  await api.updateUser(u.id, { disabled: !u.disabled });
  await reload();
}

function openResetPwd(u: User) {
  pwdUserId.value = u.id;
  newPassword.value = '';
  pwdDialogVisible.value = true;
}
async function onResetPwd() {
  if (!newPassword.value || newPassword.value.length < 8) { ElMessage.warning('新密码至少 8 位'); return; }
  await api.resetPassword(pwdUserId.value!, newPassword.value);
  ElMessage.success('密码已重置');
  pwdDialogVisible.value = false;
}

const importDialog = ref({ visible: false, submitting: false, password: 'init1234' });
const importResult = ref<api.ImportResult | null>(null);
const importFileRef = ref<File | null>(null);

async function onExport() {
  await api.downloadExcel('/users/export', '用户列表.xlsx');
}
async function onDownloadTemplate() {
  await api.downloadExcel('/users/export-template', '用户导入模板.xlsx');
}
function openImport() {
  importDialog.value = { visible: true, submitting: false, password: 'init1234' };
  importResult.value = null;
  importFileRef.value = null;
}
function onImportFileChange(file: any) {
  importFileRef.value = file.raw;
}
async function doImport() {
  if (!importFileRef.value) { ElMessage.warning('请选择文件'); return; }
  if (importDialog.value.password.length < 8) { ElMessage.warning('默认密码至少 8 位'); return; }
  importDialog.value.submitting = true;
  try {
    importResult.value = await api.importUsers(importFileRef.value, importDialog.value.password);
    ElMessage.success(`导入完成: 成功 ${importResult.value.success}/${importResult.value.total}`);
    await reload();
  } finally { importDialog.value.submitting = false; }
}

onMounted(reload);
</script>

<template>
  <el-card>
    <div class="toolbar">
      <h3 style="margin: 0">本部门用户管理</h3>
      <div>
        <el-button type="primary" @click="openCreate">+ 新建用户</el-button>
        <el-button @click="onExport">导出</el-button>
        <el-button @click="openImport">导入</el-button>
        <el-button @click="onDownloadTemplate">下载模板</el-button>
      </div>
    </div>

    <el-table :data="list" v-loading="loading" border style="margin-top: 16px">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="real_name" label="姓名" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.disabled ? 'danger' : 'success'">{{ row.disabled ? '已禁用' : '启用中' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="240">
        <template #default="{ row }">
          <el-button size="small" text @click="openEdit(row)">编辑</el-button>
          <el-button size="small" text type="warning" @click="openResetPwd(row)">重置密码</el-button>
          <el-button size="small" text :type="row.disabled ? 'success' : 'danger'" @click="toggleDisabled(row)">
            {{ row.disabled ? '启用' : '禁用' }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination layout="prev, pager, next" :total="total" :page-size="size" :current-page="page"
      style="margin-top: 16px; justify-content: flex-end; display: flex"
      @current-change="(v: number) => { page = v; reload(); }" />
  </el-card>

  <el-dialog v-model="dialogVisible" :title="editing ? '编辑用户' : '新建用户'" width="500px">
    <el-form label-position="top">
      <el-form-item label="用户名">
        <el-input v-model="form.username" :disabled="!!editing" />
      </el-form-item>
      <el-form-item label="姓名">
        <el-input v-model="form.real_name" />
      </el-form-item>
      <el-form-item label="初始密码 (至少 8 位)" v-if="!editing">
        <el-input v-model="form.password" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" @click="onSave">保存</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="pwdDialogVisible" title="重置密码" width="420px">
    <el-form label-position="top">
      <el-form-item label="新密码 (至少 8 位)">
        <el-input v-model="newPassword" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="pwdDialogVisible = false">取消</el-button>
      <el-button type="primary" @click="onResetPwd">确定</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="importDialog.visible" title="导入用户" width="560px">
    <el-form label-position="top">
      <el-form-item label="选择 Excel 文件">
        <el-upload :auto-upload="false" :show-file-list="true" :limit="1" accept=".xlsx,.xls"
          @change="onImportFileChange">
          <el-button type="primary">选择文件</el-button>
        </el-upload>
      </el-form-item>
      <el-form-item label="默认密码 (至少 8 位)">
        <el-input v-model="importDialog.password" placeholder="init1234" />
      </el-form-item>
    </el-form>
    <div v-if="importResult" style="margin-top: 12px">
      <el-alert :type="importResult.errors.length ? 'warning' : 'success'" :closable="false">
        <div>成功: {{ importResult.success }} / {{ importResult.total }}</div>
      </el-alert>
      <div v-if="importResult.errors.length" style="margin-top: 8px; max-height: 200px; overflow-y: auto">
        <div v-for="(e, i) in importResult.errors" :key="i" style="color: #f56c6c; font-size: 13px">{{ e }}</div>
      </div>
    </div>
    <template #footer>
      <el-button @click="importDialog.visible = false">关闭</el-button>
      <el-button type="primary" :loading="importDialog.submitting" @click="doImport">开始导入</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.toolbar { display: flex; justify-content: space-between; align-items: center; }
</style>
