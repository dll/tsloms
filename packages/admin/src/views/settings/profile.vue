<template>
  <div class="profile-page">
    <el-row :gutter="20">
      <!-- 左侧：工作照/头像 -->
      <el-col :span="8">
        <el-card shadow="never">
          <template #header><span>工作照 / 头像</span></template>
          <div class="avatar-section">
            <el-avatar :size="120" :src="authStore.user?.avatar" class="avatar-preview">
              {{ (authStore.user?.real_name || authStore.user?.username || 'U').charAt(0) }}
            </el-avatar>
            <el-upload
              :show-file-list="false"
              :http-request="handleAvatarUpload"
              accept="image/*"
            >
              <el-button type="primary" size="small" :loading="uploading">上传工作照</el-button>
            </el-upload>
            <div class="avatar-tip">支持 jpg/png/webp，最大 5MB；显示在右上角头像位置</div>
          </div>
        </el-card>
      </el-col>

      <!-- 右侧：人事资料 -->
      <el-col :span="16">
        <el-card shadow="never">
          <template #header><span>个人资料（人事档案）</span></template>
          <el-form :model="form" label-width="110px" v-loading="loading">
            <el-form-item label="账号">
              <el-input :value="authStore.user?.username" disabled />
            </el-form-item>
            <el-form-item label="姓名">
              <el-input v-model="form.real_name" placeholder="姓名" />
            </el-form-item>
            <el-form-item label="手机号">
              <el-input v-model="form.phone" placeholder="11位手机号，即登录账号" maxlength="11" />
            </el-form-item>
            <el-form-item label="工号">
              <el-input v-model="form.work_no" placeholder="组织单位分配的工作编号" />
            </el-form-item>
            <el-form-item label="性别">
              <el-select v-model="form.gender" placeholder="请选择" clearable style="width: 100%">
                <el-option label="男" value="male" />
                <el-option label="女" value="female" />
              </el-select>
            </el-form-item>
            <el-form-item label="身份证号">
              <el-input v-model="form.id_card" placeholder="身份证号" maxlength="18" />
            </el-form-item>
            <el-form-item label="住址">
              <el-input v-model="form.address" placeholder="住址" />
            </el-form-item>
            <el-form-item label="文化程度">
              <el-select v-model="form.education" placeholder="请选择" clearable style="width: 100%">
                <el-option v-for="d in ['初中','高中','中专','大专','本科','硕士','博士','其他']" :key="d" :label="d" :value="d" />
              </el-select>
            </el-form-item>
            <el-form-item label="工程等级">
              <el-select v-model="form.engineer_level" placeholder="请选择" clearable style="width: 100%">
                <el-option v-for="d in ['初级','中级','高级','工程师','高级工程师']" :key="d" :label="d" :value="d" />
              </el-select>
            </el-form-item>
            <el-form-item label="邮箱">
              <el-input v-model="form.email" placeholder="邮箱（可选）" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saving" @click="handleSave">保存资料</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, type UploadRequestOptions } from 'element-plus'
import { useAuthStore } from '@/store/auth'
import { updateMyProfile, uploadMyAvatar } from '@/api/auth'

const authStore = useAuthStore()
const loading = ref(false)
const saving = ref(false)
const uploading = ref(false)

const form = reactive({
  real_name: '',
  phone: '',
  work_no: '',
  gender: '',
  id_card: '',
  address: '',
  education: '',
  engineer_level: '',
  email: '',
})

function syncForm() {
  const u = authStore.user as any
  if (!u) return
  form.real_name = u.real_name || ''
  form.phone = u.phone || ''
  form.work_no = u.work_no || ''
  form.gender = u.gender || ''
  form.id_card = u.id_card || ''
  form.address = u.address || ''
  form.education = u.education || ''
  form.engineer_level = u.engineer_level || ''
  form.email = u.email || ''
}

async function handleSave() {
  if (form.phone && !/^1\d{10}$/.test(form.phone)) {
    ElMessage.warning('手机号格式不正确（需 11 位）')
    return
  }
  saving.value = true
  try {
    const res = await updateMyProfile({ ...form })
    authStore.user = res.data?.user as any
    ElMessage.success('资料已更新')
  } finally {
    saving.value = false
  }
}

async function handleAvatarUpload(opt: UploadRequestOptions) {
  const fd = new FormData()
  fd.append('file', opt.file as any)
  uploading.value = true
  try {
    const res = await uploadMyAvatar(fd)
    authStore.user = { ...authStore.user, avatar: res.data?.avatar } as any
    ElMessage.success('工作照已上传')
  } catch {
    // 拦截器已提示
  } finally {
    uploading.value = false
    opt.onSuccess?.({})
  }
}

onMounted(async () => {
  if (!authStore.user) {
    try {
      await authStore.fetchUserInfo()
    } catch { /* 忽略 */ }
  }
  syncForm()
})
</script>

<style scoped>
.avatar-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding: 8px;
}
.avatar-preview {
  background: #409eff;
  color: #fff;
  font-size: 40px;
}
.avatar-tip {
  font-size: 12px;
  color: #909399;
  text-align: center;
}
</style>
