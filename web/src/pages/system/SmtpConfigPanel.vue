<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { getSmtpConfig, updateSmtpConfig, testSmtp } from '../../api/system'
import type { SmtpConfig } from '../../api/system'
import { useToastStore } from '../../stores/toast'
import { ApiBusinessError } from '../../api/client'
import AppFormGroup from '../../components/AppFormGroup.vue'
import AppFormItem from '../../components/AppFormItem.vue'
import AppInput from '../../components/AppInput.vue'
import AppButton from '../../components/AppButton.vue'
import AppSelect from '../../components/AppSelect.vue'

const toast = useToastStore()

const encryptionOptions = [
  { label: '无加密', value: 'none' },
  { label: 'STARTTLS', value: 'starttls' },
  { label: 'SSL/TLS', value: 'ssltls' },
]

// ── SMTP form state ──
const loading = ref(false)
const saving = ref(false)

const form = reactive({
  host: '',
  port: 587,
  username: '',
  password: '',
  from: '',
  encryption: 'none',
})

const errors = reactive({
  host: '',
  port: '',
  from: '',
})

const passwordMasked = ref(false)

// ── Test send state ──
const testEmail = ref('')
const testEmailError = ref('')
const testing = ref(false)



// ── Load SMTP config ──
async function fetchConfig() {
  loading.value = true
  try {
    const config = await getSmtpConfig()
    form.host = config.host
    form.port = config.port || 587
    form.username = config.username
    form.password = config.password
    form.from = config.from
    form.encryption = config.encryption || 'none'
    passwordMasked.value = config.password === '***'
  } catch {
    // Config may not exist yet, keep defaults
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchConfig()
})

// ── Validation ──
function validateForm(): boolean {
  let valid = true
  errors.host = ''
  errors.port = ''
  errors.from = ''

  if (!form.host.trim()) {
    errors.host = 'SMTP 服务器地址不能为空'
    valid = false
  }
  if (!form.port || form.port < 1 || form.port > 65535) {
    errors.port = '端口范围 1-65535'
    valid = false
  }
  if (!form.from.trim()) {
    errors.from = '发件人邮箱不能为空'
    valid = false
  }
  return valid
}

// ── Save SMTP config ──
async function handleSave() {
  if (!validateForm()) return

  saving.value = true
  try {
    const payload: SmtpConfig = {
      host: form.host.trim(),
      port: form.port,
      username: form.username.trim(),
      password: passwordMasked.value ? '***' : form.password,
      from: form.from.trim(),
      encryption: form.encryption,
    }
    const res = await updateSmtpConfig(payload)
    form.password = res.password
    passwordMasked.value = res.password === '***'
    toast.success('SMTP 配置已保存')
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error('保存失败：' + e.message)
    } else if (e instanceof Error) {
      toast.error('保存失败：' + e.message)
    } else {
      toast.error('保存 SMTP 配置失败')
    }
  } finally {
    saving.value = false
  }
}

// ── Password field focus ──
function onPasswordFocus() {
  if (passwordMasked.value) {
    form.password = ''
    passwordMasked.value = false
  }
}

// ── Test SMTP ──
function validateTestEmail(): boolean {
  testEmailError.value = ''
  if (!testEmail.value.trim()) {
    testEmailError.value = '请输入收件人邮箱'
    return false
  }
  return true
}

async function handleTest() {
  if (!validateTestEmail()) return

  testing.value = true
  try {
    await testSmtp(testEmail.value.trim())
    toast.success('测试邮件已发送')
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error('测试发送失败：' + e.message)
    } else if (e instanceof Error) {
      toast.error('测试发送失败：' + e.message)
    } else {
      toast.error('测试发送失败')
    }
  } finally {
    testing.value = false
  }
}


</script>

<template>
  <div class="smtp-panel">
    <!-- SMTP Configuration Card -->
    <div class="smtp-card">
      <div class="smtp-card__header">
        <h3 class="smtp-card__title">SMTP 邮件服务器</h3>
        <p class="smtp-card__desc">配置 SMTP 服务器，用于发送备份通知和告警邮件。</p>
      </div>
      <div class="smtp-card__body">
        <AppFormGroup>
          <div class="smtp-form-grid">
            <AppFormItem label="SMTP 服务器地址" required :error="errors.host">
              <AppInput
                v-model="form.host"
                placeholder="例如 smtp.example.com"
                :disabled="loading"
              />
            </AppFormItem>

            <div class="smtp-form-row">
              <AppFormItem label="端口" required :error="errors.port">
                <AppInput
                  v-model="form.port"
                  type="number"
                  placeholder="587"
                  :disabled="loading"
                />
              </AppFormItem>

              <AppFormItem label="加密方式">
                <AppSelect
                  v-model="form.encryption"
                  :options="encryptionOptions"
                  :disabled="loading"
                />
              </AppFormItem>
            </div>

            <div class="smtp-form-row">
              <AppFormItem label="用户名">
                <AppInput
                  v-model="form.username"
                  placeholder="SMTP 用户名"
                  :disabled="loading"
                />
              </AppFormItem>

              <AppFormItem label="密码">
                <AppInput
                  v-model="form.password"
                  type="password"
                  placeholder="SMTP 密码"
                  :disabled="loading"
                  @focus="onPasswordFocus"
                />
              </AppFormItem>
            </div>

            <AppFormItem label="发件人邮箱" required :error="errors.from">
              <AppInput
                v-model="form.from"
                type="email"
                placeholder="例如 noreply@example.com"
                :disabled="loading"
              />
            </AppFormItem>
          </div>
        </AppFormGroup>
      </div>
      <div class="smtp-card__footer">
        <AppButton :loading="saving" @click="handleSave">保存配置</AppButton>
      </div>
    </div>

    <!-- Test Send -->
    <div class="smtp-card">
      <div class="smtp-card__header">
        <h3 class="smtp-card__title">测试发送</h3>
        <p class="smtp-card__desc">向指定邮箱发送一封测试邮件以验证配置是否正确。</p>
      </div>
      <div class="smtp-card__body">
        <div class="smtp-panel__test-row">
          <div class="smtp-panel__test-input">
            <AppFormItem :error="testEmailError">
              <AppInput
                v-model="testEmail"
                type="email"
                placeholder="收件人邮箱"
              />
            </AppFormItem>
          </div>
          <AppButton variant="outline" :loading="testing" @click="handleTest">
            发送测试邮件
          </AppButton>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.smtp-panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.smtp-card {
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
}

.smtp-card__header {
  padding: 20px 24px 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.smtp-card__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.smtp-card__desc {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}

.smtp-card__body {
  padding: 16px 24px 20px;
}

.smtp-card__footer {
  padding: 0 24px 20px;
  display: flex;
}

.smtp-form-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.smtp-form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

@media (max-width: 640px) {
  .smtp-form-row {
    grid-template-columns: 1fr;
  }
}

.smtp-panel__test-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.smtp-panel__test-input {
  flex: 1;
}


</style>
