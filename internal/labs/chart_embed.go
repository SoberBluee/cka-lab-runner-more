package labs

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:charts/cka-webapp
var ckaWebappChart embed.FS

const ckaWebappChartName = "cka-webapp"

// hostChartMirror is where the lab runner materializes charts for local helm
// clients (exam path /opt/CKA is mirrored under /tmp for workstations without
// root write access to /opt).
const hostChartMirrorRoot = "/tmp/opt/CKA/charts"

// materializeCKAWebappChart writes the embedded chart to destDir/cka-webapp.
func materializeCKAWebappChart(destRoot string) (string, error) {
	chartDir := filepath.Join(destRoot, ckaWebappChartName)
	if err := os.RemoveAll(chartDir); err != nil {
		return "", fmt.Errorf("clearing chart dir: %w", err)
	}
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		return "", fmt.Errorf("creating chart dir: %w", err)
	}

	err := fs.WalkDir(ckaWebappChart, "charts/cka-webapp", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("charts/cka-webapp", path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(chartDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := ckaWebappChart.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return "", fmt.Errorf("materializing chart: %w", err)
	}
	return chartDir, nil
}
