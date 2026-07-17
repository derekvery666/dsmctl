package compatibility

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParsePackageVersion(t *testing.T) {
	cases := []struct {
		raw      string
		known    bool
		segments []int
	}{
		{"4.0.3-27892", true, []int{4, 0, 3, 27892}},
		{"3.5.2-26102", true, []int{3, 5, 2, 26102}},
		{"1.2-0123", true, []int{1, 2, 123}},
		{" 13.1.2-0326 ", true, []int{13, 1, 2, 326}},
		{"2.0", true, []int{2, 0}},
		{"", false, nil},
		{"beta", false, nil},
	}
	for _, testCase := range cases {
		version := ParsePackageVersion(testCase.raw)
		if version.Known() != testCase.known {
			t.Fatalf("ParsePackageVersion(%q).Known() = %t", testCase.raw, version.Known())
		}
		if len(version.segments) != len(testCase.segments) {
			t.Fatalf("ParsePackageVersion(%q) segments = %v, want %v", testCase.raw, version.segments, testCase.segments)
		}
		for index, want := range testCase.segments {
			if version.segments[index] != want {
				t.Fatalf("ParsePackageVersion(%q) segments = %v, want %v", testCase.raw, version.segments, testCase.segments)
			}
		}
	}
}

func TestPackageVersionCompare(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"4.0.3-27892", "4.0.3-27892", 0},
		{"3.5", "3.5.0-0", 0},
		{"3.5", "3.5.0-1", -1},
		{"3.9.9-9999", "4.0.0", -1},
		{"4.0.0", "3.9.9-9999", 1},
		{"10.0.0", "9.9.9", 1},
		{"1.2-0123", "1.2-124", -1},
	}
	for _, testCase := range cases {
		got := ParsePackageVersion(testCase.left).Compare(ParsePackageVersion(testCase.right))
		if got != testCase.want {
			t.Fatalf("Compare(%q, %q) = %d, want %d", testCase.left, testCase.right, got, testCase.want)
		}
	}
}

func TestPackageMatchersRequireLoadedCatalog(t *testing.T) {
	target := NewTarget()
	for name, matcher := range map[string]Matcher{
		"installed": PackageInstalled("SynologyDrive"),
		"range":     PackageVersionRange("SynologyDrive", ParsePackageVersion("3.0"), PackageVersion{}),
	} {
		matched, reason := matcher(target)
		if matched || !strings.Contains(reason, "catalog was not loaded") {
			t.Fatalf("%s matcher on unloaded catalog: matched=%t reason=%q", name, matched, reason)
		}
	}

	target.SetInstalledPackages(nil)
	matched, reason := PackageInstalled("SynologyDrive")(target)
	if matched || !strings.Contains(reason, "not installed") {
		t.Fatalf("empty loaded catalog: matched=%t reason=%q", matched, reason)
	}
}

func TestPackageVersionRangeBounds(t *testing.T) {
	target := NewTarget()
	target.SetInstalledPackages([]InstalledPackage{
		{ID: "SynologyDrive", Version: ParsePackageVersion("3.5.2-26102"), Running: true},
		{ID: "NoVersion", Running: true},
	})

	cases := []struct {
		name     string
		minimum  string
		maximum  string
		want     bool
		fragment string
	}{
		{"in range", "3.0", "4.0", true, "in the required version range"},
		{"at minimum", "3.5.2-26102", "", true, "in the required version range"},
		{"below minimum", "3.6", "", false, "below the minimum"},
		{"at exclusive maximum", "", "3.5.2-26102", false, "at or above the exclusive maximum"},
		{"unbounded", "", "", true, "in the required version range"},
	}
	for _, testCase := range cases {
		matcher := PackageVersionRange("SynologyDrive", ParsePackageVersion(testCase.minimum), ParsePackageVersion(testCase.maximum))
		matched, reason := matcher(target)
		if matched != testCase.want || !strings.Contains(reason, testCase.fragment) {
			t.Fatalf("%s: matched=%t reason=%q", testCase.name, matched, reason)
		}
	}

	matched, reason := PackageVersionRange("NoVersion", ParsePackageVersion("1.0"), PackageVersion{})(target)
	if matched || !strings.Contains(reason, "no parseable version") {
		t.Fatalf("unparseable version: matched=%t reason=%q", matched, reason)
	}
	matched, reason = PackageVersionRange("Absent", PackageVersion{}, PackageVersion{})(target)
	if matched || !strings.Contains(reason, "not installed") {
		t.Fatalf("absent package: matched=%t reason=%q", matched, reason)
	}
}

// TestPackageVersionSelectsDifferentVariantsOnSameDSM proves the axis this
// framework adds: one DSM target selects different operation variants purely
// from the installed package version.
func TestPackageVersionSelectsDifferentVariantsOnSameDSM(t *testing.T) {
	operation := Operation[struct{}, string]{
		Name: "drive.test.read",
		Variants: []Variant[struct{}, string]{
			{
				Name: "drive-v4", API: "SYNO.SynologyDrive.Test", Version: 1, Priority: 20,
				Match: All(
					APIVersion("SYNO.SynologyDrive.Test", 1),
					PackageVersionRange("SynologyDrive", ParsePackageVersion("4.0"), PackageVersion{}),
				),
				Execute: returnName("drive-v4"),
			},
			{
				Name: "drive-v3", API: "SYNO.SynologyDrive.Test", Version: 1, Priority: 10,
				Match: All(
					APIVersion("SYNO.SynologyDrive.Test", 1),
					PackageVersionRange("SynologyDrive", ParsePackageVersion("3.0"), ParsePackageVersion("4.0")),
				),
				Execute: returnName("drive-v3"),
			},
		},
	}

	run := func(packageVersion string) (string, Selection, error) {
		target := NewTarget()
		target.SetAPI("SYNO.SynologyDrive.Test", APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1})
		target.SetInstalledPackages([]InstalledPackage{
			{ID: "SynologyDrive", Version: ParsePackageVersion(packageVersion), Running: true},
		})
		return operation.Run(context.Background(), target, executorFunc(func(context.Context, Request) (json.RawMessage, error) {
			return nil, nil
		}), struct{}{})
	}

	result, selection, err := run("4.0.3-27892")
	if err != nil || result != "drive-v4" || selection.Backend != "drive-v4" {
		t.Fatalf("Drive 4: result=%q selection=%#v err=%v", result, selection, err)
	}
	if !strings.Contains(selection.Reason, "package SynologyDrive 4.0.3-27892") {
		t.Fatalf("Drive 4 selection reason lacks package evidence: %q", selection.Reason)
	}
	result, selection, err = run("3.5.2-26102")
	if err != nil || result != "drive-v3" || selection.Backend != "drive-v3" {
		t.Fatalf("Drive 3: result=%q selection=%#v err=%v", result, selection, err)
	}
	if _, _, err = run("2.0.4-11112"); !IsUnsupported(err) {
		t.Fatalf("Drive 2 should be unsupported, got %v", err)
	}
}

func TestReportCarriesInstalledPackages(t *testing.T) {
	target := NewTarget()
	if report := target.Report(); report.Packages != nil {
		t.Fatalf("unloaded catalog should omit packages, got %#v", report.Packages)
	}
	target.SetInstalledPackages([]InstalledPackage{
		{ID: "SynologyDrive", Version: ParsePackageVersion("4.0.3-27892"), Running: true},
		{ID: "Chat", Version: ParsePackageVersion("2.6.0-1234"), Running: false},
	})
	report := target.Report()
	if len(report.Packages) != 2 || report.Packages[0].ID != "Chat" || report.Packages[1].ID != "SynologyDrive" {
		t.Fatalf("report packages = %#v", report.Packages)
	}
	if report.Packages[1].Version != "4.0.3-27892" || !report.Packages[1].Running {
		t.Fatalf("report package evidence = %#v", report.Packages[1])
	}
}
