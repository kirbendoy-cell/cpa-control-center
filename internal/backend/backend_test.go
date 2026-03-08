package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type fakeCPAServer struct {
	mu        sync.Mutex
	files     []map[string]any
	deleted   []string
	disabled  []string
	reenabled []string
}

func (f *fakeCPAServer) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v0/management/auth-files":
		_ = json.NewEncoder(w).Encode(map[string]any{"files": f.files})
	case r.Method == http.MethodPost && r.URL.Path == "/v0/management/api-call":
		var body struct {
			AuthIndex string `json:"authIndex"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch body.AuthIndex {
		case "invalid":
			_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 401, "body": ""})
		case "quota":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body":        `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":true}}`,
			})
		case "recovered":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body":        `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false}}`,
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status_code": 200,
				"body":        `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false}}`,
			})
		}
	case r.Method == http.MethodDelete && r.URL.Path == "/v0/management/auth-files":
		name := r.URL.Query().Get("name")
		f.deleted = append(f.deleted, name)
		next := make([]map[string]any, 0, len(f.files))
		for _, item := range f.files {
			if item["name"] != name {
				next = append(next, item)
			}
		}
		f.files = next
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	case r.Method == http.MethodPatch && r.URL.Path == "/v0/management/auth-files/status":
		var body struct {
			Name     string `json:"name"`
			Disabled bool   `json:"disabled"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Disabled {
			f.disabled = append(f.disabled, body.Name)
		} else {
			f.reenabled = append(f.reenabled, body.Name)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	default:
		http.NotFound(w, r)
	}
}

func TestBackendRunScanMaintainAndExport(t *testing.T) {
	serverState := &fakeCPAServer{
		files: []map[string]any{
			{
				"name":       "invalid-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "invalid",
				"id_token":   `{"chatgpt_account_id":"acct-invalid","plan_type":"pro"}`,
			},
			{
				"name":       "quota-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "quota",
				"id_token":   `{"chatgpt_account_id":"acct-quota","plan_type":"pro"}`,
			},
			{
				"name":       "healthy-codex.json",
				"type":       "codex",
				"provider":   "codex",
				"auth_index": "healthy",
				"id_token":   `{"chatgpt_account_id":"acct-healthy","plan_type":"pro"}`,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(serverState.handler))
	defer server.Close()

	dataDir := t.TempDir()
	service, err := New(dataDir, nil)
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}
	defer service.Close()

	_, err = service.SaveSettings(AppSettings{
		BaseURL:         server.URL,
		ManagementToken: "token",
		Locale:          localeEnglish,
		TargetType:      "codex",
		ProbeWorkers:    4,
		ActionWorkers:   2,
		TimeoutSeconds:  5,
		Retries:         0,
		UserAgent:       defaultUserAgent,
		QuotaAction:     "disable",
		Delete401:       true,
		AutoReenable:    true,
		ExportDirectory: filepath.Join(dataDir, "exports"),
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	summary, err := service.RunScan()
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if summary.Invalid401Count != 1 || summary.QuotaLimitedCount != 1 || summary.NormalCount != 1 {
		t.Fatalf("unexpected scan summary: %+v", summary)
	}

	snapshot, err := service.GetDashboardSnapshot()
	if err != nil {
		t.Fatalf("GetDashboardSnapshot: %v", err)
	}
	if snapshot.Summary.FilteredAccounts != 3 || len(snapshot.Accounts) != 3 || len(snapshot.History) != 1 {
		t.Fatalf("unexpected dashboard snapshot: %+v", snapshot)
	}

	exported, err := service.ExportAccounts("invalid401", "json", "")
	if err != nil {
		t.Fatalf("ExportAccounts: %v", err)
	}
	if exported.Exported != 1 {
		t.Fatalf("expected one exported invalid record, got %+v", exported)
	}
	if _, err := os.Stat(exported.Path); err != nil {
		t.Fatalf("expected export file: %v", err)
	}

	serverState.mu.Lock()
	serverState.files = append(serverState.files, map[string]any{
		"name":       "recovered-codex.json",
		"type":       "codex",
		"provider":   "codex",
		"auth_index": "recovered",
		"disabled":   true,
		"id_token":   `{"chatgpt_account_id":"acct-recovered","plan_type":"pro"}`,
	})
	serverState.mu.Unlock()

	storeRecord := AccountRecord{
		Name:             "recovered-codex.json",
		Type:             "codex",
		Provider:         "codex",
		State:            stateQuotaLimited,
		StateKey:         stateQuotaLimited,
		Disabled:         true,
		ManagedReason:    "quota_disabled",
		AuthIndex:        "recovered",
		ChatGPTAccountID: "acct-recovered",
		UpdatedAt:        nowISO(),
		LastSeenAt:       nowISO(),
	}
	if err := service.store.UpsertCurrentAccount(storeRecord); err != nil {
		t.Fatalf("UpsertCurrentAccount: %v", err)
	}

	result, err := service.RunMaintain(MaintainOptions{
		Delete401:    true,
		QuotaAction:  "disable",
		AutoReenable: true,
	})
	if err != nil {
		t.Fatalf("RunMaintain: %v", err)
	}
	if len(result.Delete401Results) != 1 || len(result.QuotaActionResults) != 1 || len(result.ReenableResults) != 1 {
		t.Fatalf("unexpected maintain result: %+v", result)
	}

	records, err := service.ListAccounts(AccountFilter{Type: "codex"})
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected three remaining records, got %d", len(records))
	}

	detailPage, err := service.GetScanDetailsPage(result.Scan.RunID, 1, 2)
	if err != nil {
		t.Fatalf("GetScanDetailsPage: %v", err)
	}
	if detailPage.TotalRecords != 4 || len(detailPage.Records) != 2 {
		t.Fatalf("unexpected scan detail page: %+v", detailPage)
	}

	serverState.mu.Lock()
	defer serverState.mu.Unlock()
	if len(serverState.deleted) != 1 || serverState.deleted[0] != "invalid-codex.json" {
		t.Fatalf("unexpected deleted names: %+v", serverState.deleted)
	}
	if len(serverState.disabled) != 1 || serverState.disabled[0] != "quota-codex.json" {
		t.Fatalf("unexpected disabled names: %+v", serverState.disabled)
	}
	if len(serverState.reenabled) != 1 || serverState.reenabled[0] != "recovered-codex.json" {
		t.Fatalf("unexpected reenabled names: %+v", serverState.reenabled)
	}
}
