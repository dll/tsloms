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
          <el-button type="primary" link @click="openRegister">还没有账号？立即注册</el-button>
        </div>
      </el-form>

      <!-- 自助注册弹窗 -->
      <el-dialog v-model="regVisible" title="注册账号" width="420px">
        <el-form :model="regForm" label-width="80px">
          <el-form-item label="归属部门">
            <el-select v-model="regForm.department_id" placeholder="请选择归属部门（选填）" clearable style="width:100%">
              <el-option v-for="d in deptOptions" :key="d.id" :label="d.name" :value="d.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="用户名" required>
            <el-input v-model="regForm.username" placeholder="请输入登录用户名（2-20位）" />
          </el-form-item>
          <el-form-item label="手机号">
            <el-input v-model="regForm.phone" placeholder="选填，11 位手机号" />
          </el-form-item>
          <el-form-item label="姓名">
            <el-input v-model="regForm.real_name" placeholder="选填" />
          </el-form-item>
          <el-form-item label="密码" required>
            <el-input v-model="regForm.password" type="password" show-password placeholder="至少 6 位" />
          </el-form-item>
          <el-form-item label="确认密码" required>
            <el-input v-model="regForm.confirm" type="password" show-password placeholder="再次输入密码" />
          </el-form-item>
          <el-form-item label="验证码" required>
            <div style="display:flex; gap:8px; align-items:center">
              <el-input v-model="regForm.captcha_code" placeholder="输入算式答案" />
              <div class="captcha-box" @click="loadCaptcha"><span>{{ captchaQuestion || '刷新' }}</span></div>
            </div>
          </el-form-item>
          <el-form-item>
            <div class="reg-note">注册后默认以「查看人员（只读）」角色使用，管理员可在用户管理中提升。</div>
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="regVisible = false">取消</el-button>
          <el-button type="primary" :loading="regLoading" @click="handleRegister">注册</el-button>
        </template>
      </el-dialog>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { User, Lock, Key } from '@element-plus/icons-vue'
import { useAuthStore } from '@/store/auth'
import { getCaptcha, register } from '@/api/auth'
import { getDepartments } from '@/api/department'

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

// ---- 自助注册（参考项目 a：归属部门 + 账号 + 密码 + 确认密码 + 验证码；并含本项目姓名/手机号）----
const regVisible = ref(false)
const regLoading = ref(false)
const deptOptions = ref<any[]>([])
const regForm = reactive({ username: '', phone: '', real_name: '', password: '', confirm: '', captcha_code: '', department_id: null as number | null })
function openRegister() {
  Object.assign(regForm, { username: '', phone: '', real_name: '', password: '', confirm: '', captcha_code: '', department_id: null })
  regVisible.value = true
  loadCaptcha()
  loadDepts()
}
async function loadDepts() {
  try { deptOptions.value = (await getDepartments())?.data?.list || [] } catch { /* 忽略 */ }
}
async function handleRegister() {
  const u = regForm.username.trim()
  if (!u) { ElMessage.warning('请输入用户名'); return }
  if (u.length < 2 || u.length > 20) { ElMessage.warning('用户名长度 2-20 位'); return }
  if (regForm.password.length < 6) { ElMessage.warning('密码至少 6 位'); return }
  if (regForm.password !== regForm.confirm) { ElMessage.warning('两次密码不一致'); return }
  if (!regForm.captcha_code.trim()) { ElMessage.warning('请输入算式答案'); return }
  regLoading.value = true
  try {
    await register({
      username: u,
      password: regForm.password,
      phone: regForm.phone.trim() || undefined,
      real_name: regForm.real_name.trim() || undefined,
      department_id: regForm.department_id ?? undefined,
      captcha_uuid: captchaUUID,
      captcha_code: regForm.captcha_code,
    })
    ElMessage.success('注册成功，请登录')
    regVisible.value = false
    loginForm.username = u
  } catch { /* 后端提示 */ } finally { regLoading.value = false }
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
