package backend

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type quotaBucketKey string

const (
	quotaBucketFiveHour         quotaBucketKey = "fiveHour"
	quotaBucketWeekly           quotaBucketKey = "weekly"
	quotaBucketCodeReviewWeekly quotaBucketKey = "codeReviewWeekly"
)

type quotaBucketAccumulator struct {
	supported bool
	sum       float64
	count     int
	resetAt   string
	failed    int
}

type planQuotaAccumulator struct {
	planType         string
	accountCount     int
	fiveHour         quotaBucketAccumulator
	weekly           quotaBucketAccumulator
	codeReviewWeekly quotaBucketAccumulator
}

type quotaBucketValue struct {
	remainingPercent float64
	resetAt          string
}

type quotaBucketResult struct {
	fiveHour         *quotaBucketValue
	weekly           *quotaBucketValue
	codeReviewWeekly *quotaBucketValue
}

type quotaCandidate struct {
	path        string
	usedPercent float64
	resetAt     string
	window      time.Duration
	scoreBoost  int
}

type quotaFetchOutcome struct {
	record        AccountRecord
	planType      string
	usagePlanType string
	result        quotaBucketResult
	err           error
}

var planOrder = map[string]int{
	"free":       0,
	"plus":       1,
	"pro":        2,
	"team":       3,
	"business":   4,
	"enterprise": 5,
}

func (b *Backend) GetCodexQuotaSnapshot() (CodexQuotaSnapshot, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return CodexQuotaSnapshot{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return CodexQuotaSnapshot{}, err
	}

	ctx, err := b.beginTask("quota", settings.Locale)
	if err != nil {
		return CodexQuotaSnapshot{}, err
	}
	defer b.endTask()

	status := "success"
	finishMessage := msg(settings.Locale, "task.quota.no_accounts")
	defer func() {
		b.emitTaskFinished("quota", status, finishMessage)
	}()

	b.emitLog("quota", "info", msg(settings.Locale, "task.quota.started"))
	b.emitProgress("quota", "fetch", 0, 1, msg(settings.Locale, "task.scan.loading_inventory"), false)

	files, err := b.client.FetchAuthFiles(ctx, settings)
	if err != nil {
		status = taskStatus(err)
		finishMessage = msg(settings.Locale, "task.scan.failed_auth_files", err)
		b.emitLog("quota", "error", finishMessage)
		return CodexQuotaSnapshot{}, err
	}
	b.emitProgress("quota", "fetch", 1, 1, msg(settings.Locale, "task.scan.loaded_auth_files", len(files)), true)

	timestamp := nowISO()
	records := b.selectQuotaRecords(settings, files, timestamp)

	if len(records) == 0 {
		b.emitProgress("quota", "complete", 0, 0, finishMessage, true)
		b.emitLog("quota", "warning", finishMessage)
		return CodexQuotaSnapshot{
			Plans:              nil,
			FetchedAt:          timestamp,
			TotalAccounts:      0,
			SuccessfulAccounts: 0,
			FailedAccounts:     0,
		}, nil
	}

	b.emitLog("quota", "info", msg(settings.Locale, "task.quota.refreshing", len(records)))
	b.emitProgress("quota", "query", 0, len(records), msg(settings.Locale, "task.quota.querying_account", records[0].Name), false)

	accumulators := map[string]*planQuotaAccumulator{}
	var successfulAccounts int
	var failedAccounts int

	outcomes, err := b.fetchQuotaOutcomes(ctx, settings, records)
	if err != nil {
		status = taskStatus(err)
		finishMessage = msg(settings.Locale, "task.scan.stopped", taskName(settings.Locale, "quota"), err)
		b.emitLog("quota", "warning", finishMessage)
		return CodexQuotaSnapshot{}, err
	}

	for _, outcome := range outcomes {
		planType := outcome.planType
		accumulator := ensurePlanAccumulator(accumulators, planType)
		accumulator.accountCount++
		if outcome.err != nil {
			failedAccounts++
			markQuotaAccountFailure(accumulator)
			continue
		}
		if outcome.usagePlanType != "" && outcome.usagePlanType != planType {
			accumulator.accountCount--
			pruneEmptyPlanAccumulator(accumulators, accumulator)
			accumulator = ensurePlanAccumulator(accumulators, outcome.usagePlanType)
			accumulator.accountCount++
			planType = outcome.usagePlanType
		}

		successfulAccounts++
		applyQuotaBucketResult(accumulator, outcome.result)
	}

	plans := make([]CodexPlanQuotaSummary, 0, len(accumulators))
	for _, key := range sortedPlanKeys(accumulators) {
		plans = append(plans, buildPlanQuotaSummary(accumulators[key]))
	}

	snapshot := CodexQuotaSnapshot{
		Plans:              plans,
		FetchedAt:          timestamp,
		TotalAccounts:      successfulAccounts + failedAccounts,
		SuccessfulAccounts: successfulAccounts,
		FailedAccounts:     failedAccounts,
	}
	finishMessage = msg(settings.Locale, "task.quota.completed", snapshot.TotalAccounts, snapshot.SuccessfulAccounts, snapshot.FailedAccounts)
	if snapshot.FailedAccounts > 0 {
		b.emitLog("quota", "warning", finishMessage)
	} else {
		b.emitLog("quota", "info", finishMessage)
	}
	b.emitProgress("quota", "complete", 1, 1, finishMessage, true)
	return snapshot, nil
}

func (b *Backend) selectQuotaRecords(settings AppSettings, files []map[string]any, timestamp string) []AccountRecord {
	records := make([]AccountRecord, 0, len(files))
	freeSelected := 0
	freeLimit := settings.QuotaFreeMaxAccounts

	for _, item := range files {
		record := b.buildQuotaRecord(item, timestamp)
		if record.Name == "" {
			continue
		}
		planType := normalizeQuotaPlanType(record.PlanType)
		if !quotaPlanEnabled(settings, planType) {
			continue
		}
		if planType == "free" && freeLimit >= 0 {
			if freeSelected >= freeLimit {
				continue
			}
			freeSelected++
		}
		records = append(records, record)
	}

	return records
}

func (b *Backend) buildQuotaRecord(item map[string]any, timestamp string) AccountRecord {
	record := b.client.BuildAccountRecord(item, nil, timestamp)
	if !strings.EqualFold(record.Provider, "codex") && !strings.EqualFold(record.Type, "codex") {
		return AccountRecord{}
	}
	return record
}

func quotaPlanEnabled(settings AppSettings, planType string) bool {
	switch normalizeQuotaPlanType(planType) {
	case "free":
		return settings.QuotaCheckFree
	case "plus":
		return settings.QuotaCheckPlus
	case "pro":
		return settings.QuotaCheckPro
	case "team":
		return settings.QuotaCheckTeam
	case "business":
		return settings.QuotaCheckBusiness
	case "enterprise":
		return settings.QuotaCheckEnterprise
	default:
		return true
	}
}

func (b *Backend) fetchQuotaOutcomes(ctx context.Context, settings AppSettings, records []AccountRecord) ([]quotaFetchOutcome, error) {
	workers := settings.QuotaWorkers
	if workers <= 0 {
		workers = defaultQuotaWorkers
	}
	if workers > len(records) {
		workers = len(records)
	}
	if workers == 0 {
		return nil, nil
	}

	jobs := make(chan AccountRecord)
	outcomes := make(chan quotaFetchOutcome, workers)
	var wg sync.WaitGroup
	var completed int64

	for workerIndex := 0; workerIndex < workers; workerIndex++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case record, ok := <-jobs:
					if !ok {
						return
					}

					outcome := quotaFetchOutcome{
						record:   record,
						planType: normalizeQuotaPlanType(record.PlanType),
					}

					usage, err := b.client.FetchWhamUsage(ctx, settings, record)
					if err != nil {
						outcome.err = err
					} else {
						outcome.usagePlanType = normalizeQuotaPlanType(stringValue(usage["plan_type"]))
						outcome.result, outcome.err = parseQuotaBucketResult(usage)
					}

					current := int(atomic.AddInt64(&completed, 1))
					if outcome.err != nil {
						b.emitLog("quota", "warning", msg(settings.Locale, "task.quota.account_failed", record.Name, outcome.err))
					} else {
						logPlanType := stringOr(outcome.usagePlanType, outcome.planType)
						b.emitDetailedLog(settings.DetailedLogs, "quota", "info", msg(settings.Locale, "task.quota.account_loaded", record.Name, logPlanType))
					}
					b.emitProgress("quota", "query", current, len(records), msg(settings.Locale, "task.quota.querying_account", record.Name), current == len(records))

					select {
					case outcomes <- outcome:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, record := range records {
			select {
			case <-ctx.Done():
				return
			case jobs <- record:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(outcomes)
	}()

	results := make([]quotaFetchOutcome, 0, len(records))
	for outcome := range outcomes {
		results = append(results, outcome)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return results, nil
}

func ensurePlanAccumulator(accumulators map[string]*planQuotaAccumulator, planType string) *planQuotaAccumulator {
	key := normalizeQuotaPlanType(planType)
	if key == "" {
		key = "unknown"
	}
	if existing, ok := accumulators[key]; ok {
		return existing
	}

	accumulator := &planQuotaAccumulator{
		planType: key,
		fiveHour: quotaBucketAccumulator{
			supported: !strings.EqualFold(key, "free"),
		},
		weekly: quotaBucketAccumulator{
			supported: true,
		},
		codeReviewWeekly: quotaBucketAccumulator{
			supported: true,
		},
	}
	accumulators[key] = accumulator
	return accumulator
}

func markQuotaAccountFailure(plan *planQuotaAccumulator) {
	if plan.fiveHour.supported {
		plan.fiveHour.failed++
	}
	if plan.weekly.supported {
		plan.weekly.failed++
	}
	if plan.codeReviewWeekly.supported {
		plan.codeReviewWeekly.failed++
	}
}

func applyQuotaBucketResult(plan *planQuotaAccumulator, result quotaBucketResult) {
	applyQuotaBucketValue(&plan.fiveHour, result.fiveHour)
	applyQuotaBucketValue(&plan.weekly, result.weekly)
	applyQuotaBucketValue(&plan.codeReviewWeekly, result.codeReviewWeekly)
}

func applyQuotaBucketValue(bucket *quotaBucketAccumulator, value *quotaBucketValue) {
	if !bucket.supported {
		return
	}
	if value == nil {
		bucket.failed++
		return
	}
	bucket.sum += value.remainingPercent
	bucket.count++
	if bucket.resetAt == "" || earlierReset(value.resetAt, bucket.resetAt) {
		bucket.resetAt = value.resetAt
	}
}

func buildPlanQuotaSummary(plan *planQuotaAccumulator) CodexPlanQuotaSummary {
	return CodexPlanQuotaSummary{
		PlanType:         plan.planType,
		AccountCount:     plan.accountCount,
		FiveHour:         buildQuotaBucketSummary(plan.fiveHour),
		Weekly:           buildQuotaBucketSummary(plan.weekly),
		CodeReviewWeekly: buildQuotaBucketSummary(plan.codeReviewWeekly),
	}
}

func buildQuotaBucketSummary(bucket quotaBucketAccumulator) QuotaBucketSummary {
	summary := QuotaBucketSummary{
		Supported:    bucket.supported,
		ResetAt:      bucket.resetAt,
		SuccessCount: bucket.count,
		FailedCount:  bucket.failed,
	}
	if bucket.count > 0 {
		summary.TotalRemainingPercent = float64Ptr(roundToOneDecimal(bucket.sum))
	}
	return summary
}

func parseQuotaBucketResult(payload map[string]any) (quotaBucketResult, error) {
	candidates := collectQuotaCandidates("", payload)
	if len(candidates) == 0 {
		return quotaBucketResult{}, errors.New("no quota buckets found")
	}

	return quotaBucketResult{
		fiveHour:         selectQuotaCandidateValue(candidates, quotaBucketFiveHour),
		weekly:           selectQuotaCandidateValue(candidates, quotaBucketWeekly),
		codeReviewWeekly: selectQuotaCandidateValue(candidates, quotaBucketCodeReviewWeekly),
	}, nil
}

func collectQuotaCandidates(path string, value any) []quotaCandidate {
	var candidates []quotaCandidate

	switch typed := value.(type) {
	case map[string]any:
		if candidate, ok := buildQuotaCandidate(path, typed); ok {
			candidates = append(candidates, candidate)
		}
		for key, child := range typed {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			candidates = append(candidates, collectQuotaCandidates(nextPath, child)...)
		}
	case []any:
		for index, child := range typed {
			nextPath := path + "[" + stringValue(index) + "]"
			candidates = append(candidates, collectQuotaCandidates(nextPath, child)...)
		}
	}

	return candidates
}

func buildQuotaCandidate(path string, payload map[string]any) (quotaCandidate, bool) {
	usedPercent, ok := floatValueFromAny(payload["used_percent"])
	if !ok {
		return quotaCandidate{}, false
	}
	if usedPercent <= 1 {
		usedPercent *= 100
	}

	return quotaCandidate{
		path:        strings.ToLower(path),
		usedPercent: clampPercentage(usedPercent),
		resetAt:     normalizeQuotaResetAt(payload["reset_at"]),
		window:      quotaWindowDuration(payload),
		scoreBoost:  quotaCandidateScoreBoost(payload),
	}, true
}

func quotaWindowDuration(payload map[string]any) time.Duration {
	if seconds := durationFromKeys(payload, []string{"window_seconds", "windowSecs", "window_sec", "interval_seconds", "period_seconds", "reset_after_seconds"}); seconds > 0 {
		return seconds
	}
	if hours := durationFromKeys(payload, []string{"window_hours", "interval_hours", "period_hours"}); hours > 0 {
		return hours
	}
	if days := durationFromKeys(payload, []string{"window_days", "interval_days", "period_days"}); days > 0 {
		return days
	}
	return 0
}

func durationFromKeys(payload map[string]any, keys []string) time.Duration {
	for _, key := range keys {
		value, ok := floatValueFromAny(payload[key])
		if !ok || value <= 0 {
			continue
		}
		switch {
		case strings.Contains(key, "seconds") || strings.Contains(key, "_sec"):
			return time.Duration(value * float64(time.Second))
		case strings.Contains(key, "hours"):
			return time.Duration(value * float64(time.Hour))
		case strings.Contains(key, "days"):
			return time.Duration(value * 24 * float64(time.Hour))
		}
	}
	return 0
}

func quotaCandidateScoreBoost(payload map[string]any) int {
	score := 0
	if value, ok := boolFromMap(payload, "is_primary"); ok && value {
		score += 2
	}
	if value, ok := boolFromMap(payload, "primary"); ok && value {
		score += 2
	}
	return score
}

func selectQuotaCandidateValue(candidates []quotaCandidate, bucket quotaBucketKey) *quotaBucketValue {
	bestScore := math.MinInt
	var best *quotaCandidate
	for i := range candidates {
		score := quotaCandidateMatchScore(candidates[i], bucket)
		if score > bestScore {
			bestScore = score
			best = &candidates[i]
		}
	}
	if best == nil || bestScore < 0 {
		return nil
	}
	return &quotaBucketValue{
		remainingPercent: roundToOneDecimal(100 - best.usedPercent),
		resetAt:          best.resetAt,
	}
}

func quotaCandidateMatchScore(candidate quotaCandidate, bucket quotaBucketKey) int {
	path := candidate.path
	score := candidate.scoreBoost
	isReview := strings.Contains(path, "code_review") || strings.Contains(path, "codereview") || strings.Contains(path, "review")
	isWeekly := strings.Contains(path, "weekly") || strings.Contains(path, "week")
	isFiveHour := strings.Contains(path, "five_hour") || strings.Contains(path, "fivehour") || strings.Contains(path, "5h") || strings.Contains(path, "5_hour")
	matched := false

	switch bucket {
	case quotaBucketCodeReviewWeekly:
		if !isReview {
			return -1
		}
		matched = true
		score += 6
		if nearDuration(candidate.window, 7*24*time.Hour, 24*time.Hour) {
			score += 4
			matched = true
		}
		if isWeekly {
			score += 2
			matched = true
		}
	case quotaBucketFiveHour:
		if isReview {
			return -1
		}
		if nearDuration(candidate.window, 5*time.Hour, 45*time.Minute) {
			score += 6
			matched = true
		}
		if isFiveHour {
			score += 4
			matched = true
		}
		if !matched {
			return -1
		}
	case quotaBucketWeekly:
		if isReview {
			return -1
		}
		if nearDuration(candidate.window, 5*time.Hour, 45*time.Minute) {
			return -1
		}
		if nearDuration(candidate.window, 7*24*time.Hour, 24*time.Hour) {
			score += 6
			matched = true
		}
		if isWeekly {
			score += 3
			matched = true
		}
		if isFiveHour {
			return -1
		}
		if !matched {
			return -1
		}
	}

	if score == candidate.scoreBoost && candidate.window == 0 {
		return -1
	}
	return score
}

func nearDuration(actual time.Duration, expected time.Duration, tolerance time.Duration) bool {
	if actual <= 0 {
		return false
	}
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}

func normalizeQuotaResetAt(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case int:
		if typed > 0 {
			return time.Unix(int64(typed), 0).UTC().Format(time.RFC3339)
		}
	case int64:
		if typed > 0 {
			return time.Unix(typed, 0).UTC().Format(time.RFC3339)
		}
	case float64:
		if typed > 0 {
			return time.Unix(int64(typed), 0).UTC().Format(time.RFC3339)
		}
	case json.Number:
		if unixValue, err := typed.Float64(); err == nil && unixValue > 0 {
			return time.Unix(int64(unixValue), 0).UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func normalizeQuotaPlanType(planType string) string {
	normalized := strings.ToLower(strings.TrimSpace(planType))
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func sortedPlanKeys(accumulators map[string]*planQuotaAccumulator) []string {
	keys := make([]string, 0, len(accumulators))
	for key := range accumulators {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		leftRank, leftKnown := planOrder[keys[i]]
		rightRank, rightKnown := planOrder[keys[j]]
		if leftKnown && rightKnown {
			return leftRank < rightRank
		}
		if leftKnown != rightKnown {
			return leftKnown
		}
		return keys[i] < keys[j]
	})
	return keys
}

func pruneEmptyPlanAccumulator(accumulators map[string]*planQuotaAccumulator, accumulator *planQuotaAccumulator) {
	if accumulator == nil || accumulator.accountCount > 0 {
		return
	}
	if accumulator.fiveHour.count > 0 || accumulator.fiveHour.failed > 0 {
		return
	}
	if accumulator.weekly.count > 0 || accumulator.weekly.failed > 0 {
		return
	}
	if accumulator.codeReviewWeekly.count > 0 || accumulator.codeReviewWeekly.failed > 0 {
		return
	}
	delete(accumulators, accumulator.planType)
}

func earlierReset(left string, right string) bool {
	if left == "" {
		return false
	}
	if right == "" {
		return true
	}
	leftTime, leftErr := time.Parse(time.RFC3339, left)
	rightTime, rightErr := time.Parse(time.RFC3339, right)
	if leftErr != nil || rightErr != nil {
		return left < right
	}
	return leftTime.Before(rightTime)
}

func clampPercentage(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}

func roundToOneDecimal(value float64) float64 {
	return math.Round(value*10) / 10
}

func float64Ptr(value float64) *float64 {
	result := value
	return &result
}

func floatValueFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
