<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import * as api from '@/api/admin';
import type { Department } from '@/types';

const list = ref<Department[]>([]);
const loading = ref(false);
const dialogVisible = ref(false);
const editing = ref<Department | null>(null);
const form = ref({ name: '' });

async function reload() {
  loading.value = true;
  try {
    list.value = await api.listDepartments();
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = null;
  form.value = { name: '' };
  dialogVisible.value = true;
}
function openEdit(d: Department) {
  editing.value = d;
  form.value = { name: d.name };
  dialogVisible.value = true;
}

async function onSave() {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入部门名称');
    return;
  }
  if (editing.value) {
    await api.updateDepartment(editing.value.id, { name: form.value.name });
    ElMessage.success('已更新');
  } else {
    await api.createDepartment(form.value.name);
    ElMessage.success('已创建');
  }
  dialogVisible.value = false;
  await reload();
}

async function toggleDisabled(d: Department) {
  await ElMessageBox.confirm(`确定${d.disabled ? '启用' : '禁用'}部门 "${d.name}"?`, '提示');
  await api.updateDepartment(d.id, { disabled: !d.disabled });
  await reload();
}

onMounted(reload);
</script>

<template>
  <el-card>
    <div class="toolbar">
      <h3 style="margin: 0">部门管理</h3>
      <el-button type="primary" @click="openCreate">+ 新建部门</el-button>
    </div>
    <el-table :data="list" v-loading="loading" border style="margin-top: 16px">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="部门名称" />
      <el-table-column prop="user_count" label="成员数" width="120" />
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="row.disabled ? 'danger' : 'success'">
            {{ row.disabled ? '已禁用' : '启用中' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button size="small" text @click="openEdit(row)">编辑</el-button>
          <el-button size="small" text :type="row.disabled ? 'success' : 'danger'" @click="toggleDisabled(row)">
            {{ row.disabled ? '启用' : '禁用' }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="dialogVisible" :title="editing ? '编辑部门' : '新建部门'" width="420px">
    <el-form label-position="top">
      <el-form-item label="部门名称">
        <el-input v-model="form.name" placeholder="例: 技术部" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" @click="onSave">保存</el-button>
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
