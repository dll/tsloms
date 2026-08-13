import axios from 'axios'
import { ElMessage } from 'element-plus'

// 统一 API 响应格式
export interface ApiResponse<T = any> {
  code: number
  msg: string
  data: T
  list?: T[]
  total?: number
  page?: number
  page_size?: number
}

// 创建 axios 实例，统一配置
const request = axios.create({
  baseURL: '/tsloms/api/v1',
  timeout: 15000,
})

// 请求拦截器 - 自动注入 Bearer token
request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  },
)

// 响应拦截器 - 统一处理业务错误和鉴权失效
request.interceptors.response.use(
  (response) => {
    const res = response.data
    // 业务码非 0 视为错误
    if (res.code !== 0) {
      ElMessage.error(res.msg || '请求失败')
      return Promise.reject(new Error(res.msg || 'Error'))
    }
    return res
  },
  (error) => {
    // HTTP 401 - token 过期或未授权，跳转登录页
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      ElMessage.error('登录已过期，请重新登录')
      window.location.href = import.meta.env.BASE_URL + 'login'
      return Promise.reject(error)
    }
    const msg = error.response?.data?.msg || error.message || '网络错误'
    ElMessage.error(msg)
    return Promise.reject(error)
  },
)

export default request
