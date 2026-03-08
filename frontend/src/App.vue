<script lang="ts" setup>
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref } from 'vue'
import { ElConfigProvider, ElMessage, ElOption, ElSelect } from 'element-plus'
import type { Language } from 'element-plus/es/locale'
import en from 'element-plus/es/locale/lang/en'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import { WindowSetLightTheme } from '../wailsjs/runtime/runtime'
import { useAccountsStore } from '@/stores/accounts'
import { useSettingsStore } from '@/stores/settings'
import { useTasksStore } from '@/stores/tasks'
import type { ViewKey } from '@/types'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import { localeChinese } from '@/utils/locale'
import { toErrorMessage } from '@/utils/errors'

const { t } = useI18n()
const settingsStore = useSettingsStore()
const accountsStore = useAccountsStore()
const tasksStore = useTasksStore()

const DashboardView = defineAsyncComponent(() => import('@/views/DashboardView.vue'))
const AccountsView = defineAsyncComponent(() => import('@/views/AccountsView.vue'))
const LogsView = defineAsyncComponent(() => import('@/views/LogsView.vue'))
const SettingsView = defineAsyncComponent(() => import('@/views/SettingsView.vue'))

const activeView = ref<ViewKey>('dashboard')

const navItems = computed<Array<{ key: ViewKey; label: string; caption: string }>>(() => [
  { key: 'dashboard', label: t('nav.dashboard'), caption: t('nav.dashboardCaption') },
  { key: 'accounts', label: t('nav.accounts'), caption: t('nav.accountsCaption') },
  { key: 'logs', label: t('nav.logs'), caption: t('nav.logsCaption') },
  { key: 'settings', label: t('nav.settings'), caption: t('nav.settingsCaption') },
])

const activeComponent = computed(() => {
  switch (activeView.value) {
    case 'accounts':
      return AccountsView
    case 'logs':
      return LogsView
    case 'settings':
      return SettingsView
    default:
      return DashboardView
  }
})

const connectionLabel = computed(() => {
  if (!settingsStore.connection) {
    return t('topbar.configured')
  }
  return settingsStore.connection.ok ? t('topbar.connected') : t('topbar.attention')
})

const elementLocale = computed<Language>(() => (
  (settingsStore.currentLocale === localeChinese ? zhCn : en) as unknown as Language
))

const lastScanText = computed(() => (
  accountsStore.summary.lastScanAt
    ? formatDateTime(accountsStore.summary.lastScanAt)
    : t('topbar.noRecentScan')
))

async function changeLocale(locale: string) {
  try {
    await settingsStore.saveLocalePreference(locale)
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

onMounted(async () => {
  WindowSetLightTheme()
  await settingsStore.loadSettings()
  tasksStore.initEventBridge()
  await accountsStore.refreshAll()
})

onUnmounted(() => {
  tasksStore.destroyEventBridge()
})
</script>

<template>
  <el-config-provider :locale="elementLocale">
    <div class="app-shell">
      <aside class="app-sidebar">
        <div>
          <p class="sidebar-kicker">{{ t('app.name') }}</p>
          <h1>{{ t('app.headline') }}</h1>
          <p class="sidebar-copy">
            {{ t('app.copy') }}
          </p>
        </div>

        <nav class="nav-list">
          <button
            v-for="item in navItems"
            :key="item.key"
            class="nav-item"
            :class="{ 'nav-item--active': item.key === activeView }"
            @click="activeView = item.key"
          >
            <strong>{{ item.label }}</strong>
            <span>{{ item.caption }}</span>
          </button>
        </nav>
      </aside>

      <main class="app-main">
        <header class="topbar">
          <div class="topbar-status">
            <span class="status-dot" :data-tone="settingsStore.connectionTone" />
            <div>
              <strong>{{ connectionLabel }}</strong>
              <p>{{ settingsStore.settings.baseUrl || t('topbar.endpointHint') }}</p>
            </div>
          </div>
          <div class="topbar-meta">
            <span>{{ t('topbar.tracked', { count: accountsStore.summary.filteredAccounts }) }}</span>
            <span>{{ lastScanText }}</span>
            <el-select
              class="locale-switcher"
              :model-value="settingsStore.currentLocale"
              size="small"
              @change="changeLocale"
            >
              <el-option :label="t('topbar.english')" value="en-US" />
              <el-option :label="t('topbar.chinese')" value="zh-CN" />
            </el-select>
          </div>
        </header>

        <component :is="activeComponent" />
      </main>
    </div>
  </el-config-provider>
</template>
