<script setup lang="ts">
import { computed } from 'vue'
import { ElButton, ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useQuotasStore } from '@/stores/quotas'
import type { QuotaBucketSummary } from '@/types'
import { formatDateTime } from '@/utils/format'
import { toErrorMessage } from '@/utils/errors'

const { t } = useI18n()
const quotasStore = useQuotasStore()

const plans = computed(() => quotasStore.plans)
const snapshot = computed(() => quotasStore.snapshot)
const hasData = computed(() => plans.value.length > 0)
const hasRequested = computed(() => quotasStore.hasRequested)
const lastFetchedLabel = computed(() => (
  quotasStore.lastFetchedAt ? formatDateTime(quotasStore.lastFetchedAt) : t('common.notAvailable')
))
const showPartialWarning = computed(() => (
  Boolean(snapshot.value && snapshot.value.failedAccounts > 0 && snapshot.value.successfulAccounts > 0)
))

function formatTotalRemainingPercent(value?: number | null) {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return t('quotas.unavailable')
  }
  const rounded = Math.abs(value - Math.round(value)) < 0.05 ? Math.round(value) : value.toFixed(1)
  return t('quotas.totalRemainingPercent', { value: rounded })
}

function formatResetAt(value: string) {
  return value ? formatDateTime(value) : t('common.notAvailable')
}

function coverageLabel(successCount: number, failedCount: number) {
  return t('quotas.coverage', { success: successCount, total: successCount + failedCount })
}

function quotaCapacity(bucket: QuotaBucketSummary) {
  return bucket.successCount * 100
}

function averageRemainingPercent(bucket: QuotaBucketSummary) {
  const total = bucket.totalRemainingPercent
  if (typeof total !== 'number' || Number.isNaN(total) || bucket.successCount <= 0) {
    return null
  }
  return total / bucket.successCount
}

function formatAverageRemaining(bucket: QuotaBucketSummary) {
  const average = averageRemainingPercent(bucket)
  if (typeof average !== 'number' || Number.isNaN(average)) {
    return t('quotas.unavailable')
  }
  const rounded = Math.abs(average - Math.round(average)) < 0.05 ? Math.round(average) : average.toFixed(1)
  return t('quotas.averageRemainingPercent', { value: rounded })
}

function formatCapacity(bucket: QuotaBucketSummary) {
  return t('quotas.capacityPercent', { value: quotaCapacity(bucket) })
}

function normalizedFill(bucket: QuotaBucketSummary) {
  const average = averageRemainingPercent(bucket)
  if (typeof average !== 'number' || Number.isNaN(average)) {
    return 0
  }
  return Math.max(0, Math.min(100, average))
}

function interpolateChannel(start: number, end: number, ratio: number) {
  return Math.round(start + (end - start) * ratio)
}

function meterColor(bucket: QuotaBucketSummary) {
  const fill = normalizedFill(bucket)
  const low = { r: 193, g: 74, b: 56 }
  const mid = { r: 201, g: 154, b: 37 }
  const high = { r: 45, g: 139, b: 107 }

  let start = low
  let end = mid
  let ratio = fill / 50
  if (fill >= 50) {
    start = mid
    end = high
    ratio = (fill - 50) / 50
  }

  const r = interpolateChannel(start.r, end.r, ratio)
  const g = interpolateChannel(start.g, end.g, ratio)
  const b = interpolateChannel(start.b, end.b, ratio)
  return `rgb(${r}, ${g}, ${b})`
}

async function refreshSnapshot() {
  try {
    await quotasStore.refreshSnapshot()
  } catch (error) {
    ElMessage.error(toErrorMessage(error))
  }
}
</script>

<template>
  <div class="view-shell view-shell--quotas">
    <section class="hero-panel quota-hero">
      <div>
        <p class="eyebrow">{{ t('quotas.eyebrow') }}</p>
        <h2>{{ t('quotas.title') }}</h2>
        <p class="lead">
          {{ t('quotas.lead') }}
        </p>
      </div>
      <div class="quota-hero__actions">
        <div class="quota-hero__meta muted">
          <span>{{ t('quotas.lastUpdated', { value: lastFetchedLabel }) }}</span>
          <span v-if="snapshot">
            {{ t('quotas.accountsSummary', { total: snapshot.totalAccounts, success: snapshot.successfulAccounts, failed: snapshot.failedAccounts }) }}
          </span>
        </div>
        <el-button :loading="quotasStore.loading" @click="refreshSnapshot">
          {{ t('quotas.refresh') }}
        </el-button>
      </div>
    </section>

    <article v-if="showPartialWarning" class="panel quota-callout quota-callout--warning">
      <strong>{{ t('quotas.partialWarningTitle') }}</strong>
      <p class="muted">
        {{ t('quotas.partialWarningBody', { failed: snapshot?.failedAccounts ?? 0 }) }}
      </p>
    </article>

    <article v-if="quotasStore.error && !hasData" class="panel quota-callout quota-callout--error">
      <strong>{{ t('quotas.loadFailedTitle') }}</strong>
      <p class="muted">{{ quotasStore.error }}</p>
    </article>

    <article v-if="quotasStore.loading && !hasData" class="panel panel--fill">
      <div class="panel-head panel-head--tight">
        <div>
          <p class="panel-kicker">{{ t('quotas.eyebrow') }}</p>
          <h3>{{ t('common.loading') }}</h3>
        </div>
      </div>
      <div class="panel__body muted">
        {{ t('quotas.loading') }}
      </div>
    </article>

    <article v-else-if="!hasRequested" class="panel panel--fill quota-empty-state">
      <div class="panel__body quota-empty-state__body">
        <strong>{{ t('quotas.clickRefreshTitle') }}</strong>
        <p class="muted">{{ t('quotas.clickRefreshBody') }}</p>
        <el-button type="primary" :loading="quotasStore.loading" @click="refreshSnapshot">
          {{ t('quotas.refresh') }}
        </el-button>
      </div>
    </article>

    <article v-else-if="!hasData" class="panel panel--fill">
      <div class="panel-head panel-head--tight">
        <div>
          <p class="panel-kicker">{{ t('quotas.eyebrow') }}</p>
          <h3>{{ t('quotas.emptyTitle') }}</h3>
        </div>
      </div>
      <div class="panel__body muted">
        {{ t('quotas.emptyBody') }}
      </div>
    </article>

    <section v-else class="quota-grid">
      <article v-for="plan in plans" :key="plan.planType" class="panel quota-plan-card">
        <div class="panel-head panel-head--tight">
          <div>
            <p class="panel-kicker">{{ t('quotas.planLabel') }}</p>
            <h3>{{ plan.planType }}</h3>
          </div>
          <span class="quota-plan-card__count">
            {{ t('quotas.planAccounts', { count: plan.accountCount }) }}
          </span>
        </div>

        <div class="quota-plan-card__buckets quota-plan-card__buckets--visual">
          <section v-if="plan.fiveHour.supported" class="quota-bucket">
            <div class="quota-bucket__head">
              <strong>{{ t('quotas.buckets.fiveHour') }}</strong>
              <span>{{ coverageLabel(plan.fiveHour.successCount, plan.fiveHour.failedCount) }}</span>
            </div>
            <div class="quota-bucket__value quota-bucket__value--hero">
              {{ formatTotalRemainingPercent(plan.fiveHour.totalRemainingPercent) }}
            </div>
            <div class="quota-bucket__meter" aria-hidden="true">
              <span class="quota-bucket__meter-fill" :style="{ width: `${normalizedFill(plan.fiveHour)}%`, backgroundColor: meterColor(plan.fiveHour) }" />
            </div>
            <div class="quota-bucket__stats muted">
              <span>{{ formatAverageRemaining(plan.fiveHour) }}</span>
              <span>{{ formatCapacity(plan.fiveHour) }}</span>
            </div>
            <p class="muted quota-bucket__reset">
              {{ t('quotas.resetAt', { value: formatResetAt(plan.fiveHour.resetAt) }) }}
            </p>
          </section>

          <section class="quota-bucket">
            <div class="quota-bucket__head">
              <strong>{{ t('quotas.buckets.weekly') }}</strong>
              <span>{{ coverageLabel(plan.weekly.successCount, plan.weekly.failedCount) }}</span>
            </div>
            <div class="quota-bucket__value quota-bucket__value--hero">
              {{ formatTotalRemainingPercent(plan.weekly.totalRemainingPercent) }}
            </div>
            <div class="quota-bucket__meter" aria-hidden="true">
              <span class="quota-bucket__meter-fill" :style="{ width: `${normalizedFill(plan.weekly)}%`, backgroundColor: meterColor(plan.weekly) }" />
            </div>
            <div class="quota-bucket__stats muted">
              <span>{{ formatAverageRemaining(plan.weekly) }}</span>
              <span>{{ formatCapacity(plan.weekly) }}</span>
            </div>
            <p class="muted quota-bucket__reset">
              {{ t('quotas.resetAt', { value: formatResetAt(plan.weekly.resetAt) }) }}
            </p>
          </section>

          <section class="quota-bucket">
            <div class="quota-bucket__head">
              <strong>{{ t('quotas.buckets.codeReviewWeekly') }}</strong>
              <span>{{ coverageLabel(plan.codeReviewWeekly.successCount, plan.codeReviewWeekly.failedCount) }}</span>
            </div>
            <div class="quota-bucket__value quota-bucket__value--hero">
              {{ formatTotalRemainingPercent(plan.codeReviewWeekly.totalRemainingPercent) }}
            </div>
            <div class="quota-bucket__meter" aria-hidden="true">
              <span class="quota-bucket__meter-fill" :style="{ width: `${normalizedFill(plan.codeReviewWeekly)}%`, backgroundColor: meterColor(plan.codeReviewWeekly) }" />
            </div>
            <div class="quota-bucket__stats muted">
              <span>{{ formatAverageRemaining(plan.codeReviewWeekly) }}</span>
              <span>{{ formatCapacity(plan.codeReviewWeekly) }}</span>
            </div>
            <p class="muted quota-bucket__reset">
              {{ t('quotas.resetAt', { value: formatResetAt(plan.codeReviewWeekly.resetAt) }) }}
            </p>
          </section>
        </div>
      </article>
    </section>
  </div>
</template>
