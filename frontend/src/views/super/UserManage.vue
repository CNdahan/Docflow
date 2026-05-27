<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import * as api from '@/api/admin';
import type { Department, User } from '@/types';

const list = ref<User[]>([]);
const depts = ref<Department[]>([]);
const total = ref(0);
const loading = ref(false);
const filter = reactive({ role: '', department_id: undefined as number | undefined });
const page = ref(1);
const size = 20;

const dialogVisible = ref(false);
const editing = ref<User | null>(null);
const form = reactive({
  username: '',
  password: '',
  role: 'user',
  department_id: undefined as number | undefined,
  real_name: '',
});

const pwdDialogVisible = ref(false);
const pwdUserId = ref<number | null>(null);
const newPassword = ref('');

function deptName(id?: number | null) {
  if (!id) return '-';
  return depts.value.find((d) => d.id === id)?.name || '-';
}

async function reload() {
  loading.value = true;
  try {
    const resp = await api.listUsers({
      role: filter.role || undefined,
      department_id: filter.department_id,
      page: page.value,
      size,
    });
    list.value = resp.items;
    total.value = resp.total;
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = null;
  Object.assign(form, {
    username: '', password: '', role: 'user',
    department_id: undefined, real_name: '',
  });
  dialogVisible.value = true;
}

function openEdit(u: User) {
  editing.value = u;
  Object.assign(form, {
    username: u.username,
    password: '',
    role: u.role,
    department_id: u.department_id ?? undefined,
    real_name: u.real_name,
  });
  dialogVisible.value = true;
}

async function onSave() {
  if (editing.value) {
    await api.updateUser(editing.value.id, {
      real_name: form.real_name,
      department_id: form.department_id,
    });
    ElMessage.success('已更新');
  } else {
    if (!form.username || !form.password) {
      ElMessage.warning('用户名和密码不能为空');
      return;
    }
    await api.createUser({
      username: form.username,
      password: form.password,
      role: form.role,
      department_id: form.role === 'super' ? undefined : form.department_id,
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
  if (!newPassword.value || newPassword.value.length < 8) {
    ElMessage.warning('新密码至少 8 位');
    return;
  }
  await api.resetPassword(pwdUserId.value!, newPassword.value);
  ElMessage.success('密码已重置');
  pwdDialogVisible.value = false;
}

onMounted(async () => {
  depts.value = await api.listDepartments();
  await reload();
});
</script>

<template>
  <el-card>
    <div class="toolbar">
      <h3 style="margin: 0">用户管理</h3>
      <div>
        <el-select v-model="filter.role" placeholder="全部角色" clearable style="width: 140px; margin-right: 8px"
          @change="reload">
          <el-option label="顶级用户" value="super" />
          <el-option label="部门用户" value="dept" />
          <el-option label="普通用户" value="user" />
        </el-select>
        <el-select v-model="filter.department_id" placeholder="全部部门" clearable style="width: 160px; margin-right: 8px"
          @change="reload">
          <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
        </el-select>
        <el-button type="primary" @click="openCreate">+ 新建用户</el-button>
      </div>
    </div>

    <el-table :data="list" v-loading="loading" border style="margin-top: 16px">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="real_name" label="姓名" />
      <el-table-column label="角色" width="110">
        <template #default="{ row }">
          <el-tag :type="row.role === 'super' ? 'danger' : row.role === 'dept' ? 'warning' : 'info'">
            {{ { super: '顶级', dept: '部门', user: '普通' }[row.role as string] }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="所属部门">
        <template #default="{ row }">{{ deptName(row.department_id) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.disabled ? 'danger' : 'success'">
            {{ row.disabled ? '已禁用' : '启用中' }}
          </el-tag>
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
      style="margin-top: 16px; justify-content: flex-end; display: flex" @current-change="(v: number) => { page = v; reload(); }" />
  </el-card>

  <el-dialog v-model="dialogVisible" :title="editing ? '编辑用户' : '新建用户'" width="500px">
    <el-form label-position="top">
      <el-form-item label="用户名">
        <el-input v-model="form.username" :disabled="!!editing" />
      </el-form-item>
      <el-form-item label="姓名">
        <el-input v-model="form.real_name" />
      </el-form-item>
      <el-form-item label="角色" v-if="!editing">
        <el-radio-group v-model="form.role">
          <el-radio value="user">普通用户</el-radio>
          <el-radio value="dept">部门用户</el-radio>
          <el-radio value="super">顶级用户</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="所属部门" v-if="form.role !== 'super'">
        <el-select v-model="form.department_id" placeholder="选择部门" style="width: 100%">
          <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
        </el-select>
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
</template>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
