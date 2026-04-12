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
    <AppCard title="个人信息" class="profile-page__card profile-page__card--profile">
      <div class="profile-page__hero">
        <div class="profile-page__avatar">
          {{ (user?.name ?? user?.email ?? '?').slice(0, 1).toUpperCase() }}
        </div>
        <div class="profile-page__hero-content">
          <p class="profile-page__eyebrow">账户概览</p>
          <h2 class="profile-page__hero-name">{{ user?.name ?? '未设置名称' }}</h2>
          <p class="profile-page__hero-email">{{ user?.email ?? '-' }}</p>
        </div>
        <span class="profile-page__role-pill">{{ user?.role === 'admin' ? '管理员' : '普通用户' }}</span>
      </div>

      <div class="profile-page__info-grid">
        <div class="profile-page__info-card">
          <span class="profile-page__label">登录邮箱</span>
          <span class="profile-page__value">{{ user?.email ?? '-' }}</span>
        </div>
        <div class="profile-page__info-card">
          <span class="profile-page__label">当前角色</span>
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
    <AppCard title="修改密码" class="profile-page__card profile-page__card--password">
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
    <AppCard title="通知订阅" class="profile-page__card profile-page__card--subscriptions">
      <p class="profile-page__sub-desc">开启后，当对应实例发生风险事件时将通过邮件通知您</p>
      <NotificationSubscriptions />
    </AppCard>
  </div>
</template>

<style scoped>
.profile-page {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(320px, 0.95fr);
  gap: 24px;
  max-width: 1120px;
  align-items: start;
}

.profile-page__card {
  min-width: 0;
}

.profile-page__card--subscriptions {
  grid-column: 2;
  grid-row: 1 / span 2;
}

.profile-page__hero {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 18px;
  border-radius: var(--radius-lg);
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--primary-500) 14%, white) 0%, transparent 58%),
    linear-gradient(180deg, var(--surface-sunken) 0%, transparent 100%);
  border: 1px solid color-mix(in srgb, var(--primary-500) 16%, var(--border-subtle));
  margin-bottom: 20px;
}

.profile-page__avatar {
  width: 52px;
  height: 52px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 16px;
  font-size: 22px;
  font-weight: 700;
  color: #0b3b4a;
  background: linear-gradient(135deg, var(--primary-300) 0%, var(--accent-mint-400) 100%);
  box-shadow: var(--shadow-sm);
  flex-shrink: 0;
}

.profile-page__hero-content {
  flex: 1;
  min-width: 0;
}

.profile-page__eyebrow {
  margin: 0 0 6px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.profile-page__hero-name {
  margin: 0;
  font-size: 24px;
  line-height: 1.2;
  color: var(--text-primary);
}

.profile-page__hero-email {
  margin: 6px 0 0;
  font-size: 14px;
  color: var(--text-secondary);
  word-break: break-all;
}

.profile-page__role-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 8px 12px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--primary-500) 12%, var(--surface-raised));
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.profile-page__info-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 20px;
}

.profile-page__info-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  background: color-mix(in srgb, var(--surface-sunken) 68%, transparent);
}

.profile-page__label {
  font-size: 12px;
  color: var(--text-muted);
}

.profile-page__value {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  word-break: break-word;
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
  margin: 0 0 16px;
  font-size: 13px;
  color: var(--text-muted);
}

@media (max-width: 1080px) {
  .profile-page {
    grid-template-columns: 1fr;
    max-width: 760px;
  }

  .profile-page__card--subscriptions {
    grid-column: auto;
    grid-row: auto;
  }
}

@media (max-width: 640px) {
  .profile-page__hero {
    flex-wrap: wrap;
  }

  .profile-page__role-pill {
    width: 100%;
    justify-content: flex-start;
  }

  .profile-page__info-grid {
    grid-template-columns: 1fr;
  }

  .profile-page__name-row {
    flex-direction: column;
  }
}
</style>
