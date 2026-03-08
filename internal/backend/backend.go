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

func (b *Backend) GetDashboardSummary() (DashboardSummary, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return DashboardSummary{}, err
	}
	allRecords, err := b.store.ListAccounts(AccountFilter{})
	if err != nil {
		return DashboardSummary{}, err
	}

	filteredRecords := filterAccountsBySettings(allRecords, settings)
	summary := computeSummary(filteredRecords)
	summary.TotalAccounts = len(allRecords)
	return summary, nil
}

func (b *Backend) GetDashboardSnapshot() (DashboardSnapshot, error) {
	settings, err := b.store.LoadSettings()
	if err != nil {
		return DashboardSnapshot{}, err
	}

	allRecords, err := b.store.ListAccounts(AccountFilter{})
	if err != nil {
		return DashboardSnapshot{}, err
	}
	filteredRecords := filterAccountsBySettings(allRecords, settings)
	summary := computeSummary(filteredRecords)
	summary.TotalAccounts = len(allRecords)

	history, err := b.store.ListScanHistory(12)
	if err != nil {
		return DashboardSnapshot{}, err
	}

	return DashboardSnapshot{
		Summary:  summary,
		Accounts: filteredRecords,
		History:  history,
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
