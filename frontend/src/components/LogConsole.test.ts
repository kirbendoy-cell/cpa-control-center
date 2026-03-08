import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import LogConsole from '@/components/LogConsole.vue'
import { createTestI18n } from '@/test/i18n'

describe('LogConsole', () => {
  it('renders incoming log rows', () => {
    const wrapper = mount(LogConsole, {
      global: {
        plugins: [createTestI18n()],
      },
      props: {
        entries: [
          {
            kind: 'scan',
            level: 'info',
            message: 'scan started',
            timestamp: '2026-03-07T12:00:00Z',
          },
        ],
      },
    })

    expect(wrapper.text()).toContain('scan started')
    expect(wrapper.text()).toContain('Scan')
  })
})
