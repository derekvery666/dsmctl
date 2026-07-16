// Package sanmutation implements typed, operation-scoped SAN Manager writes.
// It never performs inventory reads or constructs plans.
package sanmutation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/san"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

const (
	TargetAPIName = "SYNO.Core.ISCSI.Target"
	LUNAPIName    = "SYNO.Core.ISCSI.LUN"

	TargetCreateOperationName  = "san.target.create"
	TargetUpdateOperationName  = "san.target.update"
	TargetDeleteOperationName  = "san.target.delete"
	LUNCreateOperationName     = "san.lun.create"
	LUNUpdateOperationName     = "san.lun.update"
	LUNDeleteOperationName     = "san.lun.delete"
	MappingAttachOperationName = "san.mapping.attach"
	MappingDetachOperationName = "san.mapping.detach"

	TargetCreateCapabilityName  = "san.target.create"
	TargetUpdateCapabilityName  = "san.target.update"
	TargetDeleteCapabilityName  = "san.target.delete"
	LUNCreateCapabilityName     = "san.lun.create"
	LUNUpdateCapabilityName     = "san.lun.update"
	LUNDeleteCapabilityName     = "san.lun.delete"
	MappingAttachCapabilityName = "san.mapping.attach"
	MappingDetachCapabilityName = "san.mapping.detach"
)

type TargetInput struct {
	Action             string
	ID                 string
	Name               string
	IQN                string
	Authentication     string
	CHAPUser           string
	CHAPPassword       string
	MutualCHAPUser     string
	MutualCHAPPassword string
	NewName            *string
	NewIQN             *string
	NewAuthentication  *string
	Enabled            *bool
}

type LUNInput struct {
	Action            string
	ID                string
	Name              string
	Description       string
	BackingVolumePath string
	BackingFileSystem string
	SizeBytes         uint64
	Provisioning      string
	NewName           *string
	NewDescription    *string
	NewBackingPath    *string
	NewSizeBytes      *uint64
}

type MappingInput struct {
	Action   string
	TargetID string
	LUNID    string
}

type Result struct {
	ResourceID string `json:"resource_id,omitempty"`
	Operation  string `json:"operation"`
}

var targetCreateOperation = compatibility.Operation[TargetInput, Result]{
	Name: TargetCreateOperationName,
	Variants: []compatibility.Variant[TargetInput, Result]{targetVariant(
		"core-iscsi-target-create-v1", TargetCreateOperationName, "create", 10,
		func(input TargetInput) (map[string]any, []string, error) {
			parameters, encrypted, err := targetAuthenticationParameters(input.Authentication, input.CHAPUser, input.CHAPPassword, input.MutualCHAPUser, input.MutualCHAPPassword)
			if err != nil {
				return nil, nil, err
			}
			parameters["name"] = input.Name
			parameters["iqn"] = input.IQN
			return parameters, encrypted, nil
		}, true,
	)},
}

var targetUpdateOperation = compatibility.Operation[TargetInput, Result]{
	Name: TargetUpdateOperationName,
	Variants: []compatibility.Variant[TargetInput, Result]{
		{
			Name: "core-iscsi-target-update-v1", API: TargetAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(TargetAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input TargetInput) (Result, error) {
				method := "set"
				parameters := map[string]any{"target_id": input.ID}
				var encrypted []string
				if input.Enabled != nil {
					if *input.Enabled {
						method = "enable"
					} else {
						method = "disable"
					}
				} else {
					if input.NewName != nil {
						parameters["name"] = *input.NewName
					}
					if input.NewIQN != nil {
						parameters["iqn"] = *input.NewIQN
					}
					if input.NewAuthentication != nil {
						auth, protected, err := targetAuthenticationParameters(*input.NewAuthentication, input.CHAPUser, input.CHAPPassword, input.MutualCHAPUser, input.MutualCHAPPassword)
						if err != nil {
							return Result{}, err
						}
						for key, value := range auth {
							parameters[key] = value
						}
						encrypted = protected
					}
				}
				if _, err := executor.Execute(ctx, compatibility.Request{API: TargetAPIName, Version: 1, Method: method, JSONParameters: parameters, EncryptedParameters: encrypted}); err != nil {
					return Result{}, fmt.Errorf("call %s.%s v1: %w", TargetAPIName, method, err)
				}
				return Result{ResourceID: input.ID, Operation: TargetUpdateOperationName}, nil
			},
		},
	},
}

var targetDeleteOperation = compatibility.Operation[TargetInput, Result]{
	Name: TargetDeleteOperationName,
	Variants: []compatibility.Variant[TargetInput, Result]{targetVariant(
		"core-iscsi-target-delete-v1", TargetDeleteOperationName, "delete", 10,
		func(input TargetInput) (map[string]any, []string, error) {
			return map[string]any{"target_id": input.ID}, nil, nil
		}, false,
	)},
}

var lunCreateOperation = compatibility.Operation[LUNInput, Result]{
	Name: LUNCreateOperationName,
	Variants: []compatibility.Variant[LUNInput, Result]{lunVariant(
		"core-iscsi-lun-create-v1", LUNCreateOperationName, "create", 10,
		func(input LUNInput) (map[string]any, error) {
			lunType, attributes, err := lunTypeAndAttributes(input.BackingFileSystem, input.Provisioning)
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"name": input.Name, "description": input.Description, "location": input.BackingVolumePath,
				"size": input.SizeBytes, "type": lunType, "dev_attribs": attributes,
			}, nil
		}, true,
	)},
}

var lunUpdateOperation = compatibility.Operation[LUNInput, Result]{
	Name: LUNUpdateOperationName,
	Variants: []compatibility.Variant[LUNInput, Result]{lunVariant(
		"core-iscsi-lun-update-v1", LUNUpdateOperationName, "set", 10,
		func(input LUNInput) (map[string]any, error) {
			parameters := map[string]any{"uuid": input.ID, "is_soft_feas_ignored": false}
			if input.NewName != nil {
				parameters["new_name"] = *input.NewName
			}
			if input.NewDescription != nil {
				parameters["description"] = *input.NewDescription
			}
			if input.NewBackingPath != nil {
				parameters["new_location"] = *input.NewBackingPath
			}
			if input.NewSizeBytes != nil {
				parameters["new_size"] = *input.NewSizeBytes
			}
			return parameters, nil
		}, false,
	)},
}

var lunDeleteOperation = compatibility.Operation[LUNInput, Result]{
	Name: LUNDeleteOperationName,
	Variants: []compatibility.Variant[LUNInput, Result]{lunVariant(
		"core-iscsi-lun-delete-v1", LUNDeleteOperationName, "delete", 10,
		func(input LUNInput) (map[string]any, error) {
			return map[string]any{"uuid": "", "uuids": []string{input.ID}, "is_soft_feas_ignored": false}, nil
		}, false,
	)},
}

var mappingAttachOperation = mappingOperation(MappingAttachOperationName, "core-iscsi-mapping-attach-v1", "map_target")
var mappingDetachOperation = mappingOperation(MappingDetachOperationName, "core-iscsi-mapping-detach-v1", "unmap_target")

func targetVariant(name, operationName, method string, priority int, parameters func(TargetInput) (map[string]any, []string, error), decodeCreate bool) compatibility.Variant[TargetInput, Result] {
	return compatibility.Variant[TargetInput, Result]{
		Name: name, API: TargetAPIName, Version: 1, Priority: priority, Match: compatibility.APIVersion(TargetAPIName, 1),
		Execute: func(ctx context.Context, executor compatibility.Executor, input TargetInput) (Result, error) {
			values, encrypted, err := parameters(input)
			if err != nil {
				return Result{}, err
			}
			data, err := executor.Execute(ctx, compatibility.Request{API: TargetAPIName, Version: 1, Method: method, JSONParameters: values, EncryptedParameters: encrypted})
			if err != nil {
				return Result{}, fmt.Errorf("call %s.%s v1: %w", TargetAPIName, method, err)
			}
			resourceID := input.ID
			if decodeCreate {
				resourceID, err = decodeStableID(data, "target_id")
				if err != nil {
					return Result{}, fmt.Errorf("decode %s.%s result: %w", TargetAPIName, method, err)
				}
			}
			return Result{ResourceID: resourceID, Operation: operationName}, nil
		},
	}
}

func lunVariant(name, operationName, method string, priority int, parameters func(LUNInput) (map[string]any, error), decodeCreate bool) compatibility.Variant[LUNInput, Result] {
	return compatibility.Variant[LUNInput, Result]{
		Name: name, API: LUNAPIName, Version: 1, Priority: priority, Match: compatibility.APIVersion(LUNAPIName, 1),
		Execute: func(ctx context.Context, executor compatibility.Executor, input LUNInput) (Result, error) {
			values, err := parameters(input)
			if err != nil {
				return Result{}, err
			}
			data, err := executor.Execute(ctx, compatibility.Request{API: LUNAPIName, Version: 1, Method: method, JSONParameters: values})
			if err != nil {
				return Result{}, fmt.Errorf("call %s.%s v1: %w", LUNAPIName, method, err)
			}
			resourceID := input.ID
			if decodeCreate {
				resourceID, err = decodeStableID(data, "uuid")
				if err != nil {
					return Result{}, fmt.Errorf("decode %s.%s result: %w", LUNAPIName, method, err)
				}
			}
			return Result{ResourceID: resourceID, Operation: operationName}, nil
		},
	}
}

func mappingOperation(operationName, backend, method string) compatibility.Operation[MappingInput, Result] {
	return compatibility.Operation[MappingInput, Result]{
		Name: operationName,
		Variants: []compatibility.Variant[MappingInput, Result]{
			{
				Name: backend, API: LUNAPIName, Version: 1, Priority: 10, Match: compatibility.APIVersion(LUNAPIName, 1),
				Execute: func(ctx context.Context, executor compatibility.Executor, input MappingInput) (Result, error) {
					_, err := executor.Execute(ctx, compatibility.Request{
						API: LUNAPIName, Version: 1, Method: method,
						JSONParameters: map[string]any{"uuid": input.LUNID, "target_ids": []string{input.TargetID}},
					})
					if err != nil {
						return Result{}, fmt.Errorf("call %s.%s v1: %w", LUNAPIName, method, err)
					}
					return Result{ResourceID: input.TargetID + ":" + input.LUNID, Operation: operationName}, nil
				},
			},
		},
	}
}

func targetAuthenticationParameters(mode, user, password, mutualUser, mutualPassword string) (map[string]any, []string, error) {
	parameters := make(map[string]any)
	switch mode {
	case san.AuthenticationNone:
		parameters["auth_type"] = 0
	case san.AuthenticationCHAP:
		parameters["auth_type"] = 1
		parameters["user"] = user
		parameters["password"] = password
		return parameters, []string{"password"}, nil
	case san.AuthenticationMutualCHAP:
		parameters["auth_type"] = 2
		parameters["user"] = user
		parameters["password"] = password
		parameters["mutual_user"] = mutualUser
		parameters["mutual_password"] = mutualPassword
		return parameters, []string{"password", "mutual_password"}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported target authentication %q", mode)
	}
	return parameters, nil, nil
}

func lunTypeAndAttributes(fileSystem, provisioning string) (string, []map[string]any, error) {
	fileSystem = strings.ToLower(strings.TrimSpace(fileSystem))
	provisioning = strings.ToLower(strings.TrimSpace(provisioning))
	var lunType string
	canSnapshot, blockFeatures := 0, 0
	switch fileSystem {
	case "btrfs":
		canSnapshot, blockFeatures = 1, 1
		if provisioning == san.ProvisioningThin {
			lunType = "BLUN"
		} else if provisioning == san.ProvisioningThick {
			lunType = "BLUN_THICK"
		}
	case "ext4":
		if provisioning == san.ProvisioningThin {
			lunType = "THIN"
		} else if provisioning == san.ProvisioningThick {
			lunType = "FILE"
		}
	default:
		return "", nil, fmt.Errorf("unsupported backing file system %q", fileSystem)
	}
	if lunType == "" {
		return "", nil, fmt.Errorf("unsupported provisioning policy %q", provisioning)
	}
	attributes := []map[string]any{
		{"dev_attrib": "emulate_tpws", "enable": blockFeatures},
		{"dev_attrib": "emulate_caw", "enable": 1},
		{"dev_attrib": "emulate_3pc", "enable": blockFeatures},
		{"dev_attrib": "emulate_tpu", "enable": boolInt(provisioning == san.ProvisioningThin)},
		{"dev_attrib": "can_snapshot", "enable": canSnapshot},
	}
	return lunType, attributes, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func decodeStableID(data json.RawMessage, key string) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var values map[string]any
	if err := decoder.Decode(&values); err != nil {
		return "", err
	}
	value := strings.TrimSpace(fmt.Sprint(values[key]))
	if value == "" || value == "<nil>" {
		return "", fmt.Errorf("response has no stable %s", key)
	}
	return value, nil
}

func APINames() []string {
	set := map[string]struct{}{}
	for _, names := range [][]string{
		targetCreateOperation.APINames(), targetUpdateOperation.APINames(), targetDeleteOperation.APINames(),
		lunCreateOperation.APINames(), lunUpdateOperation.APINames(), lunDeleteOperation.APINames(),
		mappingAttachOperation.APINames(), mappingDetachOperation.APINames(),
	} {
		for _, name := range names {
			set[name] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for name := range set {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func Select(target compatibility.Target) ([]compatibility.Selection, error) {
	selectors := []func(compatibility.Target) (compatibility.Selection, error){
		selectTargetCreate, selectTargetUpdate, selectTargetDelete,
		selectLUNCreate, selectLUNUpdate, selectLUNDelete,
		selectMappingAttach, selectMappingDetach,
	}
	selections := make([]compatibility.Selection, 0, len(selectors))
	for _, selector := range selectors {
		selection, err := selector(target)
		selections = append(selections, selection)
		if err != nil && !compatibility.IsUnsupported(err) {
			return nil, err
		}
	}
	return selections, nil
}

func ExecuteTarget(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input TargetInput) (Result, compatibility.Selection, error) {
	switch input.Action {
	case san.ActionCreate:
		return targetCreateOperation.Run(ctx, target, executor, input)
	case san.ActionUpdate:
		return targetUpdateOperation.Run(ctx, target, executor, input)
	case san.ActionDelete:
		return targetDeleteOperation.Run(ctx, target, executor, input)
	default:
		return Result{}, compatibility.Selection{}, fmt.Errorf("unsupported target action %q", input.Action)
	}
}

func ExecuteLUN(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input LUNInput) (Result, compatibility.Selection, error) {
	switch input.Action {
	case san.ActionCreate:
		return lunCreateOperation.Run(ctx, target, executor, input)
	case san.ActionUpdate:
		return lunUpdateOperation.Run(ctx, target, executor, input)
	case san.ActionDelete:
		return lunDeleteOperation.Run(ctx, target, executor, input)
	default:
		return Result{}, compatibility.Selection{}, fmt.Errorf("unsupported LUN action %q", input.Action)
	}
}

func ExecuteMapping(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input MappingInput) (Result, compatibility.Selection, error) {
	switch input.Action {
	case san.ActionAttach:
		return mappingAttachOperation.Run(ctx, target, executor, input)
	case san.ActionDetach:
		return mappingDetachOperation.Run(ctx, target, executor, input)
	default:
		return Result{}, compatibility.Selection{}, fmt.Errorf("unsupported mapping action %q", input.Action)
	}
}

func selectTargetCreate(target compatibility.Target) (compatibility.Selection, error) {
	return selectOperation(targetCreateOperation, target)
}
func selectTargetUpdate(target compatibility.Target) (compatibility.Selection, error) {
	return selectOperation(targetUpdateOperation, target)
}
func selectTargetDelete(target compatibility.Target) (compatibility.Selection, error) {
	return selectOperation(targetDeleteOperation, target)
}
func selectLUNCreate(target compatibility.Target) (compatibility.Selection, error) {
	return selectOperation(lunCreateOperation, target)
}
func selectLUNUpdate(target compatibility.Target) (compatibility.Selection, error) {
	return selectOperation(lunUpdateOperation, target)
}
func selectLUNDelete(target compatibility.Target) (compatibility.Selection, error) {
	return selectOperation(lunDeleteOperation, target)
}
func selectMappingAttach(target compatibility.Target) (compatibility.Selection, error) {
	return selectOperation(mappingAttachOperation, target)
}
func selectMappingDetach(target compatibility.Target) (compatibility.Selection, error) {
	return selectOperation(mappingDetachOperation, target)
}

func selectOperation[I, O any](operation compatibility.Operation[I, O], target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := operation.Select(target)
	return selection, err
}

func Supported(selections []compatibility.Selection, index int) bool {
	return index >= 0 && index < len(selections) && selections[index].Supported
}
