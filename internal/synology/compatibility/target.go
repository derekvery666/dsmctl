package compatibility

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var dsmVersionPattern = regexp.MustCompile(`(?i)(?:DSM\s*)?(\d+)\.(\d+)(?:\.(\d+))?(?:-(\d+))?`)

type APIInfo struct {
	Path          string `json:"path"`
	MinVersion    int    `json:"minVersion"`
	MaxVersion    int    `json:"maxVersion"`
	RequestFormat string `json:"requestFormat,omitempty"`
}

func (info APIInfo) Supports(version int) bool {
	return version >= info.MinVersion && version <= info.MaxVersion
}

type DSMVersion struct {
	Raw   string `json:"raw,omitempty"`
	Major int    `json:"major,omitempty"`
	Minor int    `json:"minor,omitempty"`
	Patch int    `json:"patch,omitempty"`
	Build int    `json:"build,omitempty"`
}

func ParseDSMVersion(value string) DSMVersion {
	version := DSMVersion{Raw: strings.TrimSpace(value)}
	parts := dsmVersionPattern.FindStringSubmatch(value)
	if len(parts) == 0 {
		return version
	}
	version.Major, _ = strconv.Atoi(parts[1])
	version.Minor, _ = strconv.Atoi(parts[2])
	version.Patch, _ = strconv.Atoi(parts[3])
	version.Build, _ = strconv.Atoi(parts[4])
	return version
}

func (version DSMVersion) Known() bool {
	return version.Major > 0
}

func (version DSMVersion) Compare(other DSMVersion) int {
	left := [...]int{version.Major, version.Minor, version.Patch, version.Build}
	right := [...]int{other.Major, other.Minor, other.Patch, other.Build}
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

// PackageVersion is an installed Synology package version such as
// "4.0.3-27892". Synology package versions are dot/dash-separated numeric
// segments; they are compared segment-wise with missing segments treated as
// zero, so "3.5" < "3.5.0-1" and "3.9.9-9999" < "4.0.0".
type PackageVersion struct {
	Raw      string `json:"raw,omitempty"`
	segments []int
}

var packageVersionSegmentPattern = regexp.MustCompile(`\d+`)

func ParsePackageVersion(value string) PackageVersion {
	version := PackageVersion{Raw: strings.TrimSpace(value)}
	for _, segment := range packageVersionSegmentPattern.FindAllString(version.Raw, -1) {
		number, err := strconv.Atoi(segment)
		if err != nil {
			// A segment beyond the int range is out of scope for real Synology
			// package versions; treat the whole version as unknown.
			return PackageVersion{Raw: version.Raw}
		}
		version.segments = append(version.segments, number)
	}
	return version
}

func (version PackageVersion) Known() bool {
	return len(version.segments) > 0
}

func (version PackageVersion) Compare(other PackageVersion) int {
	length := len(version.segments)
	if len(other.segments) > length {
		length = len(other.segments)
	}
	for index := 0; index < length; index++ {
		left, right := 0, 0
		if index < len(version.segments) {
			left = version.segments[index]
		}
		if index < len(other.segments) {
			right = other.segments[index]
		}
		if left < right {
			return -1
		}
		if left > right {
			return 1
		}
	}
	return 0
}

func (version PackageVersion) String() string {
	if version.Raw != "" {
		return version.Raw
	}
	return "unknown"
}

// InstalledPackage is one entry of the installed-package catalog used for
// package-scoped operation selection.
type InstalledPackage struct {
	ID      string         `json:"id"`
	Version PackageVersion `json:"version"`
	Running bool           `json:"running"`
}

type Target struct {
	DSM          DSMVersion
	APIs         map[string]APIInfo
	capabilities map[string]struct{}
	quirks       map[string]struct{}
	packages     map[string]InstalledPackage
	// packagesKnown distinguishes "catalog loaded and package absent" from
	// "catalog never loaded"; matchers must not treat the latter as evidence.
	packagesKnown bool
}

func NewTarget() Target {
	return Target{
		APIs:         make(map[string]APIInfo),
		capabilities: make(map[string]struct{}),
		quirks:       make(map[string]struct{}),
		packages:     make(map[string]InstalledPackage),
	}
}

func (target *Target) Normalize() {
	if target.APIs == nil {
		target.APIs = make(map[string]APIInfo)
	}
	if target.capabilities == nil {
		target.capabilities = make(map[string]struct{})
	}
	if target.quirks == nil {
		target.quirks = make(map[string]struct{})
	}
	if target.packages == nil {
		target.packages = make(map[string]InstalledPackage)
	}
}

func (target Target) API(name string) (APIInfo, bool) {
	info, ok := target.APIs[name]
	return info, ok
}

func (target Target) SupportsAPI(name string, version int) bool {
	info, ok := target.API(name)
	return ok && info.Supports(version)
}

func (target *Target) SetAPI(name string, info APIInfo) {
	target.Normalize()
	target.APIs[name] = info
}

func (target *Target) AddCapability(name string) {
	target.Normalize()
	target.capabilities[name] = struct{}{}
}

func (target Target) HasCapability(name string) bool {
	_, ok := target.capabilities[name]
	return ok
}

// SetInstalledPackages replaces the complete installed-package catalog. An
// empty list is a valid loaded catalog (no packages installed); callers must
// pass the full inventory, never a partial view.
func (target *Target) SetInstalledPackages(packages []InstalledPackage) {
	target.Normalize()
	target.packages = make(map[string]InstalledPackage, len(packages))
	for _, entry := range packages {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		entry.ID = id
		target.packages[id] = entry
	}
	target.packagesKnown = true
}

func (target Target) InstalledPackage(id string) (InstalledPackage, bool) {
	entry, ok := target.packages[id]
	return entry, ok
}

// PackageCatalogKnown reports whether SetInstalledPackages has provided a
// complete inventory for this target.
func (target Target) PackageCatalogKnown() bool {
	return target.packagesKnown
}

func (target *Target) AddQuirk(name string) {
	target.Normalize()
	target.quirks[name] = struct{}{}
}

func (target Target) HasQuirk(name string) bool {
	_, ok := target.quirks[name]
	return ok
}

type APIReport struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	MinVersion    int    `json:"min_version"`
	MaxVersion    int    `json:"max_version"`
	RequestFormat string `json:"request_format,omitempty"`
}

// PackageReport is the observed installed-package evidence carried by a
// compatibility report when the package catalog has been loaded.
type PackageReport struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
	Running bool   `json:"running"`
}

type Report struct {
	DSM          DSMVersion      `json:"dsm"`
	APIs         []APIReport     `json:"apis"`
	Packages     []PackageReport `json:"packages,omitempty"`
	Capabilities []string        `json:"capabilities"`
	Quirks       []string        `json:"quirks,omitempty"`
	Operations   []Selection     `json:"operations"`
}

func (target Target) Report(selections ...Selection) Report {
	apiNames := make([]string, 0, len(target.APIs))
	for name := range target.APIs {
		apiNames = append(apiNames, name)
	}
	sort.Strings(apiNames)
	apis := make([]APIReport, 0, len(apiNames))
	for _, name := range apiNames {
		info := target.APIs[name]
		apis = append(apis, APIReport{
			Name:          name,
			Path:          info.Path,
			MinVersion:    info.MinVersion,
			MaxVersion:    info.MaxVersion,
			RequestFormat: info.RequestFormat,
		})
	}
	capabilities := sortedSet(target.capabilities)
	quirks := sortedSet(target.quirks)
	operations := append([]Selection(nil), selections...)
	sort.Slice(operations, func(i, j int) bool {
		return operations[i].Operation < operations[j].Operation
	})
	var packages []PackageReport
	if target.packagesKnown {
		packages = make([]PackageReport, 0, len(target.packages))
		for _, entry := range target.packages {
			packages = append(packages, PackageReport{ID: entry.ID, Version: entry.Version.Raw, Running: entry.Running})
		}
		sort.Slice(packages, func(i, j int) bool { return packages[i].ID < packages[j].ID })
	}
	return Report{
		DSM:          target.DSM,
		APIs:         apis,
		Packages:     packages,
		Capabilities: capabilities,
		Quirks:       quirks,
		Operations:   operations,
	}
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (version DSMVersion) String() string {
	if version.Raw != "" {
		return version.Raw
	}
	if !version.Known() {
		return "unknown"
	}
	value := fmt.Sprintf("DSM %d.%d.%d", version.Major, version.Minor, version.Patch)
	if version.Build > 0 {
		value += fmt.Sprintf("-%d", version.Build)
	}
	return value
}
