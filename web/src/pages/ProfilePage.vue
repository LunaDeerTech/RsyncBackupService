<script setup lang="ts">
import { ref, reactive } from 'vue'
import { storeToRefs } from 'pinia'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import { updateCurrentUserProfile, updateCurrentUserPassword } from '../api/users'
import { ApiBusinessError } from '../api/client'
import AppCard from '../components/AppCard.vue'
import AppFormGroup from '../components/AppFormGroup.vue'
import AppFormItem from '../components/AppFormItem.vue'
import AppInput from '../components/AppInput.vue'
import AppButton from '../components/AppButton.vue'
import NotificationSubscriptions from '../components/NotificationSubscriptions.vue'

const authStore = useAuthStore()
const toast = useToastStore()
const { user } = storeToRefs(authStore)

// ── Profile edit ──
const profileName = ref(user.value?.name ?? '')
const profileSaving = ref(false)

async function handleSaveProfile() {
  profileSaving.value = true
  try {
    const res = await updateCurrentUserProfile({ name: profileName.value.trim() })
    if (authStore.user) {
      authStore.user.name = res.name
    }
    toast.success('名称已更新')
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('保存失败')
    }
  } finally {
    profileSaving.value = false
  }
}

// ── Password change ──
const pwForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: '',
})

const pwErrors = reactive({
  old_password: '',
  new_password: '',
  confirm_password: '',
})

const pwSaving = ref(false)

function validatePassword(): boolean {
  let valid = true
  pwErrors.old_password = ''
  pwErrors.new_password = ''
  pwErrors.confirm_password = ''

  if (!pwForm.old_password) {
    pwErrors.old_password = '请输入旧密码'
    valid = false
  }
  if (!pwForm.new_password) {
    pwErrors.new_password = '请输入新密码'
    valid = false
  } else if (pwForm.new_password.length < 8) {
    pwErrors.new_password = '新密码长度不能少于 8 位'
    valid = false
  }
  if (!pwForm.confirm_password) {
    pwErrors.confirm_password = '请确认新密码'
    valid = false
  } else if (pwForm.new_password !== pwForm.confirm_password) {
    pwErrors.confirm_password = '两次输入的密码不一致'
    valid = false
  }

  return valid
}

async function handleChangePassword() {
  if (!validatePassword()) return

  pwSaving.value = true
  try {
    await updateCurrentUserPassword({
      old_password: pwForm.old_password,
      new_password: pwForm.new_password,
    })
    toast.success('密码已修改')
    pwForm.old_password = ''
    pwForm.new_password = ''
    pwForm.confirm_password = ''
    pwErrors.old_password = ''
    pwErrors.new_password = ''
    pwErrors.confirm_password = ''
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('修改密码失败')
    }
  } finally {
    pwSaving.value = false
  }
}
</script>

<template>
  <div class="profile-page">
    <!-- Personal info -->
    <AppCard title="个人信息">
      <div class="profile-page__info">
        <div class="profile-page__row">
          <span class="profile-page__label">邮箱</span>
          <span class="profile-page__value">{{ user?.email ?? '-' }}</span>
        </div>
        <div class="profile-page__row">
          <span class="profile-page__label">角色</span>
          <span class="profile-page__value">{{ user?.role === 'admin' ? '管理员' : '普通用户' }}</span>
        </div>
      </div>
      <div class="profile-page__name-form">
        <AppFormGroup>
          <AppFormItem label="名称">
            <div class="profile-page__name-row">
              <AppInput v-model="profileName" placeholder="输入您的名称" />
              <AppButton variant="primary" size="sm" :loading="profileSaving" @click="handleSaveProfile">
                保存
              </AppButton>
            </div>
          </AppFormItem>
        </AppFormGroup>
      </div>
    </AppCard>

    <!-- Password change -->
    <AppCard title="修改密码">
      <form @submit.prevent="handleChangePassword">
        <AppFormGroup>
          <AppFormItem label="旧密码" :required="true" :error="pwErrors.old_password">
            <AppInput v-model="pwForm.old_password" type="password" placeholder="请输入旧密码" />
          </AppFormItem>
          <AppFormItem label="新密码" :required="true" :error="pwErrors.new_password">
            <AppInput v-model="pwForm.new_password" type="password" placeholder="请输入新密码（至少 8 位）" />
          </AppFormItem>
          <AppFormItem label="确认新密码" :required="true" :error="pwErrors.confirm_password">
            <AppInput v-model="pwForm.confirm_password" type="password" placeholder="再次输入新密码" />
          </AppFormItem>
        </AppFormGroup>
        <div class="profile-page__pw-submit">
          <AppButton variant="primary" size="md" :loading="pwSaving" @click="handleChangePassword">
            修改密码
          </AppButton>
        </div>
      </form>
    </AppCard>

    <!-- Notification subscriptions -->
    <AppCard title="通知订阅">
      <p class="profile-page__sub-desc">开启后，当对应实例发生风险事件时将通过邮件通知您</p>
      <NotificationSubscriptions />
    </AppCard>
  </div>
</template>

<style scoped>
.profile-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
  max-width: 640px;
}

.profile-page__info {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 20px;
}

.profile-page__row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.profile-page__label {
  font-size: 13px;
  color: var(--text-muted);
  width: 48px;
  flex-shrink: 0;
}

.profile-page__value {
  font-size: 14px;
  color: var(--text-primary);
}

.profile-page__name-form {
  border-top: 1px solid var(--border-subtle);
  padding-top: 16px;
}

.profile-page__name-row {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}

.profile-page__pw-submit {
  margin-top: 16px;
}

.profile-page__sub-desc {
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--text-muted);
}
</style>
