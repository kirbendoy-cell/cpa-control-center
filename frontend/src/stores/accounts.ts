import { defineStore } from 'pinia'
import {
  DeleteAccount,
  ExportAccounts,
  GetDashboardSnapshot,
  GetScanDetailsPage,
  ProbeAccount,
  SetAccountDisabled,
} from '../../wailsjs/go/main/App'
import type {
  AccountRecord,
  AccountUpdate,
  DashboardSnapshot,
  DashboardSummary,
  ExportResult,
  ScanDetailPage,
  ScanSummary,
} from '@/types'
import { toErrorMessage } from '@/utils/errors'
import { normalizeStateKey } from '@/utils/status'

interface AccountsState {
  accounts: AccountRecord[]
  summary: DashboardSummary
  history: ScanSummary[]
  scanDetail: ScanDetailPage | null
  loading: boolean
}

function emptySummary(): DashboardSummary {
  return {
    totalAccounts: 0,
    filteredAccounts: 0,
    normalCount: 0,
    invalid401Count: 0,
    quotaLimitedCount: 0,
    recoveredCount: 0,
    errorCount: 0,
    lastScanAt: '',
  }
}

export const useAccountsStore = defineStore('accountsStore', {
  state: (): AccountsState => ({
    accounts: [],
    summary: emptySummary(),
    history: [],
    scanDetail: null,
    loading: false,
  }),
  actions: {
    async refreshAll() {
      this.loading = true
      try {
        const snapshot = await GetDashboardSnapshot() as DashboardSnapshot
        this.summary = snapshot.summary
        this.accounts = snapshot.accounts
        this.history = snapshot.history
      } finally {
        this.loading = false
      }
    },
    async loadScanDetail(runId: number, page = 1, pageSize = 20) {
      this.scanDetail = await GetScanDetailsPage(runId, page, pageSize) as ScanDetailPage
      return this.scanDetail
    },
    applyAccountUpdate(update: AccountUpdate) {
      const next = [...this.accounts]
      const index = next.findIndex((item) => item.name === update.record.name)
      if (update.removed) {
        if (index >= 0) {
          next.splice(index, 1)
        }
      } else if (index >= 0) {
        next[index] = update.record
      } else {
        next.unshift(update.record)
      }
      this.accounts = next
      this.recomputeSummary()
    },
    recomputeSummary() {
      const summary = emptySummary()
      summary.filteredAccounts = this.accounts.length
      summary.totalAccounts = Math.max(this.summary.totalAccounts, this.accounts.length)
      for (const account of this.accounts) {
        switch (normalizeStateKey(account.stateKey || account.state)) {
          case 'normal':
            summary.normalCount += 1
            break
          case 'invalid_401':
            summary.invalid401Count += 1
            break
          case 'quota_limited':
            summary.quotaLimitedCount += 1
            break
          case 'recovered':
            summary.recoveredCount += 1
            break
          case 'error':
            summary.errorCount += 1
            break
        }
        if (!summary.lastScanAt || account.lastProbedAt > summary.lastScanAt) {
          summary.lastScanAt = account.lastProbedAt
        }
      }
      this.summary = summary
    },
    async probeAccount(name: string) {
      try {
        const record = await ProbeAccount(name)
        this.applyAccountUpdate({ action: 'probe', removed: false, record })
        return record
      } catch (error) {
        throw new Error(toErrorMessage(error))
      }
    },
    async setAccountDisabled(name: string, disabled: boolean) {
      try {
        return await SetAccountDisabled(name, disabled)
      } catch (error) {
        throw new Error(toErrorMessage(error))
      }
    },
    async deleteAccount(name: string) {
      try {
        return await DeleteAccount(name)
      } catch (error) {
        throw new Error(toErrorMessage(error))
      }
    },
    async exportRecords(kind: 'invalid401' | 'quotaLimited', format: 'json' | 'csv') {
      try {
        return await ExportAccounts(kind, format, '') as ExportResult
      } catch (error) {
        throw new Error(toErrorMessage(error))
      }
    },
  },
})
