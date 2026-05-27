declare module '*.vue' {
  import type { DefineComponent } from 'vue';
  const component: DefineComponent<{}, {}, any>;
  export default component;
}

declare module 'element-plus/dist/locale/zh-cn.mjs';

declare module '@wangeditor/editor-for-vue' {
  import type { DefineComponent } from 'vue';
  export const Editor: DefineComponent<any, any, any>;
  export const Toolbar: DefineComponent<any, any, any>;
}

declare module 'vue-pdf-embed' {
  import type { DefineComponent } from 'vue';
  const VuePdfEmbed: DefineComponent<any, any, any>;
  export default VuePdfEmbed;
}

declare module 'vue-pdf-embed/dist/styles/textLayer.css';
declare module 'vue-pdf-embed/dist/styles/annotationLayer.css';

