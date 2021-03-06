// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package params

import (
	"time"

	"github.com/juju/utils/exec"
	"github.com/juju/version"

	"github.com/juju/juju/constraints"
	"github.com/juju/juju/instance"
	"github.com/juju/juju/state/multiwatcher"
	"github.com/juju/juju/status"
	"github.com/juju/juju/tools"
)

// MachineContainersParams holds the arguments for making a SetSupportedContainers
// API call.
type MachineContainersParams struct {
	Params []MachineContainers
}

// MachineContainers holds the arguments for making an SetSupportedContainers call
// on a given machine.
type MachineContainers struct {
	MachineTag     string
	ContainerTypes []instance.ContainerType
}

// WatchContainer identifies a single container type within a machine.
type WatchContainer struct {
	MachineTag    string
	ContainerType string
}

// WatchContainers holds the arguments for making a WatchContainers
// API call.
type WatchContainers struct {
	Params []WatchContainer
}

// CharmURL identifies a single charm URL.
type CharmURL struct {
	URL string
}

// CharmURLs identifies multiple charm URLs.
type CharmURLs struct {
	URLs []CharmURL
}

// StringsResult holds the result of an API call that returns a slice
// of strings or an error.
type StringsResult struct {
	Error  *Error
	Result []string
}

// StringsResults holds the bulk operation result of an API call
// that returns a slice of strings or an error.
type StringsResults struct {
	Results []StringsResult
}

// StringResult holds a string or an error.
type StringResult struct {
	Error  *Error
	Result string
}

// StringResults holds the bulk operation result of an API call
// that returns a string or an error.
type StringResults struct {
	Results []StringResult
}

// ModelResult holds the result of an API call returning a name and UUID
// for a model.
type ModelResult struct {
	Error *Error
	Name  string
	UUID  string
}

// ModelSkeletonConfigArgs wraps the args for modelmanager.SkeletonConfig.
type ModelSkeletonConfigArgs struct {
	Provider string
	Region   string
}

// ModelCreateArgs holds the arguments that are necessary to create
// a model.
type ModelCreateArgs struct {
	// OwnerTag represents the user that will own the new model.
	// The OwnerTag must be a valid user tag.  If the user tag represents
	// a local user, that user must exist.
	OwnerTag string

	// Account holds the provider specific account details necessary to
	// interact with the provider to create, list and destroy machines.
	Account map[string]interface{}

	// Config defines the model config, which includes the name of the
	// model.  An model UUID is allocated by the API server during
	// the creation of the model.
	Config map[string]interface{}
}

// Model holds the result of an API call returning a name and UUID
// for a model and the tag of the server in which it is running.
type Model struct {
	Name     string
	UUID     string
	OwnerTag string
}

// UserModel holds information about a model and the last
// time the model was accessed for a particular user.
type UserModel struct {
	Model
	LastConnection *time.Time
}

// UserModelList holds information about a list of models
// for a particular user.
type UserModelList struct {
	UserModels []UserModel
}

// ResolvedModeResult holds a resolved mode or an error.
type ResolvedModeResult struct {
	Error *Error
	Mode  ResolvedMode
}

// ResolvedModeResults holds the bulk operation result of an API call
// that returns a resolved mode or an error.
type ResolvedModeResults struct {
	Results []ResolvedModeResult
}

// StringBoolResult holds the result of an API call that returns a
// string and a boolean.
type StringBoolResult struct {
	Error  *Error
	Result string
	Ok     bool
}

// StringBoolResults holds multiple results with a string and a bool
// each.
type StringBoolResults struct {
	Results []StringBoolResult
}

// BoolResult holds the result of an API call that returns a
// a boolean or an error.
type BoolResult struct {
	Error  *Error
	Result bool
}

// BoolResults holds multiple results with BoolResult each.
type BoolResults struct {
	Results []BoolResult
}

// IntResults holds multiple results with an int in each.
type IntResults struct {
	// Results holds a list of results for calls that return an int or error.
	Results []IntResult
}

// IntResult holds the result of an API call that returns a
// int or an error.
type IntResult struct {
	// Error holds the error (if any) of this call.
	Error *Error
	// Result holds the integer result of the call (if Error is nil).
	Result int
}

// Settings holds relation settings names and values.
type Settings map[string]string

// SettingsResult holds a relation settings map or an error.
type SettingsResult struct {
	Error    *Error
	Settings Settings
}

// SettingsResults holds the result of an API calls that
// returns settings for multiple relations.
type SettingsResults struct {
	Results []SettingsResult
}

// ConfigSettings holds unit, service or cham configuration settings
// with string keys and arbitrary values.
type ConfigSettings map[string]interface{}

// ConfigSettingsResult holds a configuration map or an error.
type ConfigSettingsResult struct {
	Error    *Error
	Settings ConfigSettings
}

// ConfigSettingsResults holds multiple configuration maps or errors.
type ConfigSettingsResults struct {
	Results []ConfigSettingsResult
}

// ModelConfig holds an model configuration.
type ModelConfig map[string]interface{}

// ModelConfigResult holds model configuration or an error.
type ModelConfigResult struct {
	Config ModelConfig
}

// RelationUnit holds a relation and a unit tag.
type RelationUnit struct {
	Relation string
	Unit     string
}

// RelationUnits holds the parameters for API calls expecting a pair
// of relation and unit tags.
type RelationUnits struct {
	RelationUnits []RelationUnit
}

// RelationIds holds multiple relation ids.
type RelationIds struct {
	RelationIds []int
}

// RelationUnitPair holds a relation tag, a local and remote unit tags.
type RelationUnitPair struct {
	Relation   string
	LocalUnit  string
	RemoteUnit string
}

// RelationUnitPairs holds the parameters for API calls expecting
// multiple sets of a relation tag, a local and remote unit tags.
type RelationUnitPairs struct {
	RelationUnitPairs []RelationUnitPair
}

// RelationUnitSettings holds a relation tag, a unit tag and local
// unit settings.
type RelationUnitSettings struct {
	Relation string
	Unit     string
	Settings Settings
}

// RelationUnitsSettings holds the arguments for making a EnterScope
// or WriteSettings API calls.
type RelationUnitsSettings struct {
	RelationUnits []RelationUnitSettings
}

// RelationResult returns information about a single relation,
// or an error.
type RelationResult struct {
	Error    *Error
	Life     Life
	Id       int
	Key      string
	Endpoint multiwatcher.Endpoint
}

// RelationResults holds the result of an API call that returns
// information about multiple relations.
type RelationResults struct {
	Results []RelationResult
}

// EntityCharmURL holds an entity's tag and a charm URL.
type EntityCharmURL struct {
	Tag      string
	CharmURL string
}

// EntitiesCharmURL holds the parameters for making a SetCharmURL API
// call.
type EntitiesCharmURL struct {
	Entities []EntityCharmURL
}

// BytesResult holds the result of an API call that returns a slice
// of bytes.
type BytesResult struct {
	Result []byte
}

// LifeResult holds the life status of a single entity, or an error
// indicating why it is not available.
type LifeResult struct {
	Life  Life
	Error *Error
}

// LifeResults holds the life or error status of multiple entities.
type LifeResults struct {
	Results []LifeResult
}

// InstanceInfo holds a machine tag, provider-specific instance id, a
// nonce, a list of networks and interfaces to set up.
type InstanceInfo struct {
	Tag             string
	InstanceId      instance.Id
	Nonce           string
	Characteristics *instance.HardwareCharacteristics
	Networks        []Network
	Interfaces      []NetworkInterface
	Volumes         []Volume
	// VolumeAttachments is a mapping from volume tag to
	// volume attachment info.
	VolumeAttachments map[string]VolumeAttachmentInfo
}

// InstancesInfo holds the parameters for making a SetInstanceInfo
// call for multiple machines.
type InstancesInfo struct {
	Machines []InstanceInfo
}

// EntityStatus holds the status of an entity.
type EntityStatus struct {
	Status status.Status
	Info   string
	Data   map[string]interface{}
	Since  *time.Time
}

// EntityStatus holds parameters for setting the status of a single entity.
type EntityStatusArgs struct {
	Tag    string
	Status status.Status
	Info   string
	Data   map[string]interface{}
}

// SetStatus holds the parameters for making a SetStatus/UpdateStatus call.
type SetStatus struct {
	Entities []EntityStatusArgs
}

// ConstraintsResult holds machine constraints or an error.
type ConstraintsResult struct {
	Error       *Error
	Constraints constraints.Value
}

// ConstraintsResults holds multiple constraints results.
type ConstraintsResults struct {
	Results []ConstraintsResult
}

// AgentGetEntitiesResults holds the results of a
// agent.API.GetEntities call.
type AgentGetEntitiesResults struct {
	Entities []AgentGetEntitiesResult
}

// AgentGetEntitiesResult holds the results of a
// machineagent.API.GetEntities call for a single entity.
type AgentGetEntitiesResult struct {
	Life          Life
	Jobs          []multiwatcher.MachineJob
	ContainerType instance.ContainerType
	Error         *Error
}

// VersionResult holds the version and possibly error for a given
// DesiredVersion() API call.
type VersionResult struct {
	Version *version.Number
	Error   *Error
}

// VersionResults is a list of versions for the requested entities.
type VersionResults struct {
	Results []VersionResult
}

// ToolsResult holds the tools and possibly error for a given
// Tools() API call.
type ToolsResult struct {
	Tools                          *tools.Tools
	DisableSSLHostnameVerification bool
	Error                          *Error
}

// ToolsResults is a list of tools for various requested agents.
type ToolsResults struct {
	Results []ToolsResult
}

// Version holds a specific binary version.
type Version struct {
	Version version.Binary
}

// EntityVersion specifies the tools version to be set for an entity
// with the given tag.
// version.Binary directly.
type EntityVersion struct {
	Tag   string
	Tools *Version
}

// EntitiesVersion specifies what tools are being run for
// multiple entities.
type EntitiesVersion struct {
	AgentTools []EntityVersion
}

// NotifyWatchResult holds a NotifyWatcher id and an error (if any).
type NotifyWatchResult struct {
	NotifyWatcherId string
	Error           *Error
}

// NotifyWatchResults holds the results for any API call which ends up
// returning a list of NotifyWatchers
type NotifyWatchResults struct {
	Results []NotifyWatchResult
}

// StringsWatchResult holds a StringsWatcher id, changes and an error
// (if any).
type StringsWatchResult struct {
	StringsWatcherId string
	Changes          []string
	Error            *Error
}

// StringsWatchResults holds the results for any API call which ends up
// returning a list of StringsWatchers.
type StringsWatchResults struct {
	Results []StringsWatchResult
}

// EntitiesWatchResult holds a EntitiesWatcher id, changes and an error
// (if any).
type EntitiesWatchResult struct {
	// Note legacy serialization tag.
	EntitiesWatcherId string   `json:"EntityWatcherId"`
	Changes           []string `json:"Changes"`
	Error             *Error   `json:"Error"`
}

// EntitiesWatchResults holds the results for any API call which ends up
// returning a list of EntitiesWatchers.
type EntitiesWatchResults struct {
	Results []EntitiesWatchResult `json:"Results"`
}

// UnitSettings specifies the version of some unit's settings in some relation.
type UnitSettings struct {
	Version int64 `json:"Version"`
}

// RelationUnitsChange describes the membership and settings of; or changes to;
// some relation scope.
type RelationUnitsChange struct {

	// Changed holds a set of units that are known to be in scope, and the
	// latest known settings version for each.
	Changed map[string]UnitSettings `json:"Changed"`

	// Departed holds a set of units that have previously been reported to
	// be in scope, but which no longer are.
	Departed []string `json:"Departed"`
}

// RelationUnitsWatchResult holds a RelationUnitsWatcher id, baseline state
// (in the Changes field), and an error (if any).
type RelationUnitsWatchResult struct {
	RelationUnitsWatcherId string              `json:"RelationUnitsWatcherId"`
	Changes                RelationUnitsChange `json:"Changes"`
	Error                  *Error              `json:"Error"`
}

// RelationUnitsWatchResults holds the results for any API call which ends up
// returning a list of RelationUnitsWatchers.
type RelationUnitsWatchResults struct {
	Results []RelationUnitsWatchResult
}

// MachineStorageIdsWatchResult holds a MachineStorageIdsWatcher id,
// changes and an error (if any).
type MachineStorageIdsWatchResult struct {
	MachineStorageIdsWatcherId string
	Changes                    []MachineStorageId
	Error                      *Error
}

// MachineStorageIdsWatchResults holds the results for any API call which ends
// up returning a list of MachineStorageIdsWatchers.
type MachineStorageIdsWatchResults struct {
	Results []MachineStorageIdsWatchResult
}

// CharmsResponse is the server response to charm upload or GET requests.
type CharmsResponse struct {
	Error string `json:",omitempty"`

	// ErrorCode holds the code associated with the error.
	// Ideally, Error would hold an Error object and the
	// code would be in that, but for backward compatibility,
	// we cannot do that.
	ErrorCode string `json:",omitempty"`

	// ErrorInfo holds extra information associated with the error.
	// Like ErrorCode, this should really be in an Error object.
	ErrorInfo *ErrorInfo

	CharmURL string   `json:",omitempty"`
	Files    []string `json:",omitempty"`
}

// RunParams is used to provide the parameters to the Run method.
// Commands and Timeout are expected to have values, and one or more
// values should be in the Machines, Services, or Units slices.
type RunParams struct {
	Commands string
	Timeout  time.Duration
	Machines []string
	Services []string
	Units    []string
}

// RunResult contains the result from an individual run call on a machine.
// UnitId is populated if the command was run inside the unit context.
type RunResult struct {
	exec.ExecResponse
	MachineId string
	UnitId    string
	Error     string
}

// RunResults is used to return the slice of results.  API server side calls
// need to return single structure values.
type RunResults struct {
	Results []RunResult
}

// AgentVersionResult is used to return the current version number of the
// agent running the API server.
type AgentVersionResult struct {
	Version version.Number
}

// ProvisioningInfo holds machine provisioning info.
type ProvisioningInfo struct {
	Constraints      constraints.Value
	Series           string
	Placement        string
	Networks         []string
	Jobs             []multiwatcher.MachineJob
	Volumes          []VolumeParams
	Tags             map[string]string
	SubnetsToZones   map[string][]string
	ImageMetadata    []CloudImageMetadata
	EndpointBindings map[string]string
}

// ProvisioningInfoResult holds machine provisioning info or an error.
type ProvisioningInfoResult struct {
	Error  *Error
	Result *ProvisioningInfo
}

// ProvisioningInfoResults holds multiple machine provisioning info results.
type ProvisioningInfoResults struct {
	Results []ProvisioningInfoResult
}

// Metric holds a single metric.
type Metric struct {
	Key   string
	Value string
	Time  time.Time
}

// MetricsParam contains the metrics for a single unit.
type MetricsParam struct {
	Tag     string
	Metrics []Metric
}

// MetricsParams contains the metrics for multiple units.
type MetricsParams struct {
	Metrics []MetricsParam
}

// MetricBatch is a list of metrics with metadata.
type MetricBatch struct {
	UUID     string
	CharmURL string
	Created  time.Time
	Metrics  []Metric
}

// MetricBatchParam contains a single metric batch.
type MetricBatchParam struct {
	Tag   string
	Batch MetricBatch
}

// MetricBatchParams contains multiple metric batches.
type MetricBatchParams struct {
	Batches []MetricBatchParam
}

// MeterStatusResult holds unit meter status or error.
type MeterStatusResult struct {
	Code  string
	Info  string
	Error *Error
}

// MeterStatusResults holds meter status results for multiple units.
type MeterStatusResults struct {
	Results []MeterStatusResult
}

// SingularClaim represents a request for exclusive model administration access
// on the part of some controller.
type SingularClaim struct {
	ModelTag      string        `json:"ModelTag"`
	ControllerTag string        `json:"ControllerTag"`
	Duration      time.Duration `json:"Duration"`
}

// SingularClaims holds any number of SingularClaim~s.
type SingularClaims struct {
	Claims []SingularClaim `json:"Claims"`
}
