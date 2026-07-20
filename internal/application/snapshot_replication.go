package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/snapshotreplication"
	"github.com/ychiu1211/dsmctl/internal/synology"
)

const snapshotReplicationAPIVersion = "dsmctl.io/v1alpha1"

type SnapshotReplicationCapabilitiesResult struct {
	NAS          string                                   `json:"nas" jsonschema:"NAS profile used for the request"`
	Capabilities synology.SnapshotReplicationCapabilities `json:"capabilities" jsonschema:"Selected Snapshot Replication operations and package evidence"`
	Report       synology.CompatibilityReport             `json:"report" jsonschema:"Discovered APIs and selected backends"`
}

type SnapshotReplicationStateResult struct {
	NAS     string                                    `json:"nas" jsonschema:"NAS profile used for the request"`
	Package snapshotreplication.PackageEvidence       `json:"package" jsonschema:"Installed SnapshotReplication package evidence"`
	Node    synology.SnapshotReplicationNodeIdentity  `json:"node" jsonschema:"Local replication node identity"`
	Shares  []snapshotreplication.ShareOverview       `json:"shares" jsonschema:"Snapshot overview of every snapshot-capable shared folder"`
}

type SnapshotReplicationShareResult struct {
	NAS       string                                       `json:"nas" jsonschema:"NAS profile used for the request"`
	Config    synology.SnapshotReplicationShareConfig      `json:"config" jsonschema:"Per-share snapshot configuration"`
	Retention synology.SnapshotReplicationRetentionPolicy  `json:"retention" jsonschema:"Snapshot retention policy of the share"`
	Snapshots synology.SnapshotReplicationShareSnapshots   `json:"snapshots" jsonschema:"Snapshot inventory of the share"`
}

type SnapshotReplicationReplicationResult struct {
	NAS       string                              `json:"nas" jsonschema:"NAS profile used for the request"`
	Package   snapshotreplication.PackageEvidence `json:"package" jsonschema:"Installed SnapshotReplication package evidence"`
	Supported bool                                `json:"supported" jsonschema:"Whether replication plans can be read on this NAS"`
	Reason    string                              `json:"reason,omitempty" jsonschema:"Why replication is unavailable when supported is false"`
	Plans     *synology.SnapshotReplicationPlans  `json:"plans,omitempty" jsonschema:"Replication plans when supported"`
}

type SnapshotReplicationLogResult struct {
	NAS string                              `json:"nas" jsonschema:"NAS profile used for the request"`
	Log synology.SnapshotReplicationLogPage `json:"log" jsonschema:"One page of the Snapshot Replication log feed"`
}

// SnapshotReplicationObserved is the state a snapshot plan is bound to: the
// target share's complete snapshot inventory and its snapshot configuration.
type SnapshotReplicationObserved struct {
	Snapshots synology.SnapshotReplicationShareSnapshots `json:"snapshots" jsonschema:"Complete snapshot inventory of the target share at planning time"`
	Config    synology.SnapshotReplicationShareConfig    `json:"config" jsonschema:"Snapshot configuration of the target share at planning time"`
}

type SnapshotReplicationPlan struct {
	APIVersion          string                      `json:"api_version" jsonschema:"Plan schema version"`
	NAS                 string                      `json:"nas" jsonschema:"NAS profile selected during planning"`
	ProfileRevision     uint64                      `json:"profile_revision,omitempty" jsonschema:"Persistent gateway profile revision selected during planning"`
	Request             snapshotreplication.Change  `json:"request" jsonschema:"Validated snapshot change intent"`
	Observed            SnapshotReplicationObserved `json:"observed" jsonschema:"Share snapshot state observed during planning"`
	ObservedFingerprint string                      `json:"observed_fingerprint" jsonschema:"SHA-256 hash of the observed state"`
	Risk                string                      `json:"risk" jsonschema:"Plan risk level: medium or high"`
	Warnings            []string                    `json:"warnings" jsonschema:"Data-loss and exposure warnings"`
	Summary             []string                    `json:"summary" jsonschema:"Human-readable operations the plan will perform"`
	Hash                string                      `json:"hash" jsonschema:"SHA-256 approval hash covering intent and observed state"`
}

type SnapshotReplicationApplyResult struct {
	NAS      string                                     `json:"nas" jsonschema:"NAS profile used for apply"`
	PlanHash string                                     `json:"plan_hash" jsonschema:"Approved plan hash"`
	Applied  bool                                       `json:"applied" jsonschema:"Whether DSM accepted the change and postcondition verification passed"`
	Result   synology.SnapshotReplicationMutationResult `json:"result" jsonschema:"Selected DSM mutation backend; carries the created snapshot time name for create"`
}

type snapshotReplicationClient interface {
	ShareState(context.Context, bool) (synology.ShareState, error)
	SnapshotReplicationShareSnapshots(context.Context, string) (synology.SnapshotReplicationShareSnapshots, error)
	SnapshotReplicationShareConfig(context.Context, string) (synology.SnapshotReplicationShareConfig, error)
	SnapshotReplicationRetention(context.Context, string) (synology.SnapshotReplicationRetentionPolicy, error)
	SnapshotReplicationLog(context.Context, int, int) (synology.SnapshotReplicationLogPage, error)
	SnapshotReplicationNode(context.Context) (synology.SnapshotReplicationNodeIdentity, error)
	SnapshotReplicationPlans(context.Context) (synology.SnapshotReplicationPlans, error)
	SnapshotReplicationModuleCapabilities(context.Context) (synology.SnapshotReplicationCapabilities, synology.CompatibilityReport, error)
	ApplySnapshotReplicationChange(context.Context, snapshotreplication.Change) (synology.SnapshotReplicationMutationResult, error)
}

func (s *Service) snapshotReplicationClient(ctx context.Context, requestedNAS string) (string, snapshotReplicationClient, error) {
	name, generic, err := s.manager.Client(ctx, requestedNAS)
	if err != nil {
		return "", nil, err
	}
	client, ok := generic.(snapshotReplicationClient)
	if !ok {
		return "", nil, fmt.Errorf("NAS client does not implement Snapshot Replication management")
	}
	return name, client, nil
}

func (s *Service) GetSnapshotReplicationCapabilities(ctx context.Context, requestedNAS string) (SnapshotReplicationCapabilitiesResult, error) {
	name, client, err := s.snapshotReplicationClient(ctx, requestedNAS)
	if err != nil {
		return SnapshotReplicationCapabilitiesResult{}, err
	}
	capabilities, report, err := client.SnapshotReplicationModuleCapabilities(ctx)
	if err != nil {
		return SnapshotReplicationCapabilitiesResult{}, authenticationError(name, err)
	}
	return SnapshotReplicationCapabilitiesResult{NAS: name, Capabilities: capabilities, Report: report}, nil
}

// GetSnapshotReplicationState summarizes snapshots across every
// snapshot-capable shared folder.
func (s *Service) GetSnapshotReplicationState(ctx context.Context, requestedNAS string) (SnapshotReplicationStateResult, error) {
	name, client, err := s.snapshotReplicationClient(ctx, requestedNAS)
	if err != nil {
		return SnapshotReplicationStateResult{}, err
	}
	capabilities, _, err := client.SnapshotReplicationModuleCapabilities(ctx)
	if err != nil {
		return SnapshotReplicationStateResult{}, authenticationError(name, err)
	}
	result := SnapshotReplicationStateResult{NAS: name, Package: capabilities.Package, Shares: []snapshotreplication.ShareOverview{}}
	if capabilities.NodeRead {
		node, err := client.SnapshotReplicationNode(ctx)
		if err != nil {
			return SnapshotReplicationStateResult{}, authenticationError(name, err)
		}
		result.Node = node
	}
	shares, err := client.ShareState(ctx, false)
	if err != nil {
		return SnapshotReplicationStateResult{}, authenticationError(name, err)
	}
	for _, shared := range shares.Shares {
		if !shared.SnapshotSupported {
			continue
		}
		overview := snapshotreplication.ShareOverview{Share: shared.Name, VolumePath: shared.VolumePath}
		snapshots, err := client.SnapshotReplicationShareSnapshots(ctx, shared.Name)
		if err != nil {
			return SnapshotReplicationStateResult{}, authenticationError(name, err)
		}
		overview.Total = snapshots.Total
		if latest := latestSnapshotTime(snapshots.Snapshots); latest != "" {
			overview.Latest = latest
		}
		config, err := client.SnapshotReplicationShareConfig(ctx, shared.Name)
		if err != nil {
			return SnapshotReplicationStateResult{}, authenticationError(name, err)
		}
		overview.SnapshotBrowsing = config.SnapshotBrowsing
		retention, err := client.SnapshotReplicationRetention(ctx, shared.Name)
		if err != nil {
			return SnapshotReplicationStateResult{}, authenticationError(name, err)
		}
		overview.RetentionTask = retention.TaskID >= 0
		result.Shares = append(result.Shares, overview)
	}
	sort.Slice(result.Shares, func(i, j int) bool { return result.Shares[i].Share < result.Shares[j].Share })
	return result, nil
}

// latestSnapshotTime picks the lexically greatest snapshot time name; DSM's
// GMT-stamped names sort chronologically.
func latestSnapshotTime(snapshots []snapshotreplication.Snapshot) string {
	latest := ""
	for _, snapshot := range snapshots {
		if snapshot.Time > latest {
			latest = snapshot.Time
		}
	}
	return latest
}

// GetSnapshotReplicationShare reads one share's snapshots, configuration, and
// retention policy.
func (s *Service) GetSnapshotReplicationShare(ctx context.Context, requestedNAS, share string) (SnapshotReplicationShareResult, error) {
	if strings.TrimSpace(share) == "" {
		return SnapshotReplicationShareResult{}, fmt.Errorf("share name is required")
	}
	name, client, err := s.snapshotReplicationClient(ctx, requestedNAS)
	if err != nil {
		return SnapshotReplicationShareResult{}, err
	}
	snapshots, err := client.SnapshotReplicationShareSnapshots(ctx, share)
	if err != nil {
		return SnapshotReplicationShareResult{}, authenticationError(name, err)
	}
	config, err := client.SnapshotReplicationShareConfig(ctx, share)
	if err != nil {
		return SnapshotReplicationShareResult{}, authenticationError(name, err)
	}
	retention, err := client.SnapshotReplicationRetention(ctx, share)
	if err != nil {
		return SnapshotReplicationShareResult{}, authenticationError(name, err)
	}
	return SnapshotReplicationShareResult{NAS: name, Config: config, Retention: retention, Snapshots: snapshots}, nil
}

// GetSnapshotReplicationReplication reports replication plans, or why they are
// unavailable (the package gate fails closed rather than erroring).
func (s *Service) GetSnapshotReplicationReplication(ctx context.Context, requestedNAS string) (SnapshotReplicationReplicationResult, error) {
	name, client, err := s.snapshotReplicationClient(ctx, requestedNAS)
	if err != nil {
		return SnapshotReplicationReplicationResult{}, err
	}
	capabilities, _, err := client.SnapshotReplicationModuleCapabilities(ctx)
	if err != nil {
		return SnapshotReplicationReplicationResult{}, authenticationError(name, err)
	}
	result := SnapshotReplicationReplicationResult{NAS: name, Package: capabilities.Package}
	if !capabilities.ReplicationRead {
		result.Reason = "replication requires the SnapshotReplication package; it is not installed or exposes no verified backend"
		if capabilities.Package.Installed && !capabilities.Package.Running {
			result.Reason = "the SnapshotReplication package is installed but not running; start it with a package lifecycle plan"
		}
		return result, nil
	}
	plans, err := client.SnapshotReplicationPlans(ctx)
	if err != nil {
		return SnapshotReplicationReplicationResult{}, authenticationError(name, err)
	}
	result.Supported = true
	result.Plans = &plans
	return result, nil
}

func (s *Service) GetSnapshotReplicationLog(ctx context.Context, requestedNAS string, offset, limit int) (SnapshotReplicationLogResult, error) {
	if offset < 0 {
		return SnapshotReplicationLogResult{}, fmt.Errorf("offset cannot be negative")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		return SnapshotReplicationLogResult{}, fmt.Errorf("limit cannot exceed 1000")
	}
	name, client, err := s.snapshotReplicationClient(ctx, requestedNAS)
	if err != nil {
		return SnapshotReplicationLogResult{}, err
	}
	page, err := client.SnapshotReplicationLog(ctx, offset, limit)
	if err != nil {
		return SnapshotReplicationLogResult{}, authenticationError(name, err)
	}
	return SnapshotReplicationLogResult{NAS: name, Log: page}, nil
}

func (s *Service) PlanSnapshotReplicationChange(ctx context.Context, requestedNAS string, request snapshotreplication.Change) (SnapshotReplicationPlan, error) {
	if err := validateSnapshotReplicationChange(request); err != nil {
		return SnapshotReplicationPlan{}, err
	}
	name, client, err := s.snapshotReplicationClient(ctx, requestedNAS)
	if err != nil {
		return SnapshotReplicationPlan{}, err
	}
	plan, err := planSnapshotReplicationChangeWithClient(ctx, name, client, request)
	if err != nil {
		return SnapshotReplicationPlan{}, err
	}
	plan.ProfileRevision, err = s.profileRevision(ctx, name)
	if err == nil {
		plan.Hash, err = snapshotReplicationPlanHash(plan)
	}
	return plan, err
}

func (s *Service) ApplySnapshotReplicationPlan(ctx context.Context, plan SnapshotReplicationPlan, approvalHash string) (SnapshotReplicationApplyResult, error) {
	if err := validateSnapshotReplicationPlan(plan, approvalHash); err != nil {
		return SnapshotReplicationApplyResult{}, err
	}
	if err := s.authorizeRemoteApply(ctx, plan.NAS, plan.ProfileRevision, plan.Hash, plan.Risk); err != nil {
		return SnapshotReplicationApplyResult{}, err
	}
	if err := s.verifyProfileRevision(ctx, plan.NAS, plan.ProfileRevision); err != nil {
		return SnapshotReplicationApplyResult{}, err
	}
	name, client, err := s.snapshotReplicationClient(ctx, plan.NAS)
	if err != nil {
		return SnapshotReplicationApplyResult{}, err
	}
	if name != plan.NAS {
		return SnapshotReplicationApplyResult{}, fmt.Errorf("snapshot plan NAS %q resolved to different profile %q", plan.NAS, name)
	}
	return applySnapshotReplicationPlanWithClient(ctx, client, plan)
}

func applySnapshotReplicationPlanWithClient(ctx context.Context, client snapshotReplicationClient, plan SnapshotReplicationPlan) (SnapshotReplicationApplyResult, error) {
	current, err := planSnapshotReplicationChangeWithClient(ctx, plan.NAS, client, plan.Request)
	if err != nil {
		return SnapshotReplicationApplyResult{}, fmt.Errorf("snapshot plan precondition no longer holds: %w", err)
	}
	current.ProfileRevision = plan.ProfileRevision
	current.Hash, err = snapshotReplicationPlanHash(current)
	if err != nil {
		return SnapshotReplicationApplyResult{}, err
	}
	if current.ObservedFingerprint != plan.ObservedFingerprint || current.Hash != plan.Hash {
		return SnapshotReplicationApplyResult{}, fmt.Errorf("snapshot plan is stale; create a new plan")
	}
	result, err := client.ApplySnapshotReplicationChange(ctx, plan.Request)
	if err != nil {
		return SnapshotReplicationApplyResult{}, authenticationError(plan.NAS, err)
	}
	if err := verifySnapshotReplicationPostcondition(ctx, client, plan.Request, result); err != nil {
		return SnapshotReplicationApplyResult{}, err
	}
	return SnapshotReplicationApplyResult{NAS: plan.NAS, PlanHash: plan.Hash, Applied: true, Result: result}, nil
}

func verifySnapshotReplicationPostcondition(ctx context.Context, client snapshotReplicationClient, request snapshotreplication.Change, result synology.SnapshotReplicationMutationResult) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("verify snapshot %s: %w", request.Action, err)
		}
	}()
	switch request.Action {
	case snapshotreplication.ActionSetShareConfig:
		config, readErr := client.SnapshotReplicationShareConfig(ctx, request.Share)
		if readErr != nil {
			return readErr
		}
		if request.SnapshotBrowsing != nil && config.SnapshotBrowsing != *request.SnapshotBrowsing {
			return fmt.Errorf("snapshot browsing does not match the approved change")
		}
		if request.LocalTimeFormat != nil && config.LocalTimeFormat != *request.LocalTimeFormat {
			return fmt.Errorf("local time format does not match the approved change")
		}
		return nil
	}
	snapshots, readErr := client.SnapshotReplicationShareSnapshots(ctx, request.Share)
	if readErr != nil {
		return readErr
	}
	byTime := make(map[string]snapshotreplication.Snapshot, len(snapshots.Snapshots))
	for _, snapshot := range snapshots.Snapshots {
		byTime[snapshot.Time] = snapshot
	}
	switch request.Action {
	case snapshotreplication.ActionCreate:
		if result.Snapshot == "" {
			return fmt.Errorf("DSM returned no snapshot time name")
		}
		if _, exists := byTime[result.Snapshot]; !exists {
			return fmt.Errorf("created snapshot %q is not listed", result.Snapshot)
		}
	case snapshotreplication.ActionSetAttributes:
		snapshot, exists := byTime[request.Snapshot]
		if !exists {
			return fmt.Errorf("snapshot %q is no longer listed", request.Snapshot)
		}
		if request.Description != nil && snapshot.Description != *request.Description {
			return fmt.Errorf("description does not match the approved change")
		}
		if request.Lock != nil && snapshot.Locked != *request.Lock {
			return fmt.Errorf("lock state does not match the approved change")
		}
	case snapshotreplication.ActionDelete:
		for _, target := range request.Snapshots {
			if _, exists := byTime[target]; exists {
				return fmt.Errorf("snapshot %q is still listed after delete", target)
			}
		}
	}
	return nil
}

func planSnapshotReplicationChangeWithClient(ctx context.Context, nas string, client snapshotReplicationClient, request snapshotreplication.Change) (SnapshotReplicationPlan, error) {
	capabilities, _, err := client.SnapshotReplicationModuleCapabilities(ctx)
	if err != nil {
		return SnapshotReplicationPlan{}, authenticationError(nas, err)
	}
	if err := snapshotReplicationActionSupported(capabilities, request.Action); err != nil {
		return SnapshotReplicationPlan{}, fmt.Errorf("NAS %q: %w", nas, err)
	}
	snapshots, err := client.SnapshotReplicationShareSnapshots(ctx, request.Share)
	if err != nil {
		return SnapshotReplicationPlan{}, authenticationError(nas, err)
	}
	config, err := client.SnapshotReplicationShareConfig(ctx, request.Share)
	if err != nil {
		return SnapshotReplicationPlan{}, authenticationError(nas, err)
	}
	observed := SnapshotReplicationObserved{Snapshots: snapshots, Config: config}
	if err := validateSnapshotReplicationChangeAgainstObserved(request, observed); err != nil {
		return SnapshotReplicationPlan{}, err
	}
	plan := SnapshotReplicationPlan{
		APIVersion: snapshotReplicationAPIVersion,
		NAS:        nas,
		Request:    request,
		Observed:   observed,
	}
	plan.ObservedFingerprint, err = hashJSON(plan.Observed)
	if err != nil {
		return SnapshotReplicationPlan{}, err
	}
	plan.Risk, plan.Warnings, plan.Summary = snapshotReplicationPlanEffects(request, observed)
	plan.Hash, err = snapshotReplicationPlanHash(plan)
	if err != nil {
		return SnapshotReplicationPlan{}, err
	}
	return plan, nil
}

func snapshotReplicationActionSupported(capabilities synology.SnapshotReplicationCapabilities, action string) error {
	switch action {
	case snapshotreplication.ActionCreate:
		if !capabilities.SnapshotCreate {
			return fmt.Errorf("snapshot create is not supported by a verified backend")
		}
	case snapshotreplication.ActionSetAttributes:
		if !capabilities.SnapshotSetAttributes {
			return fmt.Errorf("snapshot attribute edit is not supported by a verified backend")
		}
	case snapshotreplication.ActionDelete:
		if !capabilities.SnapshotDelete {
			return fmt.Errorf("snapshot delete is not supported by a verified backend")
		}
	case snapshotreplication.ActionSetShareConfig:
		if !capabilities.ShareConfigSet {
			return fmt.Errorf("share snapshot configuration write is not supported by a verified backend")
		}
	}
	if !capabilities.SnapshotsRead || !capabilities.ShareConfigRead {
		return fmt.Errorf("snapshot reads are not supported by a verified backend, so plans cannot bind observed state")
	}
	return nil
}

func validateSnapshotReplicationChange(change snapshotreplication.Change) error {
	if strings.TrimSpace(change.Share) == "" {
		return fmt.Errorf("share is required")
	}
	switch change.Action {
	case snapshotreplication.ActionCreate:
		if change.Snapshot != "" || len(change.Snapshots) != 0 {
			return fmt.Errorf("create does not take snapshot targets")
		}
		if change.SnapshotBrowsing != nil || change.LocalTimeFormat != nil {
			return fmt.Errorf("create does not take share configuration fields")
		}
	case snapshotreplication.ActionSetAttributes:
		if strings.TrimSpace(change.Snapshot) == "" {
			return fmt.Errorf("set_attributes requires the snapshot time name")
		}
		if change.Description == nil && change.Lock == nil {
			return fmt.Errorf("set_attributes requires description or lock")
		}
		if change.SnapshotBrowsing != nil || change.LocalTimeFormat != nil {
			return fmt.Errorf("set_attributes does not take share configuration fields")
		}
	case snapshotreplication.ActionDelete:
		if len(change.Snapshots) == 0 {
			return fmt.Errorf("delete requires at least one snapshot time name")
		}
		seen := make(map[string]struct{}, len(change.Snapshots))
		for _, target := range change.Snapshots {
			if strings.TrimSpace(target) == "" {
				return fmt.Errorf("delete targets cannot be empty")
			}
			if _, duplicate := seen[target]; duplicate {
				return fmt.Errorf("delete target %q appears more than once", target)
			}
			seen[target] = struct{}{}
		}
		if change.Description != nil || change.Lock != nil || change.SnapshotBrowsing != nil || change.LocalTimeFormat != nil {
			return fmt.Errorf("delete takes only snapshot targets")
		}
	case snapshotreplication.ActionSetShareConfig:
		if change.SnapshotBrowsing == nil && change.LocalTimeFormat == nil {
			return fmt.Errorf("set_share_config requires snapshot_browsing or local_time_format")
		}
		if change.Snapshot != "" || len(change.Snapshots) != 0 || change.Description != nil || change.Lock != nil {
			return fmt.Errorf("set_share_config takes only share configuration fields")
		}
	default:
		return fmt.Errorf("unsupported action %q", change.Action)
	}
	return nil
}

func validateSnapshotReplicationChangeAgainstObserved(change snapshotreplication.Change, observed SnapshotReplicationObserved) error {
	byTime := make(map[string]snapshotreplication.Snapshot, len(observed.Snapshots.Snapshots))
	for _, snapshot := range observed.Snapshots.Snapshots {
		byTime[snapshot.Time] = snapshot
	}
	switch change.Action {
	case snapshotreplication.ActionSetAttributes:
		snapshot, exists := byTime[change.Snapshot]
		if !exists {
			return fmt.Errorf("snapshot %q does not exist on share %q", change.Snapshot, change.Share)
		}
		if (change.Description == nil || snapshot.Description == *change.Description) &&
			(change.Lock == nil || snapshot.Locked == *change.Lock) {
			return fmt.Errorf("snapshot attribute patch would not change the current state")
		}
	case snapshotreplication.ActionDelete:
		for _, target := range change.Snapshots {
			if _, exists := byTime[target]; !exists {
				return fmt.Errorf("snapshot %q does not exist on share %q", target, change.Share)
			}
		}
	case snapshotreplication.ActionSetShareConfig:
		if (change.SnapshotBrowsing == nil || observed.Config.SnapshotBrowsing == *change.SnapshotBrowsing) &&
			(change.LocalTimeFormat == nil || observed.Config.LocalTimeFormat == *change.LocalTimeFormat) {
			return fmt.Errorf("share configuration patch would not change the current state")
		}
	}
	return nil
}

func snapshotReplicationPlanEffects(change snapshotreplication.Change, observed SnapshotReplicationObserved) (string, []string, []string) {
	risk := "medium"
	warnings := []string{}
	summary := []string{}
	switch change.Action {
	case snapshotreplication.ActionCreate:
		description := ""
		if change.Description != nil {
			description = fmt.Sprintf(" with description %q", *change.Description)
		}
		lock := " (DSM default lock)"
		if change.Lock != nil {
			lock = fmt.Sprintf(" (locked=%t)", *change.Lock)
		}
		summary = append(summary, fmt.Sprintf("take a snapshot of share %q%s%s", change.Share, description, lock))
	case snapshotreplication.ActionSetAttributes:
		if change.Description != nil {
			summary = append(summary, fmt.Sprintf("set description of snapshot %q on share %q to %q", change.Snapshot, change.Share, *change.Description))
		}
		if change.Lock != nil {
			verb := "unlock"
			if *change.Lock {
				verb = "lock"
			}
			summary = append(summary, fmt.Sprintf("%s snapshot %q on share %q", verb, change.Snapshot, change.Share))
			if !*change.Lock {
				warnings = append(warnings, "an unlocked snapshot becomes eligible for retention pruning")
			}
		}
	case snapshotreplication.ActionDelete:
		risk = "high"
		warnings = append(warnings, "deleting a snapshot permanently discards its point-in-time data; this cannot be undone")
		byTime := make(map[string]snapshotreplication.Snapshot, len(observed.Snapshots.Snapshots))
		for _, snapshot := range observed.Snapshots.Snapshots {
			byTime[snapshot.Time] = snapshot
		}
		for _, target := range change.Snapshots {
			summary = append(summary, fmt.Sprintf("delete snapshot %q of share %q", target, change.Share))
			if snapshot, exists := byTime[target]; exists && snapshot.Locked {
				warnings = append(warnings, fmt.Sprintf("snapshot %q is locked; deleting it discards a protected snapshot", target))
			}
		}
	case snapshotreplication.ActionSetShareConfig:
		if change.SnapshotBrowsing != nil {
			summary = append(summary, fmt.Sprintf("set snapshot browsing of share %q to %t", change.Share, *change.SnapshotBrowsing))
			if *change.SnapshotBrowsing {
				warnings = append(warnings, "snapshot browsing exposes prior file versions to every account with access to the share")
			}
		}
		if change.LocalTimeFormat != nil {
			summary = append(summary, fmt.Sprintf("set local-time snapshot naming of share %q to %t", change.Share, *change.LocalTimeFormat))
		}
	}
	summary = append(summary,
		"re-read the share snapshot state and verify the plan precondition before applying",
		"verify the resulting state after DSM accepts the change")
	return risk, warnings, summary
}

func validateSnapshotReplicationPlan(plan SnapshotReplicationPlan, approvalHash string) error {
	if strings.TrimSpace(approvalHash) == "" || approvalHash != plan.Hash {
		return fmt.Errorf("approval hash does not match the snapshot plan")
	}
	if plan.APIVersion != snapshotReplicationAPIVersion || strings.TrimSpace(plan.NAS) == "" {
		return fmt.Errorf("invalid snapshot plan metadata")
	}
	if err := validateSnapshotReplicationChange(plan.Request); err != nil {
		return err
	}
	expectedFingerprint, err := hashJSON(plan.Observed)
	if err != nil || expectedFingerprint != plan.ObservedFingerprint {
		return fmt.Errorf("snapshot plan observed state was modified")
	}
	expectedHash, err := snapshotReplicationPlanHash(plan)
	if err != nil {
		return err
	}
	if expectedHash != plan.Hash {
		return fmt.Errorf("snapshot plan contents were modified after planning")
	}
	return nil
}

func snapshotReplicationPlanHash(plan SnapshotReplicationPlan) (string, error) {
	plan.Hash = ""
	return hashJSON(plan)
}

var _ snapshotReplicationClient = (*synology.Client)(nil)
