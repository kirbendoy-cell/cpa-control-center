import { i18n } from '@/i18n'

export function formatDateTime(value: string | null | undefined): string {
  if (!value) {
    return i18n.global.t('common.notAvailable')
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return new Intl.DateTimeFormat(i18n.global.locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(parsed)
}
