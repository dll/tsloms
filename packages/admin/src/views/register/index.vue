<template>
  <div class="register-container">
    <el-card class="register-card" shadow="always">
      <template #header>
        <div class="register-header">
          <el-icon :size="32" color="#409EFF"><UserFilled /></el-icon>
          <h2 class="register-title">TSLOMS 用户注册</h2>
        </div>
      </template>

      <el-form
        ref="regFormRef"
        :model="regForm"
        :rules="regRules"
        label-width="80px"
        @submit.prevent="handleRegister"
      >
        <el-form-item label="归属部门" prop="department_id">
          <el-select v-model="regForm.department_id" placeholder="请选择归属部门（选填）" clearable style="width:100%">
            <el-option v-for="d in deptOptions" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户名" prop="username">
          <el-input v-model="regForm.username" placeholder="请输入登录用户名（2-20位）" />
        </el-form-item>
        <el-form-item label="手机号" prop="phone">
          <el-input v-model="regForm.phone" placeholder="选填，11 位手机号" />
        </el-form-item>
        <el-form-item label="姓名" prop="real_name">
          <el-input v-model="regForm.real_name" placeholder="选填" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="regForm.password" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirm">
          <el-input v-model="regForm.confirm" type="password" show-password placeholder="再次输入密码" />
        </el-form-item>
        <el-form-item label="验证码" prop="captcha_code">
          <div class="captcha-row">
            <el-input v-model="regForm.captcha_code" placeholder="输入算式答案" />
            <div class="captcha-box" @click="loadCaptcha" :title="'点击刷新验证'">
              <span>{{ captchaQuestion || '刷新' }}</span>
            </div>
          </div>
        </el-form-item>
        <el-form-item>
          <div class="reg-note">注册后默认以「查看人员（只读）」角色使用，管理员可在用户管理中提升。</div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" style="width:100%" :loading="regLoading" @click="handleRegister">
            注 册
          </el-button>
        </el-form-item>
        <div class="register-back">
          <el-button type="primary" link @click="goLogin">已有账号？返回登录</el-button>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { UserFilled } from '@element-plus/icons-vue'
import { getCaptcha, register } from '@/api/auth'
import { getPublicDepartments } from '@/api/department'

const router = useRouter()

const regFormRef = ref<FormInstance>()
const regLoading = ref(false)
const captchaQuestion = ref('')
let captchaUUID = ''
const deptOptions = ref<any[]>([])

const regForm = reactive({
  username: '',
  phone: '',
  real_name: '',
  password: '',
  confirm: '',
  captcha_code: '',
  department_id: null as number | null,
})

const regRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 2, max: 20, message: '用户名长度 2-20 位', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少 6 位', trigger: 'blur' },
  ],
  confirm: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (_r, value, cb) => {
        if (value !== regForm.password) cb(new Error('两次密码不一致'))
        else cb()
      },
      trigger: 'blur',
    },
  ],
  captcha_code: [{ required: true, message: '请输入算式答案', trigger: 'blur' }],
}

function goLogin() {
  router.push('/login')
}

async function loadCaptcha() {
  try {
    const res = await getCaptcha()
    captchaUUID = res.data?.uuid || ''
    captchaQuestion.value = res.data?.question || ''
  } catch {
    captchaQuestion.value = '验证码加载失败，点击重试'
  }
}

async function loadDepts() {
  try {
    deptOptions.value = (await getPublicDepartments())?.data?.list || []
  } catch {
    /* 忽略 */
  }
}

async function handleRegister() {
  if (!regFormRef.value) return
  await regFormRef.value.validate(async (valid) => {
    if (!valid) return
    regLoading.value = true
    try {
      await register({
        username: regForm.username.trim(),
        password: regForm.password,
        phone: regForm.phone.trim() || undefined,
        real_name: regForm.real_name.trim() || undefined,
        department_id: regForm.department_id ?? undefined,
        captcha_uuid: captchaUUID,
        captcha_code: regForm.captcha_code,
      })
      ElMessage.success('注册成功，请登录')
      router.push('/login')
    } catch {
      loadCaptcha()
    } finally {
      regLoading.value = false
    }
  })
}

onMounted(() => {
  loadCaptcha()
  loadDepts()
})
</script>

<style scoped>
.register-container {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1a2a6c, #2a4858);
}
.register-card {
  width: 460px;
  border-radius: 8px;
}
.register-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
.register-title {
  margin: 0;
  font-size: 20px;
  color: #303133;
  text-align: center;
}
.captcha-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
.captcha-box {
  min-width: 110px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  font-size: 15px;
  font-weight: 600;
  color: #409EFF;
  background: #f5f7fa;
  cursor: pointer;
  user-select: none;
}
.reg-note {
  font-size: 12px;
  color: #909399;
  line-height: 1.5;
}
.register-back {
  text-align: center;
}
</style>
