import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import configTS from '@vue/eslint-config-typescript'

export default [
  // 忽略构建产物与依赖
  {
    ignores: ['dist/**', 'node_modules/**', 'public/**', 'cesium/**'],
  },
  // JS 基础推荐规则
  js.configs.recommended,
  // TypeScript 推荐规则（含 Vue SFC 解析）
  ...configTS(),
  // Vue3 基础 + 推荐规则
  ...pluginVue.configs['flat/recommended'],
  {
    rules: {
      // 兼容项目现有代码风格，关闭较严格但非必要的规则
      'vue/multi-word-component-names': 'off',
      'vue/no-v-html': 'off',
      'vue/require-default-prop': 'off',
      // 关闭纯格式化类规则（项目采用紧凑单行风格，不强制 vue 官方默认换行/属性排版）
      'vue/singleline-html-element-content-newline': 'off',
      'vue/multiline-html-element-content-newline': 'off',
      'vue/max-attributes-per-line': 'off',
      'vue/first-attribute-linebreak': 'off',
      'vue/html-closing-bracket-newline': 'off',
      'vue/attributes-order': 'off',
      'vue/max-len': 'off',
      'vue/html-self-closing': 'off',
      'vue/component-tags-order': 'off',
      // TS 规则适度放宽，聚焦真正的问题
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/ban-ts-comment': 'off',
      'no-undef': 'off',
    },
  },
]
