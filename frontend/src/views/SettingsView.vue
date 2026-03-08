<script setup lang="ts">
import { computed } from 'vue'
import {
  ElButton,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
  ElMessage,
  ElOption,
  ElSelect,
  ElSwitch,
} from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAccountsStore } from '@/stores/accounts'
import { useSettingsStore } from '@/stores/settings'
import { formatDateTime } from '@/utils/format'
import { toErrorMessage } from '@/utils/errors'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const accountsStore = useAccountsStore()

const connectionCopy = computed(() => {
  if (!settingsStore.connection) {
    return t('settings.notTestedYet')
  }
  return t('settings.connectionSummary', {
    message: settingsStore.connection.message,
    count: settingsStore.connection.accountCount,
    checkedAt: formatDateTime(settingsStore.connection.checkedAt),
  })
})

async function testOnly() {
  try {
    const result = await settingsStore.testConnection()
    ElMessage.success(t('settings.testReachable', { message: result.message, count: result.accountCount }))
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

async function testAndSave() {
  try {
    const result = await settingsStore.testAndSave()
    await accountsStore.refreshAll()
    ElMessage.success(t('settings.savedReachable', { count: result.accountCount }))
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

async function changeLocale(locale: string) {
  try {
    await settingsStore.saveLocalePreference(locale)
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}
</script>

<template>
  <div class="view-shell view-shell--settings">
    <section class="panel settings-panel panel--scroll">
      <div class="panel-head">
        <div>
          <p class="panel-kicker">{{ t('settings.connectionProfile') }}</p>
          <h3>{{ t('settings.savedTarget') }}</h3>
        </div>
        <span class="muted">{{ connectionCopy }}</span>
      </div>

      <el-form label-position="top" class="settings-form">
        <div class="settings-grid">
          <el-form-item :label="t('settings.language')">
            <el-select :model-value="settingsStore.currentLocale" @change="changeLocale">
              <el-option :label="t('topbar.english')" value="en-US" />
              <el-option :label="t('topbar.chinese')" value="zh-CN" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('settings.baseUrl')" :error="settingsStore.errors.baseUrl">
            <el-input v-model="settingsStore.settings.baseUrl" :placeholder="t('settings.baseUrlPlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('settings.managementToken')" :error="settingsStore.errors.managementToken">
            <el-input v-model="settingsStore.settings.managementToken" type="password" show-password :placeholder="t('settings.tokenPlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('settings.targetType')">
            <el-input v-model="settingsStore.settings.targetType" />
          </el-form-item>
          <el-form-item :label="t('settings.provider')">
            <el-input v-model="settingsStore.settings.provider" :placeholder="t('settings.providerPlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('settings.probeWorkers')" :error="settingsStore.errors.probeWorkers">
            <el-input-number v-model="settingsStore.settings.probeWorkers" :min="1" :max="200" />
          </el-form-item>
          <el-form-item :label="t('settings.actionWorkers')" :error="settingsStore.errors.actionWorkers">
            <el-input-number v-model="settingsStore.settings.actionWorkers" :min="1" :max="100" />
          </el-form-item>
          <el-form-item :label="t('settings.timeoutSeconds')" :error="settingsStore.errors.timeoutSeconds">
            <el-input-number v-model="settingsStore.settings.timeoutSeconds" :min="1" :max="120" />
          </el-form-item>
          <el-form-item :label="t('settings.retries')" :error="settingsStore.errors.retries">
            <el-input-number v-model="settingsStore.settings.retries" :min="0" :max="10" />
          </el-form-item>
          <el-form-item :label="t('settings.quotaAction')" :error="settingsStore.errors.quotaAction">
            <el-select v-model="settingsStore.settings.quotaAction">
              <el-option :label="t('quotaActions.disable')" value="disable" />
              <el-option :label="t('quotaActions.delete')" value="delete" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('settings.exportDirectory')">
            <el-input v-model="settingsStore.settings.exportDirectory" />
          </el-form-item>
          <el-form-item :label="t('settings.userAgent')" class="span-2">
            <el-input v-model="settingsStore.settings.userAgent" />
          </el-form-item>
        </div>

        <p class="muted">{{ t('settings.languageHint') }}</p>

        <div class="settings-toggles">
          <el-switch v-model="settingsStore.settings.delete401" :active-text="t('settings.delete401')" />
          <el-switch v-model="settingsStore.settings.autoReenable" :active-text="t('settings.autoReenable')" />
          <el-switch v-model="settingsStore.settings.detailedLogs" :active-text="t('settings.detailedLogs')" />
        </div>

        <div class="hero-actions">
          <el-button plain @click="testOnly">{{ t('settings.testConnection') }}</el-button>
          <el-button type="primary" :loading="settingsStore.saving" @click="testAndSave">
            {{ t('settings.testAndSave') }}
          </el-button>
        </div>
      </el-form>
    </section>
  </div>
</template>
