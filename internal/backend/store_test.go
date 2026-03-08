package backend

import (
	"testing"
)

func TestStoreSettingsAndHistory(t *testing.T) {
	t.Parallel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	settings, err := store.SaveSettings(AppSettings{
		BaseURL:         "https://example.com",
		ManagementToken: "token",
		Locale:          localeEnglish,
		DetailedLogs:    true,
		TargetType:      "codex",
		ProbeWorkers:    12,
		ActionWorkers:   6,
		TimeoutSeconds:  10,
		Retries:         2,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: store.exportsDir,
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if settings.BaseURL != "https://example.com" {
		t.Fatalf("unexpected BaseURL: %s", settings.BaseURL)
	}

	loaded, err := store.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if loaded.ManagementToken != "token" {
		t.Fatalf("unexpected token: %s", loaded.ManagementToken)
	}
	if !loaded.DetailedLogs {
		t.Fatalf("expected detailed logs to persist")
	}

	record := AccountRecord{
		Name:      "codex-1.json",
		Type:      "codex",
		Provider:  "codex",
		State:     stateNormal,
		StateKey:  stateNormal,
		UpdatedAt: nowISO(),
	}
	if err := store.ReplaceCurrentAccounts([]AccountRecord{record}); err != nil {
		t.Fatalf("ReplaceCurrentAccounts: %v", err)
	}

	items, err := store.ListAccounts(AccountFilter{Type: "codex"})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(items) != 1 || items[0].Name != record.Name {
		t.Fatalf("unexpected accounts: %+v", items)
	}

	runID, err := store.StartScanRun(loaded)
	if err != nil {
		t.Fatalf("StartScanRun: %v", err)
	}

	summary := ScanSummary{
		RunID:             runID,
		Status:            "success",
		StartedAt:         nowISO(),
		FinishedAt:        nowISO(),
		TotalAccounts:     1,
		FilteredAccounts:  1,
		ProbedAccounts:    1,
		NormalCount:       1,
		Invalid401Count:   0,
		QuotaLimitedCount: 0,
		RecoveredCount:    0,
		ErrorCount:        0,
		Delete401:         true,
		QuotaAction:       "disable",
		AutoReenable:      true,
		ProbeWorkers:      12,
		ActionWorkers:     6,
		TimeoutSeconds:    10,
		Retries:           2,
		Message:           "ok",
	}
	if err := store.FinishScanRun(summary); err != nil {
		t.Fatalf("FinishScanRun: %v", err)
	}
	if err := store.SaveScanRecords(runID, []AccountRecord{record}); err != nil {
		t.Fatalf("SaveScanRecords: %v", err)
	}

	history, err := store.ListScanHistory(5)
	if err != nil {
		t.Fatalf("ListScanHistory: %v", err)
	}
	if len(history) != 1 || history[0].RunID != runID {
		t.Fatalf("unexpected history: %+v", history)
	}

	detail, err := store.GetScanDetails(runID)
	if err != nil {
		t.Fatalf("GetScanDetails: %v", err)
	}
	if len(detail.Records) != 1 || detail.Records[0].Name != record.Name {
		t.Fatalf("unexpected detail: %+v", detail)
	}

	paged, err := store.GetScanDetailsPage(runID, 1, 1)
	if err != nil {
		t.Fatalf("GetScanDetailsPage: %v", err)
	}
	if paged.TotalRecords != 1 || len(paged.Records) != 1 || paged.Records[0].Name != record.Name {
		t.Fatalf("unexpected paged detail: %+v", paged)
	}
}
