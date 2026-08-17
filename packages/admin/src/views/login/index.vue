<template>
  <div class="login-container">
    <el-card class="login-card" shadow="always">
      <template #header>
        <div class="login-header">
          <el-icon :size="32" color="#409EFF"><Monitor /></el-icon>
          <h2 class="login-title">TSLOMS 交通信号灯运维系统</h2>
        </div>
      </template>

      <el-tabs v-model="activeTab" class="login-tabs">
        <!-- 账号密码登录（原通道，向后兼容） -->
        <el-tab-pane label="账号密码" name="password">
          <el-form
            ref="pwFormRef"
            :model="pwForm"
            :rules="pwRules"
            label-width="0"
            @submit.prevent="handlePwLogin"
          >
            <el-form-item prop="username">
              <el-input v-model="pwForm.username" placeholder="请输入用户名" size="large" :prefix-icon="User" />
            </el-form-item>
            <el-form-item prop="password">
              <el-input
                v-model="pwForm.password"
                type="password"
                placeholder="请输入密码"
                size="large"
                show-password
                :prefix-icon="Lock"
                @keyup.enter="handlePwLogin"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="large" :loading="loading" style="width: 100%" @click="handlePwLogin">
                登 录
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 手机号验证码登录 -->
        <el-tab-pane label="手机号验证码" name="phone">
          <el-form ref="phoneFormRef" :model="phoneForm" :rules="phoneRules" label-width="0">
            <el-form-item prop="phone">
              <el-input v-model="phoneForm.phone" placeholder="请输入手机号" size="large" :prefix-icon="Iphone" maxlength="11" />
            </el-form-item>
            <el-form-item prop="code">
              <div class="code-row">
                <el-input v-model="phoneForm.code" placeholder="请输入验证码" size="large" maxlength="6" :prefix-icon="Key" />
                <el-button size="large" :disabled="countdown > 0 || !isPhone" @click="handleSendCode">
                  {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
                </el-button>
              </div>
              <div class="sms-tip">开发环境：验证码会打印在后端日志（Console 通道）；生产需配置短信服务商</div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="large" :loading="loading" style="width: 100%" @click="handlePhoneLogin">
                登 录
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { User, Lock, Iphone, Key } from '@element-plus/icons-vue'
import { useAuthStore } from '@/store/auth'
import { sendSmsCode } from '@/api/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const activeTab = ref('password')
const loading = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

// ---------- 账号密码表单 ----------
const pwFormRef = ref<FormInstance>()
const pwForm = reactive({ username: '', password: '' })
const pwRules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

// ---------- 手机号验证码表单 ----------
const phoneFormRef = ref<FormInstance>()
const phoneForm = reactive({ phone: '', code: '' })
const isPhone = computed(() => /^1\d{10}$/.test(phoneForm.phone))
const phoneRules: FormRules = {
  phone: [{ pattern: /^1\d{10}$/, message: '请输入 11 位手机号', trigger: 'blur' }],
  code: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
}

function startCountdown() {
  countdown.value = 60
  timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0 && timer) {
      clearInterval(timer)
      timer = null
    }
  }, 1000)
}

async function handleSendCode() {
  if (!isPhone.value) {
    ElMessage.warning('请输入正确的手机号')
    return
  }
  try {
    await sendSmsCode(phoneForm.phone)
    ElMessage.success('验证码已发送（开发环境见后端日志）')
    startCountdown()
  } catch {
    // 错误已由拦截器提示
  }
}

function afterLogin() {
  ElMessage.success('登录成功')
  const redirect = (route.query.redirect as string) || '/dashboard'
  router.push(redirect)
}

async function handlePwLogin() {
  if (!pwFormRef.value) return
  await pwFormRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      await authStore.login(pwForm.username, pwForm.password)
      afterLogin()
    } catch {
      // 拦截器已提示
    } finally {
      loading.value = false
    }
  })
}

async function handlePhoneLogin() {
  if (!phoneFormRef.value) return
  await phoneFormRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      await authStore.loginWithPhone(phoneForm.phone, phoneForm.code)
      afterLogin()
    } catch {
      // 拦截器已提示
    } finally {
      loading.value = false
    }
  })
}

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
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
.code-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
.sms-tip {
  margin-top: 6px;
  font-size: 12px;
  color: #909399;
}
</style>
