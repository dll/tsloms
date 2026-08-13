<template>
  <div class="settings-page">
    <!-- 用户信息卡片 -->
    <el-card shadow="never" class="info-card">
      <template #header>
        <span>用户信息</span>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="用户名">{{ authStore.user?.username || '-' }}</el-descriptions-item>
        <el-descriptions-item label="角色">{{ roleLabel }}</el-descriptions-item>
        <el-descriptions-item label="手机号">{{ authStore.user?.phone || '-' }}</el-descriptions-item>
        <el-descriptions-item label="用户ID">{{ authStore.user?.id || '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 修改手机号表单 -->
    <el-card shadow="never" class="form-card">
      <template #header>
        <span>修改手机号</span>
      </template>
      <el-form ref="formRef" :model="phoneForm" :rules="rules" label-width="100px" style="max-width: 500px">
        <el-form-item label="新手机号" prop="phone">
          <el-input v-model="phoneForm.phone" placeholder="请输入新手机号" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useAuthStore } from '@/store/auth'

const authStore = useAuthStore()
const formRef = ref<FormInstance>()
const saving = ref(false)

// 角色文字映射
const roleLabel = computed(() => {
  const role = authStore.user?.role
  const map: Record<string, string> = {
    admin: '管理员',
    operator: '运维人员',
    viewer: '查看人员',
  }
  return role ? (map[role] || role) : '-'
})

// 修改手机号表单
const phoneForm = reactive({
  phone: '',
})

// 表单校验规则
const rules: FormRules = {
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的手机号', trigger: 'blur' },
  ],
}

// 保存手机号（预留接口）
async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      // TODO: 调用后端更新手机号接口
      // await updatePhone({ phone: phoneForm.phone })
      ElMessage.success('手机号修改成功（功能开发中）')
    } catch {
      // 请求失败忽略
    } finally {
      saving.value = false
    }
  })
}

onMounted(async () => {
  // 获取用户信息
  if (authStore.token && !authStore.user) {
    try {
      await authStore.fetchUserInfo()
    } catch {
      // 请求失败忽略
    }
  }
})
</script>

<style scoped>
.settings-page {
  max-width: 800px;
}

.info-card {
  margin-bottom: 20px;
}

.form-card {
  border-radius: 4px;
}
</style>
