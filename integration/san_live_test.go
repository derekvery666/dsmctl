package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ychiu1211/dsmctl/internal/application"
	"github.com/ychiu1211/dsmctl/internal/domain/san"
	"github.com/ychiu1211/dsmctl/internal/domain/storage"
	"github.com/ychiu1211/dsmctl/internal/synology"
)

// TestMCPSANDisposableLUNLive is intentionally gated separately from other
// mutation tests. It is authorized only for one unique, unmapped
// dsmctl-e2e-lun-* LUN created and deleted by this test. It never creates a
// target or mapping and refuses cleanup unless the exact stable UUID and
// unmapped state are verified from a fresh inventory read.
func TestMCPSANDisposableLUNLive(t *testing.T) {
	if os.Getenv("DSMCTL_LIVE_SAN_MUTATIONS") != "1" {
		t.Skip("set DSMCTL_LIVE_SAN_MUTATIONS=1 only after authorizing one disposable unmapped LUN create/delete")
	}
	binary := os.Getenv("DSMCTL_MCP_BINARY")
	nas := os.Getenv("DSMCTL_LIVE_NAS")
	if binary == "" || nas == "" {
		t.Skip("set DSMCTL_MCP_BINARY and DSMCTL_LIVE_NAS to run the live SAN mutation test")
	}

	args := []string{}
	if configPath := os.Getenv("DSMCTL_LIVE_CONFIG"); configPath != "" {
		args = append(args, "--config", configPath)
	}
	command := exec.Command(binary, args...)
	client := mcp.NewClient(&mcp.Implementation{Name: "dsmctl-live-san-test", Version: "0.1.0"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatalf("connect to MCP server: %v", err)
	}
	defer session.Close()

	name := "dsmctl-e2e-lun-" + randomHex(t, 6)
	baseline := liveSANState(t, ctx, session, nas)
	if matches := findLiveLUNs(baseline, name); len(matches) != 0 {
		t.Fatalf("refusing test because unique LUN name %q already has %d match(es)", name, len(matches))
	}
	volume := selectLiveSANVolume(t, liveStorageState(t, ctx, session, nas))

	createAttempted := false
	capturedID := ""
	defer func() {
		if createAttempted {
			cleanupLiveLUN(t, ctx, session, nas, name, capturedID)
		}
	}()

	createPlan := planLiveSAN(t, ctx, session, nas, san.ChangeRequest{
		Action: san.ActionCreate, Resource: san.ResourceLUN,
		LUN: &san.LUNChange{
			Name: name, Description: "temporary dsmctl WI-005 integration LUN",
			BackingVolumeID: volume.ID, SizeBytes: 1 << 30, Provisioning: san.ProvisioningThin,
		},
	})
	createAttempted = true
	createResult := applyLiveSAN(t, ctx, session, createPlan)
	capturedID = createResult.ResourceID
	if capturedID == "" {
		t.Fatalf("create result for %q did not return a stable DSM LUN UUID", name)
	}

	created := liveSANState(t, ctx, session, nas)
	matches := findLiveLUNs(created, name)
	if len(matches) != 1 || matches[0].ID != capturedID || matches[0].Mapped || mappingCountForLiveLUN(created, capturedID) != 0 {
		t.Fatalf("created LUN verification failed: name=%q captured_id=%q matches=%#v mappings=%d", name, capturedID, matches, mappingCountForLiveLUN(created, capturedID))
	}

	cleanupLiveLUN(t, ctx, session, nas, name, capturedID)
	createAttempted = false
	if matches := findLiveLUNs(liveSANState(t, ctx, session, nas), name); len(matches) != 0 {
		t.Fatalf("disposable LUN %q (%s) remains after verified delete", name, capturedID)
	}
	t.Logf("verified disposable LUN create/delete: name=%s stable_id=%s backing_volume=%s", name, capturedID, volume.ID)
}

func cleanupLiveLUN(t *testing.T, ctx context.Context, session *mcp.ClientSession, nas, name, capturedID string) {
	t.Helper()
	current := liveSANState(t, ctx, session, nas)
	cleanupID, exists, err := liveLUNCleanupCandidate(current, name, capturedID)
	if err != nil {
		t.Errorf("cleanup refused: %v", err)
		return
	}
	if !exists {
		return
	}
	if capturedID == "" {
		// A DSM create may succeed even if its response or strict postcondition
		// is lost. The baseline proved this unique name absent, so capture the
		// one unmapped inventory UUID, then require a second fresh exact match
		// before constructing the delete plan.
		capturedID = cleanupID
		current = liveSANState(t, ctx, session, nas)
		cleanupID, exists, err = liveLUNCleanupCandidate(current, name, capturedID)
		if err != nil || !exists {
			t.Errorf("cleanup refused after capturing inventory UUID %q for %q: exists=%t error=%v", capturedID, name, exists, err)
			return
		}
	}
	deletePlan := planLiveSAN(t, ctx, session, nas, san.ChangeRequest{
		Action: san.ActionDelete, Resource: san.ResourceLUN, LUN: &san.LUNChange{ID: cleanupID},
	})
	applyLiveSAN(t, ctx, session, deletePlan)
	if matches := findLiveLUNs(liveSANState(t, ctx, session, nas), name); len(matches) != 0 {
		t.Errorf("cleanup failed: LUN %q stable ID %q remains", name, cleanupID)
	}
}

func liveLUNCleanupCandidate(state san.State, name, capturedID string) (string, bool, error) {
	matches := findLiveLUNs(state, name)
	if len(matches) == 0 {
		return "", false, nil
	}
	if len(matches) != 1 {
		return "", true, fmt.Errorf("LUN name %q has %d matches", name, len(matches))
	}
	lun := matches[0]
	if lun.ID == "" {
		return "", true, fmt.Errorf("LUN %q has no stable UUID", name)
	}
	if capturedID != "" && lun.ID != capturedID {
		return "", true, fmt.Errorf("LUN %q stable ID changed from %q to %q", name, capturedID, lun.ID)
	}
	if lun.Mapped || mappingCountForLiveLUN(state, lun.ID) != 0 {
		return "", true, fmt.Errorf("LUN %q ID=%q mapped=%t mapping_count=%d", name, lun.ID, lun.Mapped, mappingCountForLiveLUN(state, lun.ID))
	}
	return lun.ID, true, nil
}

func TestLiveLUNCleanupCandidateFaultPath(t *testing.T) {
	name := "dsmctl-e2e-lun-fault"
	state := san.State{LUNs: []san.LUN{{ID: "lun-created", Name: name}}}
	id, exists, err := liveLUNCleanupCandidate(state, name, "")
	if err != nil || !exists || id != "lun-created" {
		t.Fatalf("fault-path capture = id %q exists %t error %v", id, exists, err)
	}
	if _, _, err := liveLUNCleanupCandidate(state, name, "different-id"); err == nil {
		t.Fatal("cleanup candidate accepted a stable-ID mismatch")
	}
	state.LUNs[0].Mapped = true
	if _, _, err := liveLUNCleanupCandidate(state, name, "lun-created"); err == nil {
		t.Fatal("cleanup candidate accepted a mapped LUN")
	}
}

func liveSANState(t *testing.T, ctx context.Context, session *mcp.ClientSession, nas string) synology.SANState {
	t.Helper()
	var output struct {
		SAN synology.SANState `json:"san"`
	}
	callLiveTool(t, ctx, session, "get_san_state", map[string]any{"nas": nas}, &output)
	return output.SAN
}

func liveStorageState(t *testing.T, ctx context.Context, session *mcp.ClientSession, nas string) synology.StorageState {
	t.Helper()
	var output struct {
		Storage synology.StorageState `json:"storage"`
	}
	callLiveTool(t, ctx, session, "get_storage_state", map[string]any{"nas": nas}, &output)
	return output.Storage
}

func planLiveSAN(t *testing.T, ctx context.Context, session *mcp.ClientSession, nas string, request san.ChangeRequest) application.SANPlan {
	t.Helper()
	var output struct {
		Plan application.SANPlan `json:"plan"`
	}
	callLiveTool(t, ctx, session, "plan_san_change", map[string]any{"nas": nas, "request": request}, &output)
	return output.Plan
}

func applyLiveSAN(t *testing.T, ctx context.Context, session *mcp.ClientSession, plan application.SANPlan) application.SANApplyResult {
	t.Helper()
	var output struct {
		Result application.SANApplyResult `json:"result"`
	}
	callLiveTool(t, ctx, session, "apply_san_plan", map[string]any{"plan": plan, "approval_hash": plan.Hash}, &output)
	return output.Result
}

func selectLiveSANVolume(t *testing.T, state storage.State) storage.Volume {
	t.Helper()
	for _, volume := range state.Volumes {
		if volume.Path != "" && !volume.ReadOnly && strings.EqualFold(volume.Status, "normal") &&
			(volume.FileSystem == "btrfs" || volume.FileSystem == "ext4") && volume.AvailableBytes >= 1<<30 {
			return volume
		}
	}
	t.Fatal("no normal writable btrfs/ext4 volume with a stable path and at least 1 GiB free was discovered")
	return storage.Volume{}
}

func findLiveLUNs(state san.State, name string) []san.LUN {
	var result []san.LUN
	for _, lun := range state.LUNs {
		if lun.Name == name {
			result = append(result, lun)
		}
	}
	return result
}

func mappingCountForLiveLUN(state san.State, id string) int {
	count := 0
	for _, mapping := range state.Mappings {
		if mapping.LUNID == id {
			count++
		}
	}
	return count
}
