package app

import "ets2omsi/internal/scanner"

func ScanPath(path string, helpers []string) (scanner.PackageReport, error) {
	p, e := ScanProject(ScanOptions{PackagePath: path, HelperPaths: helpers})
	return p.Report, e
}
