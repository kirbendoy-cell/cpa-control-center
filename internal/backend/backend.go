package backend

import (
	"context"
	"errors"
	"os"
	"sync"
)

type EventEmitter interface {
	Emit(event string, payload any)
}

type Backend struct {
	store   *Store
	client  *Client
	logger  *Logger
	emitter EventEmitter

	mu         sync.Mutex
	activeKind string
	cancelFunc context.CancelFunc
}

func New(dataDir string, emitter EventEmitter) (*Backend, error) {
	store, err := NewStore(dataDir)
	if err != nil {
		return nil, err
	}

	logger, err := NewLogger(logFilePath(dataDir))
	if err != nil {
		_ = store.Close()
		return nil, err
	}

	return &Backend{
		store:   store,
		client:  NewClient(),
		logger:  logger,
		emitter: emitter,
	}, nil
}

func DefaultDataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepathJoin(configDir, "CPA Control Center"), nil
}

func (b *Backend) Close() error {
	if b == nil {
		return nil
	}
	var firstErr error
	if err := b.logger.Close(); err != nil {
		firstErr = err
	}
	if err := b.store.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (b *Backend) GetSettings() (AppSettings, error) {
	return b.store.LoadSettings()
}

func (b *Backend) SaveSettings(input AppSettings) (AppSettings, error) {
	settings, err := b.store.SaveSettings(input)
	if err != nil {
		return settings, err
	}
	b.emitLog("scan", "info", msg(settings.Locale, "settings.saved", stringOr(settings.BaseURL, "(empty)")))
	return settings, nil
}

func (b *Backend) TestConnection(input AppSettings) (ConnectionResult, error) {
	settings := normalizeSettings(input, b.store.exportsDir)
	result, err := b.client.TestConnection(context.Background(), settings)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (b *Backend) SyncInventory() (InventorySyncResult, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return InventorySyncResult{}, err
	}
	if err := ensureConfigured(settings); err != nil {
		return InventorySyncResult{}, err
	}

	files, err := b.client.FetchAuthFiles(context.Background(), settings)
	if err != nil {
		return InventorySyncResult{}, err
	}

	existing, err := b.store.LoadCurrentMap()
	if err != nil {
		return InventorySyncResult{}, err
	}

	timestamp := nowISO()
	records := make([]AccountRecord, 0, len(files))
	filteredCount := 0
	for _, item := range files {
		name := stringValue(item["name"])
		if name == "" {
			continue
		}
		var previous *AccountRecord
		if current, ok := existing[name]; ok {
			currentCopy := current
			previous = &currentCopy
		}
		record := b.client.BuildAccountRecord(item, previous, timestamp)
		record = carryInventorySnapshot(record, previous)
		if matchesInventoryFilter(record, settings) {
			filteredCount++
		}
		records = append(records, record)
	}

	if err := b.store.ReplaceCurrentAccounts(records); err != nil {
		return InventorySyncResult{}, err
	}

	result := InventorySyncResult{
		TotalAccounts:    len(records),
		FilteredAccounts: filteredCount,
		SyncedAt:         timestamp,
	}
	b.emitLog("scan", "info", msg(settings.Locale, "task.inventory.synced", filteredCount, len(records)))
	return result, nil
}

func (b *Backend) GetDashboardSummary() (DashboardSummary, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return DashboardSummary{}, err
	}
	summary, err := b.store.SummarizeAccounts(AccountFilter{
		Type:     settings.TargetType,
		Provider: settings.Provider,
	})
	if err != nil {
		return DashboardSummary{}, err
	}
	totalAccounts, err := b.store.CountAccounts(AccountFilter{})
	if err != nil {
		return DashboardSummary{}, err
	}
	summary.TotalAccounts = totalAccounts
	return summary, nil
}

func (b *Backend) GetDashboardSnapshot() (DashboardSnapshot, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return DashboardSnapshot{}, err
	}

	summary, err := b.store.SummarizeAccounts(AccountFilter{
		Type:     settings.TargetType,
		Provider: settings.Provider,
	})
	if err != nil {
		return DashboardSnapshot{}, err
	}
	totalAccounts, err := b.store.CountAccounts(AccountFilter{})
	if err != nil {
		return DashboardSnapshot{}, err
	}
	summary.TotalAccounts = totalAccounts

	history, err := b.store.ListScanHistory(12)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	if history == nil {
		history = make([]ScanSummary, 0)
	}

	return DashboardSnapshot{
		Summary: summary,
		History: history,
	}, nil
}

func (b *Backend) ListAccounts(filter AccountFilter) ([]AccountRecord, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return nil, err
	}
	if filter.Type == "" {
		filter.Type = settings.TargetType
	}
	if filter.Provider == "" {
		filter.Provider = settings.Provider
	}
	return b.store.ListAccounts(filter)
}

func (b *Backend) ListAccountsPage(filter AccountFilter, page int, pageSize int) (AccountPage, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return AccountPage{}, err
	}
	if filter.Type == "" {
		filter.Type = settings.TargetType
	}
	if filter.Provider == "" {
		filter.Provider = settings.Provider
	}
	return b.store.ListAccountsPage(filter, page, pageSize)
}

func (b *Backend) CancelScan() (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cancelFunc == nil {
		return false, nil
	}

	b.cancelFunc()
	return true, nil
}

func (b *Backend) beginTask(kind string, locale string) (context.Context, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cancelFunc != nil {
		return nil, errors.New(msg(locale, "error.task_already_running", taskName(locale, b.activeKind)))
	}

	ctx, cancel := context.WithCancel(context.Background())
	b.activeKind = kind
	b.cancelFunc = cancel
	return ctx, nil
}

func (b *Backend) endTask() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.activeKind = ""
	b.cancelFunc = nil
}

func (b *Backend) emitLog(kind string, level string, message string) {
	b.emitLogInternal(kind, level, message)
}

func (b *Backend) emitDetailedLog(enabled bool, kind string, level string, message string) {
	if !enabled {
		return
	}
	b.emitLogInternal(kind, level, message)
}

func (b *Backend) emitLogInternal(kind string, level string, message string) {
	entry := LogEntry{
		Kind:      kind,
		Level:     level,
		Message:   message,
		Timestamp: nowISO(),
	}
	if b.logger != nil {
		_ = b.logger.Write(entry)
	}
	if b.emitter != nil {
		b.emitter.Emit(kind+":log", entry)
	}
}

func (b *Backend) emitProgress(kind string, phase string, current int, total int, message string, done bool) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit(kind+":progress", TaskProgress{
		Kind:    kind,
		Phase:   phase,
		Current: current,
		Total:   total,
		Message: message,
		Done:    done,
	})
}

func (b *Backend) emitAccountUpdate(action string, removed bool, record AccountRecord) {
	if b.emitter == nil {
		return
	}
	b.emitter.Emit("account:update", AccountUpdate{
		Action:  action,
		Removed: removed,
		Record:  record,
	})
}

func filterAccountsBySettings(records []AccountRecord, settings AppSettings) []AccountRecord {
	var filtered []AccountRecord
	for _, record := range records {
		if matchesInventoryFilter(record, settings) {
			filtered = append(filtered, record)
		}
	}
	sortAccounts(filtered)
	return filtered
}

func ensureConfigured(settings AppSettings) error {
	if settings.BaseURL == "" {
		return errors.New(msg(settings.Locale, "error.base_url_required"))
	}
	if settings.ManagementToken == "" {
		return errors.New(msg(settings.Locale, "error.management_token_required"))
	}
	return nil
}
