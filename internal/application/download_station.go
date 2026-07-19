package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/downloadstation"
	"github.com/ychiu1211/dsmctl/internal/synology"
)

const downloadStationAPIVersion = "dsmctl.io/v1alpha1"

type DownloadStationCapabilitiesResult struct {
	NAS          string                               `json:"nas" jsonschema:"NAS profile used for the request"`
	Capabilities synology.DownloadStationCapabilities `json:"capabilities" jsonschema:"Download Station reads currently exposed by dsmctl"`
	Report       synology.CompatibilityReport         `json:"report" jsonschema:"Discovered APIs and selected Download Station backends"`
}

type DownloadStationServiceResult struct {
	NAS     string                               `json:"nas" jsonschema:"NAS profile used for the request"`
	Service synology.DownloadStationServiceState `json:"service" jsonschema:"Normalized Download Station service configuration"`
}

type DownloadStationTasksResult struct {
	NAS   string                        `json:"nas" jsonschema:"NAS profile used for the request"`
	Tasks synology.DownloadStationTasks `json:"tasks" jsonschema:"Download task list"`
}

type DownloadStationStatisticsResult struct {
	NAS        string                             `json:"nas" jsonschema:"NAS profile used for the request"`
	Statistics synology.DownloadStationStatistics `json:"statistics" jsonschema:"Aggregate transfer statistics"`
}

type DownloadStationSettingsResult struct {
	NAS      string                           `json:"nas" jsonschema:"NAS profile used for the request"`
	Settings synology.DownloadStationSettings `json:"settings" jsonschema:"Full detailed Download Station configuration"`
}

func (s *Service) GetDownloadStationCapabilities(ctx context.Context, requestedNAS string) (DownloadStationCapabilitiesResult, error) {
	name, client, err := s.manager.Client(ctx, requestedNAS)
	if err != nil {
		return DownloadStationCapabilitiesResult{}, err
	}
	capabilities, report, err := client.DownloadStationCapabilities(ctx)
	if err != nil {
		return DownloadStationCapabilitiesResult{}, authenticationError(name, err)
	}
	return DownloadStationCapabilitiesResult{NAS: name, Capabilities: capabilities, Report: report}, nil
}

func (s *Service) GetDownloadStationService(ctx context.Context, requestedNAS string) (DownloadStationServiceResult, error) {
	name, client, err := s.manager.Client(ctx, requestedNAS)
	if err != nil {
		return DownloadStationServiceResult{}, err
	}
	state, err := client.DownloadStationServiceState(ctx)
	if err != nil {
		return DownloadStationServiceResult{}, authenticationError(name, err)
	}
	return DownloadStationServiceResult{NAS: name, Service: state}, nil
}

func (s *Service) GetDownloadStationTasks(ctx context.Context, requestedNAS string) (DownloadStationTasksResult, error) {
	name, client, err := s.manager.Client(ctx, requestedNAS)
	if err != nil {
		return DownloadStationTasksResult{}, err
	}
	tasks, err := client.DownloadStationTasks(ctx)
	if err != nil {
		return DownloadStationTasksResult{}, authenticationError(name, err)
	}
	return DownloadStationTasksResult{NAS: name, Tasks: tasks}, nil
}

func (s *Service) GetDownloadStationStatistics(ctx context.Context, requestedNAS string) (DownloadStationStatisticsResult, error) {
	name, client, err := s.manager.Client(ctx, requestedNAS)
	if err != nil {
		return DownloadStationStatisticsResult{}, err
	}
	stats, err := client.DownloadStationStatistics(ctx)
	if err != nil {
		return DownloadStationStatisticsResult{}, authenticationError(name, err)
	}
	return DownloadStationStatisticsResult{NAS: name, Statistics: stats}, nil
}

func (s *Service) GetDownloadStationSettings(ctx context.Context, requestedNAS string) (DownloadStationSettingsResult, error) {
	name, client, err := s.manager.Client(ctx, requestedNAS)
	if err != nil {
		return DownloadStationSettingsResult{}, err
	}
	settings, err := client.DownloadStationSettings(ctx)
	if err != nil {
		return DownloadStationSettingsResult{}, authenticationError(name, err)
	}
	return DownloadStationSettingsResult{NAS: name, Settings: settings}, nil
}

// DownloadStationTaskSummary is a stable-field projection of a target task,
// bound into a task plan so an apply can detect a target that has since
// disappeared without binding to volatile transfer progress.
type DownloadStationTaskSummary struct {
	ID    string `json:"id" jsonschema:"Task identifier"`
	Title string `json:"title,omitempty" jsonschema:"Task title"`
	Type  string `json:"type,omitempty" jsonschema:"Download protocol"`
}

type DownloadStationTaskPlan struct {
	APIVersion          string                       `json:"api_version" jsonschema:"Plan schema version"`
	NAS                 string                       `json:"nas" jsonschema:"NAS profile selected during planning"`
	ProfileRevision     uint64                       `json:"profile_revision,omitempty" jsonschema:"Persistent gateway profile revision selected during planning"`
	Request             downloadstation.TaskChange   `json:"request" jsonschema:"Validated task mutation intent"`
	Observed            []DownloadStationTaskSummary `json:"observed" jsonschema:"Target tasks observed during planning (control actions); empty for create"`
	ObservedFingerprint string                       `json:"observed_fingerprint" jsonschema:"SHA-256 hash of the observed target tasks"`
	Risk                string                       `json:"risk" jsonschema:"Plan risk level"`
	Warnings            []string                     `json:"warnings" jsonschema:"Operational warnings"`
	Summary             []string                     `json:"summary" jsonschema:"Human-readable operations"`
	Hash                string                       `json:"hash" jsonschema:"SHA-256 approval hash covering intent and observed targets"`
}

type DownloadStationTaskApplyResult struct {
	NAS      string                                     `json:"nas" jsonschema:"NAS profile used for apply"`
	PlanHash string                                     `json:"plan_hash" jsonschema:"Approved plan hash"`
	Applied  bool                                       `json:"applied" jsonschema:"Whether DSM accepted the change and postcondition verification passed"`
	Result   synology.DownloadStationTaskMutationResult `json:"result" jsonschema:"Selected DSM mutation backend and affected task ids"`
}

type downloadStationTaskClient interface {
	DownloadStationTasks(context.Context) (synology.DownloadStationTasks, error)
	DownloadStationCapabilities(context.Context) (synology.DownloadStationCapabilities, synology.CompatibilityReport, error)
	ApplyDownloadStationTaskChange(context.Context, synology.DownloadStationTaskChange) (synology.DownloadStationTaskMutationResult, error)
}

func (s *Service) downloadStationTaskClient(ctx context.Context, requestedNAS string) (string, downloadStationTaskClient, error) {
	name, generic, err := s.manager.Client(ctx, requestedNAS)
	if err != nil {
		return "", nil, err
	}
	client, ok := generic.(downloadStationTaskClient)
	if !ok {
		return "", nil, fmt.Errorf("NAS client does not implement Download Station task management")
	}
	return name, client, nil
}

func (s *Service) PlanDownloadStationTaskChange(ctx context.Context, requestedNAS string, request downloadstation.TaskChange) (DownloadStationTaskPlan, error) {
	if err := validateTaskChangeShape(request); err != nil {
		return DownloadStationTaskPlan{}, err
	}
	name, client, err := s.downloadStationTaskClient(ctx, requestedNAS)
	if err != nil {
		return DownloadStationTaskPlan{}, err
	}
	plan, err := planDownloadStationTaskWithClient(ctx, name, client, request)
	if err != nil {
		return DownloadStationTaskPlan{}, err
	}
	plan.ProfileRevision, err = s.profileRevision(ctx, name)
	if err == nil {
		plan.Hash, err = downloadStationTaskPlanHash(plan)
	}
	return plan, err
}

func (s *Service) ApplyDownloadStationTaskPlan(ctx context.Context, plan DownloadStationTaskPlan, approvalHash string) (DownloadStationTaskApplyResult, error) {
	if strings.TrimSpace(approvalHash) == "" || approvalHash != plan.Hash {
		return DownloadStationTaskApplyResult{}, fmt.Errorf("approval hash does not match the task plan")
	}
	if plan.APIVersion != downloadStationAPIVersion || strings.TrimSpace(plan.NAS) == "" {
		return DownloadStationTaskApplyResult{}, fmt.Errorf("invalid task plan metadata")
	}
	if err := validateTaskChangeShape(plan.Request); err != nil {
		return DownloadStationTaskApplyResult{}, err
	}
	expectedHash, err := downloadStationTaskPlanHash(plan)
	if err != nil {
		return DownloadStationTaskApplyResult{}, err
	}
	if expectedHash != plan.Hash {
		return DownloadStationTaskApplyResult{}, fmt.Errorf("task plan contents were modified after planning")
	}
	if err := s.authorizeRemoteApply(ctx, plan.NAS, plan.ProfileRevision, plan.Hash, plan.Risk); err != nil {
		return DownloadStationTaskApplyResult{}, err
	}
	if err := s.verifyProfileRevision(ctx, plan.NAS, plan.ProfileRevision); err != nil {
		return DownloadStationTaskApplyResult{}, err
	}
	name, client, err := s.downloadStationTaskClient(ctx, plan.NAS)
	if err != nil {
		return DownloadStationTaskApplyResult{}, err
	}
	if name != plan.NAS {
		return DownloadStationTaskApplyResult{}, fmt.Errorf("task plan NAS %q resolved to different profile %q", plan.NAS, name)
	}
	current, err := planDownloadStationTaskWithClient(ctx, plan.NAS, client, plan.Request)
	if err != nil {
		return DownloadStationTaskApplyResult{}, fmt.Errorf("task plan precondition no longer holds: %w", err)
	}
	current.ProfileRevision = plan.ProfileRevision
	current.Hash, err = downloadStationTaskPlanHash(current)
	if err != nil {
		return DownloadStationTaskApplyResult{}, err
	}
	if current.ObservedFingerprint != plan.ObservedFingerprint || current.Hash != plan.Hash {
		return DownloadStationTaskApplyResult{}, fmt.Errorf("task plan is stale; create a new plan")
	}
	result, err := client.ApplyDownloadStationTaskChange(ctx, plan.Request)
	if err != nil {
		return DownloadStationTaskApplyResult{}, authenticationError(plan.NAS, err)
	}
	if err := verifyDownloadStationTaskPostcondition(ctx, client, plan.Request); err != nil {
		return DownloadStationTaskApplyResult{}, fmt.Errorf("verify task change: %w", err)
	}
	return DownloadStationTaskApplyResult{NAS: plan.NAS, PlanHash: plan.Hash, Applied: true, Result: result}, nil
}

func planDownloadStationTaskWithClient(ctx context.Context, nas string, client downloadStationTaskClient, request downloadstation.TaskChange) (DownloadStationTaskPlan, error) {
	capabilities, _, err := client.DownloadStationCapabilities(ctx)
	if err != nil {
		return DownloadStationTaskPlan{}, authenticationError(nas, err)
	}
	if !capabilities.TaskRead || !capabilities.TaskWrite {
		return DownloadStationTaskPlan{}, fmt.Errorf("NAS %q does not expose a verified Download Station task read/write backend", nas)
	}
	plan := DownloadStationTaskPlan{APIVersion: downloadStationAPIVersion, NAS: nas, Request: request, Observed: []DownloadStationTaskSummary{}}
	if request.Action != downloadstation.TaskActionCreate {
		tasks, err := client.DownloadStationTasks(ctx)
		if err != nil {
			return DownloadStationTaskPlan{}, authenticationError(nas, err)
		}
		byID := make(map[string]synology.DownloadStationTask, len(tasks.Tasks))
		for _, task := range tasks.Tasks {
			byID[task.ID] = task
		}
		observed := make([]DownloadStationTaskSummary, 0, len(request.TaskIDs))
		for _, id := range request.TaskIDs {
			task, ok := byID[id]
			if !ok {
				return DownloadStationTaskPlan{}, fmt.Errorf("task %q was not found on NAS %q", id, nas)
			}
			observed = append(observed, DownloadStationTaskSummary{ID: task.ID, Title: task.Title, Type: task.Type})
		}
		sort.Slice(observed, func(i, j int) bool { return observed[i].ID < observed[j].ID })
		plan.Observed = observed
	}
	plan.ObservedFingerprint, err = hashJSON(plan.Observed)
	if err != nil {
		return DownloadStationTaskPlan{}, err
	}
	plan.Risk, plan.Warnings, plan.Summary = downloadStationTaskEffects(request)
	plan.Hash, err = downloadStationTaskPlanHash(plan)
	if err != nil {
		return DownloadStationTaskPlan{}, err
	}
	return plan, nil
}

// validateTaskChangeShape rejects everything invalid regardless of NAS state.
func validateTaskChangeShape(change downloadstation.TaskChange) error {
	switch change.Action {
	case downloadstation.TaskActionCreate:
		if len(change.URIs) == 0 {
			return fmt.Errorf("a create task requires at least one uri")
		}
		for _, uri := range change.URIs {
			if err := validateDownloadURI(strings.TrimSpace(uri)); err != nil {
				return err
			}
		}
	case downloadstation.TaskActionPause, downloadstation.TaskActionResume, downloadstation.TaskActionDelete:
		if len(change.TaskIDs) == 0 {
			return fmt.Errorf("a %s action requires at least one task_id", change.Action)
		}
		for _, id := range change.TaskIDs {
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("task_id must not be empty")
			}
		}
	default:
		return fmt.Errorf("unsupported task action %q; use create, pause, resume, or delete", change.Action)
	}
	return nil
}

func validateDownloadURI(uri string) error {
	if uri == "" {
		return fmt.Errorf("uri must not be empty")
	}
	for _, scheme := range []string{"http://", "https://", "ftp://", "ftps://", "magnet:"} {
		if strings.HasPrefix(strings.ToLower(uri), scheme) {
			return nil
		}
	}
	return fmt.Errorf("unsupported download uri %q; expected an http(s), ftp(s), or magnet uri", uri)
}

func downloadStationTaskEffects(change downloadstation.TaskChange) (string, []string, []string) {
	switch change.Action {
	case downloadstation.TaskActionCreate:
		return "high",
			[]string{"creating a task makes the NAS fetch external content from the supplied uri(s)"},
			[]string{fmt.Sprintf("create %d download task(s) to %s", len(change.URIs), destinationOrDefault(change.Destination))}
	case downloadstation.TaskActionResume:
		return "high",
			[]string{"resuming restarts downloading, so the NAS fetches external content"},
			[]string{fmt.Sprintf("resume task(s) %s", strings.Join(change.TaskIDs, ", "))}
	case downloadstation.TaskActionDelete:
		warning := "deleting removes the task and its partial data"
		if change.ForceComplete {
			warning = "force_complete marks the task complete and keeps downloaded data instead of removing it"
		}
		return "high", []string{warning}, []string{fmt.Sprintf("delete task(s) %s", strings.Join(change.TaskIDs, ", "))}
	case downloadstation.TaskActionPause:
		return "medium",
			[]string{"pausing stops transfer for the task(s); it is reversible with resume"},
			[]string{fmt.Sprintf("pause task(s) %s", strings.Join(change.TaskIDs, ", "))}
	default:
		return "high", []string{}, []string{}
	}
}

func destinationOrDefault(destination string) string {
	if strings.TrimSpace(destination) == "" {
		return "the DSM default destination"
	}
	return strings.TrimSpace(destination)
}

func verifyDownloadStationTaskPostcondition(ctx context.Context, client downloadStationTaskClient, change downloadstation.TaskChange) error {
	tasks, err := client.DownloadStationTasks(ctx)
	if err != nil {
		return err
	}
	byID := make(map[string]synology.DownloadStationTask, len(tasks.Tasks))
	for _, task := range tasks.Tasks {
		byID[task.ID] = task
	}
	switch change.Action {
	case downloadstation.TaskActionCreate:
		wanted := make(map[string]struct{}, len(change.URIs))
		for _, uri := range change.URIs {
			wanted[strings.TrimSpace(uri)] = struct{}{}
		}
		for _, task := range tasks.Tasks {
			if _, ok := wanted[strings.TrimSpace(task.URI)]; ok {
				return nil
			}
		}
		return fmt.Errorf("no task matching the requested uri(s) is present after create")
	case downloadstation.TaskActionDelete:
		for _, id := range change.TaskIDs {
			if _, ok := byID[id]; ok {
				return fmt.Errorf("task %q is still present after delete", id)
			}
		}
		return nil
	case downloadstation.TaskActionPause:
		for _, id := range change.TaskIDs {
			task, ok := byID[id]
			if !ok {
				return fmt.Errorf("task %q is missing after pause", id)
			}
			if !strings.EqualFold(task.Status, "paused") {
				return fmt.Errorf("task %q is %q, want paused", id, task.Status)
			}
		}
		return nil
	case downloadstation.TaskActionResume:
		for _, id := range change.TaskIDs {
			task, ok := byID[id]
			if !ok {
				return fmt.Errorf("task %q is missing after resume", id)
			}
			if strings.EqualFold(task.Status, "paused") {
				return fmt.Errorf("task %q is still paused after resume", id)
			}
		}
		return nil
	default:
		return nil
	}
}

func downloadStationTaskPlanHash(plan DownloadStationTaskPlan) (string, error) {
	plan.Hash = ""
	return hashJSON(plan)
}

var _ downloadStationTaskClient = (*synology.Client)(nil)
