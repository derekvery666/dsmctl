package synology

import (
	"context"
	"fmt"

	"github.com/ychiu1211/dsmctl/internal/domain/san"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
	"github.com/ychiu1211/dsmctl/internal/synology/operations/saninventory"
	"github.com/ychiu1211/dsmctl/internal/synology/operations/sanmutation"
)

type SANState = san.State
type SANCapabilities = san.Capabilities
type SANChangeRequest = san.ChangeRequest
type SANMutationResult = sanmutation.Result

type SANMutationInput struct {
	Request              SANChangeRequest
	BackingVolumePath    string
	BackingFileSystem    string
	NewBackingVolumePath *string
	CHAPPassword         string
	MutualCHAPPassword   string
}

func (c *Client) SANState(ctx context.Context) (SANState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.prepareCompatibilityTargetLocked(ctx, saninventory.APINames()...); err != nil {
		return SANState{}, fmt.Errorf("prepare SAN inventory target: %w", err)
	}
	state, selections, err := saninventory.Execute(ctx, c.target, lockedExecutor{client: c})
	if err != nil {
		return SANState{}, fmt.Errorf("get SAN inventory: %w", err)
	}
	c.addSANCapabilitiesLocked(selections)
	return state, nil
}

func (c *Client) SANCapabilities(ctx context.Context) (SANCapabilities, CompatibilityReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	apiNames := append(saninventory.APINames(), sanmutation.APINames()...)
	if err := c.prepareCompatibilityTargetLocked(ctx, apiNames...); err != nil {
		return SANCapabilities{}, CompatibilityReport{}, fmt.Errorf("prepare SAN capabilities target: %w", err)
	}
	selections, err := saninventory.Select(c.target)
	if err != nil {
		return SANCapabilities{}, CompatibilityReport{}, fmt.Errorf("select SAN inventory backends: %w", err)
	}
	c.addSANCapabilitiesLocked(selections)
	mutationSelections, err := sanmutation.Select(c.target)
	if err != nil {
		return SANCapabilities{}, CompatibilityReport{}, fmt.Errorf("select SAN mutation backends: %w", err)
	}
	c.addSANMutationCapabilitiesLocked(mutationSelections)
	c.updateDerivedCapabilitiesLocked()
	targetRead := saninventory.TargetsSupported(selections)
	lunRead := saninventory.LUNsSupported(selections)
	inventoryRead := saninventory.InventorySupported(selections)
	capabilities := SANCapabilities{
		InventoryRead: inventoryRead,
		TargetRead:    targetRead,
		LUNRead:       lunRead,
		MappingRead:   targetRead,
		TargetCreate:  sanmutation.Supported(mutationSelections, 0),
		TargetUpdate:  sanmutation.Supported(mutationSelections, 1),
		TargetDelete:  sanmutation.Supported(mutationSelections, 2),
		LUNCreate:     sanmutation.Supported(mutationSelections, 3),
		LUNUpdate:     sanmutation.Supported(mutationSelections, 4),
		LUNDelete:     sanmutation.Supported(mutationSelections, 5),
		MappingAttach: sanmutation.Supported(mutationSelections, 6),
		MappingDetach: sanmutation.Supported(mutationSelections, 7),
	}
	capabilities.Mutations = capabilities.TargetCreate || capabilities.TargetUpdate || capabilities.TargetDelete || capabilities.LUNCreate || capabilities.LUNUpdate || capabilities.LUNDelete || capabilities.MappingAttach || capabilities.MappingDetach
	return capabilities, c.target.Report(append(selections, mutationSelections...)...), nil
}

func (c *Client) ApplySANChange(ctx context.Context, input SANMutationInput) (SANMutationResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.prepareCompatibilityTargetLocked(ctx, sanmutation.APINames()...); err != nil {
		return SANMutationResult{}, fmt.Errorf("prepare SAN mutation target: %w", err)
	}
	request := input.Request
	switch request.Resource {
	case san.ResourceTarget:
		change := request.Target
		if change == nil {
			return SANMutationResult{}, fmt.Errorf("SAN target change is required")
		}
		result, selection, err := sanmutation.ExecuteTarget(ctx, c.target, lockedExecutor{client: c}, sanmutation.TargetInput{
			Action: request.Action, ID: change.ID, Name: change.Name, IQN: change.IQN, Authentication: change.Authentication,
			CHAPUser: change.CHAPUser, CHAPPassword: input.CHAPPassword, MutualCHAPUser: change.MutualCHAPUser,
			MutualCHAPPassword: input.MutualCHAPPassword, NewName: change.NewName, NewIQN: change.NewIQN,
			NewAuthentication: change.NewAuthentication, Enabled: change.Enabled,
		})
		if err != nil {
			return SANMutationResult{}, fmt.Errorf("apply SAN target change with %s: %w", selection.Backend, err)
		}
		c.target.AddCapability(selection.Operation)
		return result, nil
	case san.ResourceLUN:
		change := request.LUN
		if change == nil {
			return SANMutationResult{}, fmt.Errorf("SAN LUN change is required")
		}
		result, selection, err := sanmutation.ExecuteLUN(ctx, c.target, lockedExecutor{client: c}, sanmutation.LUNInput{
			Action: request.Action, ID: change.ID, Name: change.Name, Description: change.Description,
			BackingVolumePath: input.BackingVolumePath, BackingFileSystem: input.BackingFileSystem,
			SizeBytes: change.SizeBytes, Provisioning: change.Provisioning, NewName: change.NewName,
			NewDescription: change.NewDescription, NewBackingPath: input.NewBackingVolumePath, NewSizeBytes: change.NewSizeBytes,
		})
		if err != nil {
			return SANMutationResult{}, fmt.Errorf("apply SAN LUN change with %s: %w", selection.Backend, err)
		}
		c.target.AddCapability(selection.Operation)
		return result, nil
	case san.ResourceMapping:
		change := request.Mapping
		if change == nil {
			return SANMutationResult{}, fmt.Errorf("SAN mapping change is required")
		}
		result, selection, err := sanmutation.ExecuteMapping(ctx, c.target, lockedExecutor{client: c}, sanmutation.MappingInput{
			Action: request.Action, TargetID: change.TargetID, LUNID: change.LUNID,
		})
		if err != nil {
			return SANMutationResult{}, fmt.Errorf("apply SAN mapping change with %s: %w", selection.Backend, err)
		}
		c.target.AddCapability(selection.Operation)
		return result, nil
	default:
		return SANMutationResult{}, fmt.Errorf("unsupported SAN resource %q", request.Resource)
	}
}

func (c *Client) addSANCapabilitiesLocked(selections []compatibility.Selection) {
	if saninventory.TargetsSupported(selections) {
		c.target.AddCapability(saninventory.TargetCapabilityName)
		c.target.AddCapability(saninventory.MappingCapabilityName)
	}
	if saninventory.LUNsSupported(selections) {
		c.target.AddCapability(saninventory.LUNCapabilityName)
	}
	if saninventory.InventorySupported(selections) {
		c.target.AddCapability(saninventory.CapabilityName)
	}
}

func (c *Client) addSANMutationCapabilitiesLocked(selections []compatibility.Selection) {
	names := []string{
		sanmutation.TargetCreateCapabilityName, sanmutation.TargetUpdateCapabilityName, sanmutation.TargetDeleteCapabilityName,
		sanmutation.LUNCreateCapabilityName, sanmutation.LUNUpdateCapabilityName, sanmutation.LUNDeleteCapabilityName,
		sanmutation.MappingAttachCapabilityName, sanmutation.MappingDetachCapabilityName,
	}
	for index, name := range names {
		if sanmutation.Supported(selections, index) {
			c.target.AddCapability(name)
		}
	}
}
