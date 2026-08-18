<template>
  <div class="login-container">
    <el-card class="login-card" shadow="always">
      <template #header>
        <div class="login-header">
          <el-icon :size="32" color="#409EFF"><Monitor /></el-icon>
          <h2 class="login-title">TSLOMS 交通信号灯运维系统</h2>
        </div>
      </template>

      <el-form
        ref="loginFormRef"
        :model="loginForm"
        :rules="rules"
        label-width="0"
        @submit.prevent="handleLogin"
      >
        <el-form-item prop="username">
          <el-input
            v-model="loginForm.username"
            placeholder="请输入账号（用户名或手机号）"
            size="large"
            :prefix-icon="User"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="请输入密码"
            size="large"
            show-password
            :prefix-icon="Lock"
          />
        </el-form-item>
        <!-- 算术验证码：2+8=? 答 10 通过（替代短信/图形验证码，参考项目 a） -->
        <el-form-item prop="captcha_code">
          <div class="captcha-row">
            <el-input
              v-model="loginForm.captcha_code"
              placeholder="输入算式答案"
              size="large"
              :prefix-icon="Key"
              @keyup.enter="handleLogin"
            />
            <div class="captcha-box" @click="loadCaptcha" :title="'点击刷新验证'">
              <span>{{ captchaQuestion || '加载中...' }}</span>
            </div>
          </div>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            style="width: 100%"
            @click="handleLogin"
          >
            登 录
          </el-button>
        </el-form-item>
        <div class="login-register">
          <el-button type="primary" link @click="goRegister">还没有账号？立即注册</el-button>
        </div>
      </el-form>

      <!-- 自助注册已独立为 /register 页面，不再使用登录页弹窗 -->
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { User, Lock, Key } from '@element-plus/icons-vue'
import { useAuthStore } from '@/store/auth'
import { getCaptcha } from '@/api/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const loginFormRef = ref<FormInstance>()
const loading = ref(false)
const captchaQuestion = ref('')
let captchaUUID = ''

const loginForm = reactive({
  username: '',
  password: '',
  captcha_code: '',
})

const rules: FormRules = {
  username: [{ required: true, message: '请输入账号（用户名或手机号）', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
  captcha_code: [{ required: true, message: '请输入算式答案', trigger: 'blur' }],
}

// ---- 自助注册：独立 /register 页面（原登录页内弹窗已改） ----
function goRegister() {
  router.push('/register')
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

async function handleLogin() {
  if (!loginFormRef.value) return
  await loginFormRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      await authStore.login({
        username: loginForm.username,
        password: loginForm.password,
        captcha_uuid: captchaUUID,
        captcha_code: loginForm.captcha_code,
      })
      ElMessage.success('登录成功')
      const redirect = (route.query.redirect as string) || '/dashboard'
      router.push(redirect)
    } catch {
      // 验证码错误等已由拦截器提示；失败后刷新算术题
      loadCaptcha()
    } finally {
      loading.value = false
    }
  })
}

onMounted(loadCaptcha)
</script>

<style scoped>
.login-container {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1a2a6c, #2a4858);
}
.login-card {
  width: 420px;
  border-radius: 8px;
}
.login-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
.login-title {
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
.login-register {
  text-align: center;
  margin-top: 8px;
}
.reg-note {
  font-size: 12px;
  color: #909399;
  line-height: 1.5;
}
</style>
