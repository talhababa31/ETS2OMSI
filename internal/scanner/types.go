package scanner

import (
	"ets2omsi/internal/archive"
	"ets2omsi/internal/graph"
)

type PackageReport struct {
	Version      string       `json:"version"`
	Package      string       `json:"package"`
	Kind         archive.Kind `json:"archive_kind"`
	FileCount    int          `json:"file_count"`
	Vehicles     []Vehicle    `json:"vehicles"`
	Candidates   []Candidate  `json:"candidates,omitempty"`
	SharedAssets []string     `json:"shared_assets,omitempty"`
	Warnings     []string     `json:"warnings,omitempty"`
	Diagnostics  Diagnostics  `json:"diagnostics"`
	Stats        Stats        `json:"stats"`
}

type Stats struct {
	VehicleCount       int `json:"vehicle_count"`
	ReadyCount         int `json:"ready_count"`
	WarningCount       int `json:"warning_count"`
	UnboundCount       int `json:"unbound_count"`
	UniqueDependencies int `json:"unique_dependencies"`
	CandidateCount     int `json:"candidate_count"`
}

type Diagnostics struct {
	LogicalRootPrefix         string         `json:"logical_root_prefix,omitempty"`
	SIIFiles                  int            `json:"sii_files"`
	SUIFiles                  int            `json:"sui_files"`
	VehicleDefinitionFiles    int            `json:"vehicle_definition_files"`
	TrafficStorageFiles       int            `json:"traffic_storage_files"`
	TrafficStoragePaths       []string       `json:"traffic_storage_paths,omitempty"`
	TrafficVehicleUnits       int            `json:"traffic_vehicle_units"`
	LegacyStorageRoots        int            `json:"legacy_storage_roots"`
	StorageDirectIncludes     int            `json:"storage_direct_includes"`
	LegacyModelLinkedIncludes int            `json:"legacy_model_linked_includes"`
	ChassisUnits              int            `json:"chassis_units"`
	IncludeDirectives         int            `json:"include_directives"`
	ParseErrors               int            `json:"parse_errors"`
	ParseErrorPaths           []string       `json:"parse_error_paths,omitempty"`
	EncodedFiles              int            `json:"encoded_files"`
	PMDFiles                  int            `json:"pmd_files"`
	PMGFiles                  int            `json:"pmg_files"`
	PMCFiles                  int            `json:"pmc_files"`
	MATFiles                  int            `json:"mat_files"`
	TOBJFiles                 int            `json:"tobj_files"`
	TextureFiles              int            `json:"texture_files"`
	TopLevelFolders           []string       `json:"top_level_folders,omitempty"`
	UnitTypes                 map[string]int `json:"unit_types,omitempty"`
	GlobalUnitRefsResolved    int            `json:"global_unit_refs_resolved,omitempty"`
	PackageModelBridges       int            `json:"package_model_bridges,omitempty"`
}

type Candidate struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Path        string   `json:"path"`
	UnitType    string   `json:"unit_type"`
	Basis       string   `json:"basis"`
	Confidence  float64  `json:"confidence"`
	ModelRefs   []string `json:"model_refs,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// ModelAsset preserves the semantic role from the SCS definition instead of
// guessing LOD order from filenames. Role is one of main, lod, detail, or accessory.
type ModelAsset struct {
	Path             string `json:"path"`
	Role             string `json:"role"`
	Variant          string `json:"variant,omitempty"`
	Look             string `json:"look,omitempty"`
	SourceDefinition string `json:"source_definition,omitempty"`
}

// WheelAttachment represents a real vehicle_wheel_accessory -> accessory_wheel_data
// chain. The PMD is deliberately kept separate from Vehicle.Models so a wheel can
// never be mistaken for a body LOD.
type WheelAttachment struct {
	UnitID   string `json:"unit_id,omitempty"`
	DataPath string `json:"data_path"`
	Offset   int    `json:"offset"`
	Family   string `json:"family,omitempty"` // f, r, t, ai, or unknown
	Model    string `json:"model,omitempty"`
	Look     string `json:"look,omitempty"`
	Variant  string `json:"variant,omitempty"`
}

type Vehicle struct {
	Readiness          string            `json:"readiness"`
	Blocking           []string          `json:"blocking,omitempty"`
	SharedRequirements []string          `json:"shared_requirements,omitempty"`
	ID                 string            `json:"id"`
	DisplayName        string            `json:"display_name"`
	VehicleType        string            `json:"vehicle_type"`
	RootDefinition     string            `json:"root_definition"`
	StorageDefinition  string            `json:"storage_definition,omitempty"`
	Bound              bool              `json:"bound"`
	DiscoveryBasis     string            `json:"discovery_basis,omitempty"`
	Chassis            []string          `json:"chassis,omitempty"`
	Models             []string          `json:"models,omitempty"`
	ModelAssets        []ModelAsset      `json:"model_assets,omitempty"`
	WheelAttachments   []WheelAttachment `json:"wheel_attachments,omitempty"`
	WheelModels        []string          `json:"wheel_models,omitempty"`
	Variants           []string          `json:"variants,omitempty"`
	Looks              []string          `json:"looks,omitempty"`
	Dependencies       []graph.Node      `json:"dependencies"`
	Edges              []graph.Edge      `json:"edges"`
	Missing            []string          `json:"missing,omitempty"`
	Warnings           []string          `json:"warnings,omitempty"`
	Counts             Counts            `json:"counts"`
}

type Counts struct {
	Models       int `json:"models"`
	WheelModels  int `json:"wheel_models,omitempty"`
	Wheels       int `json:"wheels,omitempty"`
	Materials    int `json:"materials"`
	Textures     int `json:"textures"`
	Dependencies int `json:"dependencies"`
	Variants     int `json:"variants"`
	Missing      int `json:"missing"`
}
