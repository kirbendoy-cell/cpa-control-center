import { defineStore } from 'pinia'
import { GetSettings, SaveSettings, TestConnection } from '../../wailsjs/go/main/App'
import { i18n, setI18nLocale } from '@/i18n'
import type { AppSettings, ConnectionResult } from '@/types'
import { createDefaultSettings, validateSettings } from '@/utils/settings'
import { toErrorMessage } from '@/utils/errors'
import { detectPreferredLocale, normalizeLocaleCode } from '@/utils/locale'

interface SettingsState {
  settings: AppSettings
  connection: ConnectionResult | null
  loading: boolean
  saving: boolean
  errors: Record<string, string>
}

export const useSettingsStore = defineStore('settingsStore', {
  state: (): SettingsState => ({
    settings: createDefaultSettings(),
    connection: null,
    loading: false,
    saving: false,
    errors: {},
  }),
  getters: {
    connectionTone: (state) => {
      if (!state.connection) {
        return 'idle'
      }
      return state.connection.ok ? 'ok' : 'error'
    },
    currentLocale: (state) => normalizeLocaleCode(state.settings.locale || i18n.global.locale.value),
  },
  actions: {
    applyLocale(locale?: string) {
      const next = setI18nLocale(locale || detectPreferredLocale())
      this.settings.locale = next
    },
    async persistSettings() {
      const saved = await SaveSettings(this.settings)
      this.settings = { ...createDefaultSettings(), ...saved, detailedLogs: this.settings.detailedLogs }
      this.applyLocale(this.settings.locale)
      return this.settings
    },
    async loadSettings() {
      this.loading = true
      try {
        const result = await GetSettings()
        this.settings = { ...createDefaultSettings(), ...result }
        this.applyLocale(this.settings.locale || detectPreferredLocale())
      } finally {
        this.loading = false
      }
    },
    async saveLocalePreference(locale: string) {
      const previous = this.currentLocale
      this.applyLocale(locale)
      try {
        await this.persistSettings()
      } catch (error) {
        this.applyLocale(previous)
        throw new Error(toErrorMessage(error))
      }
    },
    async testConnection() {
      this.errors = validateSettings(this.settings, i18n.global.t)
      if (Object.keys(this.errors).length > 0) {
        throw new Error(i18n.global.t('validation.fixBeforeTesting'))
      }
      this.connection = await TestConnection(this.settings)
      return this.connection
    },
    async saveSettings() {
      this.errors = validateSettings(this.settings, i18n.global.t)
      if (Object.keys(this.errors).length > 0) {
        throw new Error(i18n.global.t('validation.fixBeforeSaving'))
      }
      this.saving = true
      try {
        return await this.persistSettings()
      } finally {
        this.saving = false
      }
    },
    async testAndSave() {
      try {
        const connection = await this.testConnection()
        await this.saveSettings()
        this.connection = connection
        return connection
      } catch (error) {
        throw new Error(toErrorMessage(error))
      }
    },
  },
})
