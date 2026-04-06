<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ApiBusinessError, ApiNetworkError } from '../api/client'
import AuthLayout from '../layouts/AuthLayout.vue'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const router = useRouter()

const form = reactive({
  email: '',
  password: '',
})

const isSubmitting = ref(false)
const errorMessage = ref('')

const inputClass = 'mt-2 w-full rounded-2xl border border-outline bg-surface-base px-4 py-3 text-sm text-content-primary outline-none transition placeholder:text-content-muted focus:border-primary-500 focus:ring-4 focus:ring-primary-500/10'

function validateForm() {
  const normalizedEmail = form.email.trim()
  if (!normalizedEmail) {
    return '请输入邮箱地址'
  }

  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(normalizedEmail)) {
    return '请输入有效的邮箱地址'
  }

  if (!form.password.trim()) {
    return '请输入密码'
  }

  return ''
}

function resolveLoginErrorMessage(error: unknown) {
  if (error instanceof ApiBusinessError) {
    if (error.code === 40101) {
      return '邮箱或密码错误'
    }

    if (error.code === 42901) {
      return '账号已锁定，请稍后再试'
    }

    return error.message || '登录失败，请稍后重试'
  }

  if (error instanceof ApiNetworkError) {
    return error.message
  }

  return '登录失败，请稍后重试'
}

async function handleSubmit() {
  errorMessage.value = ''

  const validationMessage = validateForm()
  if (validationMessage) {
    errorMessage.value = validationMessage
    return
  }

  isSubmitting.value = true

  try {
    await authStore.login(form.email.trim(), form.password)
    await router.replace(authStore.defaultRoute)
  } catch (error) {
    errorMessage.value = resolveLoginErrorMessage(error)
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <AuthLayout
    eyebrow="Operator Login"
    title="登录控制台"
    description="使用已分配的邮箱与密码进入备份控制台，查看实例、任务和恢复操作。"
  >
    <form class="space-y-5" @submit.prevent="handleSubmit">
      <div>
        <label class="text-sm font-medium text-content-primary" for="login-email">邮箱</label>
        <input
          id="login-email"
          v-model="form.email"
          type="email"
          name="email"
          autocomplete="email"
          placeholder="name@example.com"
          :class="inputClass"
        >
      </div>

      <div>
        <label class="text-sm font-medium text-content-primary" for="login-password">密码</label>
        <input
          id="login-password"
          v-model="form.password"
          type="password"
          name="password"
          autocomplete="current-password"
          placeholder="输入密码"
          :class="inputClass"
        >
      </div>

      <p v-if="errorMessage" class="rounded-2xl border border-error-500/30 bg-error-500/10 px-4 py-3 text-sm text-error-500">
        {{ errorMessage }}
      </p>

      <button
        type="submit"
        class="inline-flex w-full items-center justify-center rounded-2xl bg-[linear-gradient(135deg,var(--primary-500),#7EF2D4)] px-4 py-3 text-sm font-semibold text-slate-950 transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-60"
        :disabled="isSubmitting"
      >
        {{ isSubmitting ? '登录中...' : '登录' }}
      </button>
    </form>

    <p class="mt-6 text-center text-sm text-content-secondary">
      还没有账号？
      <RouterLink to="/register" class="font-semibold text-primary-600 transition hover:text-primary-500">
        去注册
      </RouterLink>
    </p>
  </AuthLayout>
</template>