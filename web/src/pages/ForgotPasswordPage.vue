<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ApiBusinessError, ApiNetworkError } from '../api/client'
import { forgotPassword } from '../api/auth'
import AuthLayout from '../layouts/AuthLayout.vue'

const form = reactive({
  email: '',
})

const isSubmitting = ref(false)
const errorMessage = ref('')
const successMessage = ref('')

const inputClass = 'mt-2 w-full rounded-2xl border border-outline bg-surface-base px-4 py-3 text-sm text-content-primary outline-none transition placeholder:text-content-muted focus:border-primary-500 focus:ring-4 focus:ring-primary-500/10'

function validateEmail() {
  const normalizedEmail = form.email.trim()
  if (!normalizedEmail) {
    return '请输入邮箱地址'
  }

  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(normalizedEmail)) {
    return '请输入有效的邮箱地址'
  }

  return ''
}

function resolveForgotPasswordErrorMessage(error: unknown) {
  if (error instanceof ApiBusinessError) {
    return error.message || '重置失败，请稍后重试'
  }

  if (error instanceof ApiNetworkError) {
    return error.message
  }

  return '重置失败，请稍后重试'
}

async function handleSubmit() {
  errorMessage.value = ''

  const validationMessage = validateEmail()
  if (validationMessage) {
    errorMessage.value = validationMessage
    return
  }

  isSubmitting.value = true

  try {
    const response = await forgotPassword(form.email.trim())
    successMessage.value = response.message
    form.email = ''
  } catch (error) {
    errorMessage.value = resolveForgotPasswordErrorMessage(error)
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <AuthLayout
    eyebrow="Password Recovery"
    title="重置登录密码"
    description="输入注册邮箱后，系统会在需要时重新生成登录密码并发送到邮箱。"
  >
    <div v-if="successMessage" class="space-y-5 text-center">
      <div class="rounded-[28px] border border-success-500/30 bg-success-500/10 px-5 py-8">
        <p class="text-lg font-semibold text-content-primary">{{ successMessage }}</p>
        <p class="mt-3 text-sm leading-7 text-content-secondary">
          请前往邮箱查收系统发送的登录密码，再返回登录页继续操作。
        </p>
      </div>

      <RouterLink
        to="/login"
        class="inline-flex w-full items-center justify-center rounded-2xl border border-outline bg-surface-base px-4 py-3 text-sm font-semibold text-content-primary transition hover:border-primary-500 hover:text-primary-600"
      >
        返回登录页
      </RouterLink>
    </div>

    <template v-else>
      <form class="space-y-5" @submit.prevent="handleSubmit">
        <div>
          <label class="text-sm font-medium text-content-primary" for="forgot-password-email">邮箱</label>
          <input
            id="forgot-password-email"
            v-model="form.email"
            type="email"
            name="email"
            autocomplete="email"
            placeholder="name@example.com"
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
          {{ isSubmitting ? '提交中...' : '发送重置密码邮件' }}
        </button>
      </form>

      <p class="mt-6 text-center text-sm text-content-secondary">
        想起密码了？
        <RouterLink to="/login" class="font-semibold text-primary-600 transition hover:text-primary-500">
          返回登录
        </RouterLink>
      </p>
    </template>
  </AuthLayout>
</template>
