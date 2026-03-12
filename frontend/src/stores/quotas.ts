import { defineStore } from 'pinia'
import { GetCodexQuotaSnapshot } from '../../wailsjs/go/main/App'
import type { CodexQuotaSnapshot } from '@/types'
import { toErrorMessage } from '@/utils/errors'

interface QuotasState {
  snapshot: CodexQuotaSnapshot | null
  loading: boolean
  error: string
  hasRequested: boolean
}

export const useQuotasStore = defineStore('quotasStore', {
  state: (): QuotasState => ({
    snapshot: null,
    loading: false,
    error: '',
    hasRequested: false,
  }),
  getters: {
    plans: (state) => state.snapshot?.plans ?? [],
    hasData: (state) => (state.snapshot?.plans?.length ?? 0) > 0,
    lastFetchedAt: (state) => state.snapshot?.fetchedAt ?? '',
  },
  actions: {
    async refreshSnapshot() {
      this.loading = true
      this.error = ''
      this.hasRequested = true
      try {
        const snapshot = await GetCodexQuotaSnapshot() as CodexQuotaSnapshot
        this.snapshot = snapshot
        return snapshot
      } catch (error) {
        const message = toErrorMessage(error)
        this.error = message
        if (!this.snapshot) {
          this.snapshot = null
        }
        throw new Error(message)
      } finally {
        this.loading = false
      }
    },
  },
})
