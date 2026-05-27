<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue';
import VuePdfEmbed from 'vue-pdf-embed';
import 'vue-pdf-embed/dist/styles/textLayer.css';
import 'vue-pdf-embed/dist/styles/annotationLayer.css';
import client from '@/api/client';

const props = defineProps<{ src: string }>();
const error = ref('');
const fetching = ref(true);
const blobUrl = ref('');

async function load() {
  error.value = '';
  fetching.value = true;
  revoke();
  try {
    const path = props.src.replace(/^\/api\/v1/, '');
    const resp = await client.get(path, { responseType: 'blob' });
    blobUrl.value = URL.createObjectURL(resp.data);
  } catch (e: any) {
    error.value = e?.response?.status === 401 ? '未登录或无权限' : (e?.message || String(e));
  } finally {
    fetching.value = false;
  }
}

function revoke() {
  if (blobUrl.value) {
    URL.revokeObjectURL(blobUrl.value);
    blobUrl.value = '';
  }
}

function onError(err: any) {
  error.value = err?.message || String(err);
}

watch(() => props.src, load);
onMounted(load);
onUnmounted(revoke);
</script>

<template>
  <div class="pdf-container">
    <div v-if="error" class="err">PDF 加载失败: {{ error }}</div>
    <div v-else-if="fetching" v-loading="true" style="height: 200px"></div>
    <VuePdfEmbed v-else-if="blobUrl" :source="blobUrl" @loading-failed="onError"
      @rendering-failed="onError" />
  </div>
</template>

<style scoped>
.pdf-container {
  background: #f5f7fa;
  padding: 16px;
  border-radius: 4px;
  max-height: 80vh;
  overflow: auto;
}
.err {
  color: #f56c6c;
  padding: 16px;
}
</style>
