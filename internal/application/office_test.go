package application

import (
	"context"
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/domain/office"
	"github.com/ychiu1211/dsmctl/internal/synology"
)

type fakeOfficeClient struct {
	system       synology.OfficeSystemSettings
	preferences  synology.OfficePreferences
	systemErr    error
	systemSets   []office.SystemChange
	prefSets     []office.PreferencesChange
	applySystem  func(office.SystemChange)
	applyPrefs   func(office.PreferencesChange)
	capabilities synology.OfficeCapabilities
}

func newFakeOfficeClient() *fakeOfficeClient {
	client := &fakeOfficeClient{
		system:      synology.OfficeSystemSettings{HistoryPrune: false},
		preferences: synology.OfficePreferences{Ruler: true, AIHelperLanguages: []string{}},
		capabilities: synology.OfficeCapabilities{
			Module: office.ModuleName, InfoRead: true,
			SystemRead: true, SystemSet: true,
			PreferencesRead: true, PreferencesSet: true, FontsRead: true,
			Package: office.PackageEvidence{ID: "Spreadsheet", Installed: true, Version: "3.7.2-22592", Running: true},
		},
	}
	client.applySystem = func(change office.SystemChange) {
		if change.HistoryPrune != nil {
			client.system.HistoryPrune = *change.HistoryPrune
		}
	}
	client.applyPrefs = func(change office.PreferencesChange) {
		if change.Ruler != nil {
			client.preferences.Ruler = *change.Ruler
		}
	}
	return client
}

func (c *fakeOfficeClient) OfficeInfo(context.Context) (synology.OfficeInfo, error) {
	return synology.OfficeInfo{Version: "3.7.2-22592", IsManager: true}, nil
}

func (c *fakeOfficeClient) OfficeSystemSettings(context.Context) (synology.OfficeSystemSettings, error) {
	return c.system, c.systemErr
}

func (c *fakeOfficeClient) OfficePreferences(context.Context) (synology.OfficePreferences, error) {
	return c.preferences, nil
}

func (c *fakeOfficeClient) OfficeFonts(context.Context) ([]synology.OfficeFont, error) {
	return []synology.OfficeFont{{Name: "Arial"}}, nil
}

func (c *fakeOfficeClient) OfficeCapabilities(context.Context) (synology.OfficeCapabilities, synology.CompatibilityReport, error) {
	return c.capabilities, synology.CompatibilityReport{}, nil
}

func (c *fakeOfficeClient) ApplyOfficeSystemChange(_ context.Context, change office.SystemChange) (synology.OfficeMutationResult, error) {
	c.systemSets = append(c.systemSets, change)
	c.applySystem(change)
	return synology.OfficeMutationResult{}, nil
}

func (c *fakeOfficeClient) ApplyOfficePreferencesChange(_ context.Context, change office.PreferencesChange) (synology.OfficeMutationResult, error) {
	c.prefSets = append(c.prefSets, change)
	c.applyPrefs(change)
	return synology.OfficeMutationResult{}, nil
}

func TestValidateOfficeChangeRequiresExactlyOneScope(t *testing.T) {
	if err := validateOfficeChange(office.Change{}); err == nil {
		t.Fatal("validateOfficeChange() accepted an empty change")
	}
	both := office.Change{
		System:      &office.SystemChange{HistoryPrune: boolPointer(true)},
		Preferences: &office.PreferencesChange{Ruler: boolPointer(true)},
	}
	if err := validateOfficeChange(both); err == nil {
		t.Fatal("validateOfficeChange() accepted a change with both scopes")
	}
	if err := validateOfficeChange(office.Change{System: &office.SystemChange{}}); err == nil {
		t.Fatal("validateOfficeChange() accepted an empty system patch")
	}
	if err := validateOfficeChange(office.Change{Preferences: &office.PreferencesChange{}}); err == nil {
		t.Fatal("validateOfficeChange() accepted an empty preferences patch")
	}
	if err := validateOfficeChange(office.Change{System: &office.SystemChange{HistoryPrune: boolPointer(true)}}); err != nil {
		t.Fatalf("validateOfficeChange() rejected a valid system patch: %v", err)
	}
}

func TestPlanOfficeChangeSystemScopeWarnsOnEnablingPrune(t *testing.T) {
	client := newFakeOfficeClient()
	request := office.Change{System: &office.SystemChange{HistoryPrune: boolPointer(true)}}

	plan, err := planOfficeChangeWithClient(context.Background(), "lab", client, request)
	if err != nil {
		t.Fatalf("planOfficeChangeWithClient() error = %v", err)
	}
	if plan.Risk != "high" || len(plan.Warnings) != 1 {
		t.Fatalf("plan risk = %q, warnings = %v", plan.Risk, plan.Warnings)
	}
	if plan.Observed.System == nil || plan.Observed.Preferences != nil {
		t.Fatalf("plan observed the wrong scope: %#v", plan.Observed)
	}
}

func TestPlanOfficeChangeRejectsNoOpPatch(t *testing.T) {
	client := newFakeOfficeClient()
	request := office.Change{Preferences: &office.PreferencesChange{Ruler: boolPointer(true)}}

	if _, err := planOfficeChangeWithClient(context.Background(), "lab", client, request); err == nil ||
		!strings.Contains(err.Error(), "would not change") {
		t.Fatalf("planOfficeChangeWithClient() no-op error = %v", err)
	}
}

func TestApplyOfficePlanRejectsStaleState(t *testing.T) {
	client := newFakeOfficeClient()
	request := office.Change{System: &office.SystemChange{HistoryPrune: boolPointer(true)}}
	plan, err := planOfficeChangeWithClient(context.Background(), "lab", client, request)
	if err != nil {
		t.Fatalf("planOfficeChangeWithClient() error = %v", err)
	}

	// Another manager changes an unrelated preference: still fresh. A system
	// change between plan and apply must be stale.
	client.system.HistoryPrune = true
	if _, err := applyOfficePlanWithClient(context.Background(), client, plan); err == nil {
		t.Fatal("applyOfficePlanWithClient() accepted a stale plan")
	}
	if len(client.systemSets) != 0 {
		t.Fatalf("stale apply still mutated DSM: %#v", client.systemSets)
	}
}

func TestApplyOfficePlanAppliesAndVerifiesPreferences(t *testing.T) {
	client := newFakeOfficeClient()
	request := office.Change{Preferences: &office.PreferencesChange{Ruler: boolPointer(false)}}
	plan, err := planOfficeChangeWithClient(context.Background(), "lab", client, request)
	if err != nil {
		t.Fatalf("planOfficeChangeWithClient() error = %v", err)
	}

	result, err := applyOfficePlanWithClient(context.Background(), client, plan)
	if err != nil {
		t.Fatalf("applyOfficePlanWithClient() error = %v", err)
	}
	if !result.Applied || len(client.prefSets) != 1 || client.preferences.Ruler {
		t.Fatalf("apply result = %#v, sets = %#v", result, client.prefSets)
	}
}

func TestApplyOfficePlanFailsWhenPostconditionDoesNotHold(t *testing.T) {
	client := newFakeOfficeClient()
	// DSM accepts the set but silently does not change the value.
	client.applySystem = func(office.SystemChange) {}
	request := office.Change{System: &office.SystemChange{HistoryPrune: boolPointer(true)}}
	plan, err := planOfficeChangeWithClient(context.Background(), "lab", client, request)
	if err != nil {
		t.Fatalf("planOfficeChangeWithClient() error = %v", err)
	}

	if _, err := applyOfficePlanWithClient(context.Background(), client, plan); err == nil ||
		!strings.Contains(err.Error(), "do not match the approved patch") {
		t.Fatalf("applyOfficePlanWithClient() postcondition error = %v", err)
	}
}
