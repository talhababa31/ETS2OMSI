package app

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"ets2omsi/internal/archive"
	"ets2omsi/internal/scanner"
)

type ScanOptions struct {
	PackagePath string   `json:"path"`
	HelperPaths []string `json:"-"`
}

type Project struct {
	Options    ScanOptions           `json:"options"`
	Report     scanner.PackageReport `json:"report"`
	MountPaths []string              `json:"mount_paths"` // V2: selected SCS only
}

func ScanProject(opt ScanOptions) (Project, error) {
	if strings.TrimSpace(opt.PackagePath) == "" {
		return Project{}, errors.New("package path is required")
	}
	src, err := archive.Open(opt.PackagePath, archive.OpenOptions{HelperPaths: opt.HelperPaths})
	if err != nil {
		return Project{}, err
	}
	defer src.Close()
	rep, err := scanner.Scan(src)
	if err != nil {
		return Project{}, err
	}

	// V2 intentionally targets basic ETS2 AI traffic cars only. Trucks, buses,
	// player vehicles and unbound diagnostic candidates are outside this product.
	cars := make([]scanner.Vehicle, 0, len(rep.Vehicles))
	for _, v := range rep.Vehicles {
		if strings.EqualFold(v.VehicleType, "car") && v.Bound {
			cars = append(cars, v)
		}
	}
	rep.Vehicles = cars
	rep.Stats.VehicleCount = len(cars)
	rep.Stats.ReadyCount, rep.Stats.WarningCount, rep.Stats.UnboundCount = 0, 0, 0
	dep := map[string]bool{}
	for _, v := range cars {
		if v.Readiness == "ready" {
			rep.Stats.ReadyCount++
		}
		if len(v.Warnings) > 0 || len(v.Missing) > 0 {
			rep.Stats.WarningCount++
		}
		for _, n := range v.Dependencies {
			dep[strings.ToLower(n.Path)] = true
		}
	}
	rep.Stats.UniqueDependencies = len(dep)
	rep.Stats.CandidateCount = 0
	rep.Candidates = nil
	rep.Warnings = append(rep.Warnings, "V2 SCS-only mode: only storage-bound AI cars are shown; ETS2 base/DLC installation is never used.")
	abs, _ := filepath.Abs(opt.PackagePath)
	return Project{Options: opt, Report: rep, MountPaths: []string{abs}}, nil
}

func DefaultOutputRoot(_ string) string { return "output" }

func FindVehicle(rep scanner.PackageReport, id string) (scanner.Vehicle, bool) {
	for _, v := range rep.Vehicles {
		if v.ID == id {
			return v, true
		}
	}
	return scanner.Vehicle{}, false
}
func SortedVehicleIDs(rep scanner.PackageReport) []string {
	out := make([]string, 0, len(rep.Vehicles))
	for _, v := range rep.Vehicles {
		out = append(out, v.ID)
	}
	sort.Strings(out)
	return out
}
