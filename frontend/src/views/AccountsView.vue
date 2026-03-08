<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  ElButton,
  ElInput,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElPagination,
  ElSelect,
  ElTable,
  ElTableColumn,
} from 'element-plus'
import { useI18n } from 'vue-i18n'
import StatusPill from '@/components/StatusPill.vue'
import { useAccountsStore } from '@/stores/accounts'
import { useTasksStore } from '@/stores/tasks'
import { formatDateTime } from '@/utils/format'
import { stateDescription, stateOrder, normalizeStateKey } from '@/utils/status'
import { toErrorMessage } from '@/utils/errors'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const tasksStore = useTasksStore()

const query = ref('')
const stateFilter = ref('')
const providerFilter = ref('')
const currentPage = ref(1)
const pageSize = ref(20)
const pageSizeOptions = [20, 50, 100, 200]

const providerOptions = computed(() => [...new Set(accountsStore.accounts.map((item) => item.provider).filter(Boolean))])
const stateOptions = computed(() => stateOrder.map((value) => ({ value, label: t(`states.${value}`) })))

const filteredAccounts = computed(() =>
  accountsStore.accounts.filter((account) => {
    if (stateFilter.value && normalizeStateKey(account.stateKey || account.state) !== stateFilter.value) {
      return false
    }
    if (providerFilter.value && account.provider !== providerFilter.value) {
      return false
    }
    if (!query.value.trim()) {
      return true
    }
    const haystack = [account.name, account.email, account.provider, account.planType, account.probeErrorText].join(' ').toLowerCase()
    return haystack.includes(query.value.trim().toLowerCase())
  }),
)

const totalFiltered = computed(() => filteredAccounts.value.length)

const paginatedAccounts = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredAccounts.value.slice(start, start + pageSize.value)
})

watch([query, stateFilter, providerFilter], () => {
  currentPage.value = 1
})

watch(totalFiltered, (nextTotal) => {
  const maxPage = Math.max(1, Math.ceil(nextTotal / pageSize.value))
  if (currentPage.value > maxPage) {
    currentPage.value = maxPage
  }
})

watch(pageSize, () => {
  currentPage.value = 1
})

async function probe(name: string) {
  try {
    await accountsStore.probeAccount(name)
    ElMessage.success(t('accounts.messages.probed', { name }))
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}

async function toggleDisabled(name: string, disabled: boolean) {
  try {
    await ElMessageBox.confirm(
      t('accounts.dialogs.toggleMessage', { name, state: disabled ? t('accounts.actions.disable') : t('accounts.actions.enable') }),
      t('accounts.dialogs.toggleTitle'),
      {
        confirmButtonText: disabled ? t('accounts.actions.disable') : t('accounts.actions.enable'),
        cancelButtonText: t('accounts.dialogs.cancel'),
        customClass: 'cpa-message-box',
        type: disabled ? 'warning' : 'info',
      },
    )
    await accountsStore.setAccountDisabled(name, disabled)
    await accountsStore.refreshAll()
    ElMessage.success(t('accounts.messages.updated', { name }))
  } catch (error) {
    if (String(error) !== 'cancel') {
      ElMessage.error(toErrorMessage(error))
    }
  }
}

async function remove(name: string) {
  try {
    await ElMessageBox.confirm(
      t('accounts.dialogs.deleteMessage', { name }),
      t('accounts.dialogs.deleteTitle'),
      {
        confirmButtonText: t('accounts.actions.delete'),
        cancelButtonText: t('accounts.dialogs.cancel'),
        customClass: 'cpa-message-box',
        type: 'warning',
      },
    )
    await accountsStore.deleteAccount(name)
    await accountsStore.refreshAll()
    ElMessage.success(t('accounts.messages.deleted', { name }))
  } catch (error) {
    if (String(error) !== 'cancel') {
      ElMessage.error(toErrorMessage(error))
    }
  }
}

async function exportKind(kind: 'invalid401' | 'quotaLimited', format: 'json' | 'csv') {
  try {
    const result = await accountsStore.exportRecords(kind, format)
    ElMessage.success(t('accounts.messages.exported', { count: result.exported, path: result.path }))
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}
</script>

<template>
  <div class="view-shell view-shell--accounts">
    <section class="panel panel--fill">
      <div class="toolbar">
        <div class="toolbar-group">
          <el-input v-model="query" :placeholder="t('accounts.searchPlaceholder')" clearable />
          <el-select v-model="stateFilter" :placeholder="t('accounts.statePlaceholder')" clearable style="width: 180px">
            <el-option v-for="option in stateOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
          <el-select v-model="providerFilter" :placeholder="t('accounts.providerPlaceholder')" clearable style="width: 180px">
            <el-option v-for="provider in providerOptions" :key="provider" :label="provider" :value="provider" />
          </el-select>
        </div>
        <div class="toolbar-group toolbar-group--compact">
          <el-button plain @click="exportKind('invalid401', 'json')">{{ t('accounts.exportInvalidJson') }}</el-button>
          <el-button plain @click="exportKind('invalid401', 'csv')">{{ t('accounts.exportInvalidCsv') }}</el-button>
          <el-button plain @click="exportKind('quotaLimited', 'json')">{{ t('accounts.exportQuotaJson') }}</el-button>
          <el-button plain @click="exportKind('quotaLimited', 'csv')">{{ t('accounts.exportQuotaCsv') }}</el-button>
        </div>
      </div>

      <div class="panel__body panel__body--table">
        <div class="table-wrap">
          <el-table :data="paginatedAccounts" height="100%">
            <el-table-column prop="name" :label="t('accounts.columns.name')" min-width="220" />
            <el-table-column :label="t('accounts.columns.state')" width="144">
              <template #default="{ row }">
                <StatusPill :state="row.stateKey || row.state" />
              </template>
            </el-table-column>
            <el-table-column prop="email" :label="t('accounts.columns.email')" min-width="220" />
            <el-table-column prop="provider" :label="t('accounts.columns.provider')" width="120" />
            <el-table-column prop="planType" :label="t('accounts.columns.plan')" width="140" />
            <el-table-column :label="t('accounts.columns.disabled')" width="96">
              <template #default="{ row }">
                {{ row.disabled ? t('common.yes') : t('common.no') }}
              </template>
            </el-table-column>
            <el-table-column :label="t('accounts.columns.lastProbed')" min-width="180">
              <template #default="{ row }">
                {{ formatDateTime(row.lastProbedAt) }}
              </template>
            </el-table-column>
            <el-table-column :label="t('accounts.columns.details')" min-width="260">
              <template #default="{ row }">
                <span class="muted">{{ stateDescription(row) }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('accounts.columns.actions')" width="260" fixed="right">
              <template #default="{ row }">
                <div class="row-actions">
                  <el-button text :disabled="tasksStore.hasActiveTask" @click="probe(row.name)">{{ t('accounts.actions.probe') }}</el-button>
                  <el-button text :disabled="tasksStore.hasActiveTask" @click="toggleDisabled(row.name, !row.disabled)">
                    {{ row.disabled ? t('accounts.actions.enable') : t('accounts.actions.disable') }}
                  </el-button>
                  <el-button text type="danger" :disabled="tasksStore.hasActiveTask" @click="remove(row.name)">
                    {{ t('accounts.actions.delete') }}
                  </el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div class="table-footer">
          <span class="muted table-footer__summary">
            {{ t('accounts.paginationSummary', { shown: paginatedAccounts.length, total: totalFiltered, all: accountsStore.accounts.length }) }}
          </span>
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            background
            :page-sizes="pageSizeOptions"
            :total="totalFiltered"
            layout="total, sizes, prev, pager, next, jumper"
          />
        </div>
      </div>
    </section>
  </div>
</template>
