<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ApiBusinessError, ApiNetworkError } from '../api/client'
import { getRegistrationStatus } from '../api/auth'
import AuthLayout from '../layouts/AuthLayout.vue'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()

const form = reactive({
  email: '',
})

const checkingStatus = ref(true)
const isRegistrationEnabled = ref(false)
const isSubmitting = ref(false)
const errorMessage = ref('')
const statusErrorMessage = ref('')
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

function resolveRegisterErrorMessage(error: unknown) {
  if (error instanceof ApiBusinessError) {
    if (error.code === 40901) {
      return '该邮箱已注册'
    }

    return error.message || '注册失败，请稍后重试'
  }

  if (error instanceof ApiNetworkError) {
    return error.message
  }

  return '注册失败，请稍后重试'
}

async function loadRegistrationStatus() {
  checkingStatus.value = true
  statusErrorMessage.value = ''

  try {
    const response = await getRegistrationStatus()
    isRegistrationEnabled.value = response.enabled
  } catch (error) {
    if (error instanceof ApiNetworkError && error.status === 404) {
      isRegistrationEnabled.value = true
      return
    }

    isRegistrationEnabled.value = false
    statusErrorMessage.value = resolveRegisterErrorMessage(error)
  } finally {
    checkingStatus.value = false
  }
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
    await authStore.register(form.email.trim())
    successMessage.value = '注册成功，请查收邮件获取密码'
    form.email = ''
  } catch (error) {
    errorMessage.value = resolveRegisterErrorMessage(error)
  } finally {
    isSubmitting.value = false
  }
}

onMounted(() => {
  void loadRegistrationStatus()
})
</script>

<template>
  <AuthLayout
    eyebrow="Open Registration"
    title="申请控制台访问"
    description="注册后系统会通过邮件发送初始密码。首位注册用户将自动获得管理员权限。"
  >
    <div v-if="checkingStatus" class="rounded-[28px] border border-outline-subtle bg-surface-raised px-5 py-8 text-center text-sm text-content-secondary">
      正在检查注册状态...
    </div>

    <div v-else-if="successMessage" class="space-y-5 text-center">
      <div class="rounded-[28px] border border-success-500/30 bg-success-500/10 px-5 py-8">
        <p class="text-lg font-semibold text-content-primary">{{ successMessage }}</p>
        <p class="mt-3 text-sm leading-7 text-content-secondary">
          请前往邮箱查收系统生成的密码，再返回登录页继续操作。
        </p>
      </div>

      <RouterLink
        to="/login"
        class="inline-flex w-full items-center justify-center rounded-2xl border border-outline bg-surface-base px-4 py-3 text-sm font-semibold text-content-primary transition hover:border-primary-500 hover:text-primary-600"
      >
        返回登录页
      </RouterLink>
    </div>

    <div v-else-if="statusErrorMessage" class="space-y-5">
      <div class="rounded-[28px] border border-error-500/30 bg-error-500/10 px-5 py-6 text-sm text-error-500">
        {{ statusErrorMessage }}
      </div>

      <button
        type="button"
        class="inline-flex w-full items-center justify-center rounded-2xl border border-outline bg-surface-base px-4 py-3 text-sm font-semibold text-content-primary transition hover:border-primary-500 hover:text-primary-600"
        @click="loadRegistrationStatus"
      >
        重新检查
      </button>
    </div>

    <div v-else-if="!isRegistrationEnabled" class="space-y-5 text-center">
      <div class="rounded-[28px] border border-warning-500/30 bg-warning-500/10 px-5 py-8">
        <p class="text-lg font-semibold text-content-primary">注册已关闭</p>
        <p class="mt-3 text-sm leading-7 text-content-secondary">
          当前仅允许已分配账号的成员登录。需要访问权限时，请联系管理员开通注册。
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
          <label class="text-sm font-medium text-content-primary" for="register-email">邮箱</label>
          <input
            id="register-email"
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
          {{ isSubmitting ? '提交中...' : '注册' }}
        </button>
      </form>

      <p class="mt-6 text-center text-sm text-content-secondary">
        已有账号？
        <RouterLink to="/login" class="font-semibold text-primary-600 transition hover:text-primary-500">
          去登录
        </RouterLink>
      </p>
    </template>
  </AuthLayout>
</template>