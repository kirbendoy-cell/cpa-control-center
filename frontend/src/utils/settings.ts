import type { AppSettings } from '@/types'
import { detectPreferredLocale } from '@/utils/locale'

type Translate = (key: string, params?: Record<string, unknown>) => string

const fallbackTranslate: Translate = (key) => key

export function createDefaultSettings(): AppSettings {
  return {
    baseUrl: '',
    managementToken: '',
    locale: detectPreferredLocale(),
    detailedLogs: false,
    targetType: 'codex',
    provider: '',
    probeWorkers: 40,
    actionWorkers: 20,
    timeoutSeconds: 15,
    retries: 1,
    userAgent: 'codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal',
    quotaAction: 'disable',
    delete401: true,
    autoReenable: true,
    exportDirectory: '',
  }
}

export function validateSettings(settings: AppSettings, t: Translate = fallbackTranslate): Record<string, string> {
  const errors: Record<string, string> = {}

  if (!settings.baseUrl.trim()) {
    errors.baseUrl = t('validation.baseUrlRequired')
  } else if (!/^https?:\/\//i.test(settings.baseUrl.trim())) {
    errors.baseUrl = t('validation.baseUrlProtocol')
  }

  if (!settings.managementToken.trim()) {
    errors.managementToken = t('validation.managementTokenRequired')
  }

  if (settings.probeWorkers < 1) {
    errors.probeWorkers = t('validation.probeWorkersMin')
  }
  if (settings.actionWorkers < 1) {
    errors.actionWorkers = t('validation.actionWorkersMin')
  }
  if (settings.timeoutSeconds < 1) {
    errors.timeoutSeconds = t('validation.timeoutMin')
  }
  if (settings.retries < 0) {
    errors.retries = t('validation.retriesMin')
  }
  if (!['disable', 'delete'].includes(settings.quotaAction)) {
    errors.quotaAction = t('validation.quotaActionInvalid')
  }

  return errors
}
