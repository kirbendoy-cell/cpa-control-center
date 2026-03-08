import { describe, expect, it } from 'vitest'
import { createDefaultSettings, validateSettings } from '@/utils/settings'

describe('validateSettings', () => {
  it('accepts a valid CPA profile', () => {
    const settings = createDefaultSettings()
    expect(settings.detailedLogs).toBe(false)
    settings.baseUrl = 'https://example.com'
    settings.managementToken = 'token'

    expect(validateSettings(settings)).toEqual({})
  })

  it('rejects missing or malformed core fields', () => {
    const settings = createDefaultSettings()
    settings.baseUrl = 'example.com'
    settings.managementToken = ''
    settings.probeWorkers = 0

    expect(validateSettings(settings)).toMatchObject({
      baseUrl: expect.any(String),
      managementToken: expect.any(String),
      probeWorkers: expect.any(String),
    })
  })
})
