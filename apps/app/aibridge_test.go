package app

import (
	"os"
	"path/filepath"
	"testing"
)

// installState has to answer "is this provider usable?" without starting the
// bridge, so it reads the vendored CLI off disk. The layout it depends on lives
// in the frontend repo's node_modules, which is easy to break from the other
// side — this pins it.
func TestInstallStateFindsVendoredCLI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GRROXY_BRIDGES_DIR", root)

	for _, spec := range bridgeSpecs {
		t.Run(spec.key, func(t *testing.T) {
			proc := &bridgeProc{spec: spec}

			// Nothing on disk yet: not installed (assuming no global CLI here).
			if _, err := os.Stat(filepath.Join(root, spec.dirName)); err == nil {
				t.Fatalf("temp dir unexpectedly populated")
			}

			dir := filepath.Join(root, spec.dirName)
			// resolveDir keys off server.mjs, so the bridge itself must exist too.
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "server.mjs"), []byte("//"), 0o644); err != nil {
				t.Fatal(err)
			}

			// Bridge present but deps not installed → falls back to PATH only.
			installed, cliPath := proc.installState()
			if installed && cliPath == "" {
				t.Fatalf("reported installed with no vendored CLI and no CLI on PATH")
			}

			cli := filepath.Join(dir, spec.bundledCLI)
			if err := os.MkdirAll(filepath.Dir(cli), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cli, []byte("//"), 0o644); err != nil {
				t.Fatal(err)
			}

			if installed, _ := proc.installState(); !installed {
				t.Fatalf("vendored CLI at %s not detected", cli)
			}
		})
	}
}

// The vendored paths in bridgeSpecs are hand-written strings pointing into the
// frontend's node_modules. If a dependency bump moves them, this fails in the
// checkout before it fails silently in the UI.
func TestBundledCLIPathsMatchCheckout(t *testing.T) {
	for _, spec := range bridgeSpecs {
		proc := &bridgeProc{spec: spec}
		dir, err := proc.resolveDir()
		if err != nil {
			t.Skipf("%s bridge not present in this checkout: %v", spec.key, err)
		}
		cli := filepath.Join(dir, spec.bundledCLI)
		if _, err := os.Stat(cli); err != nil {
			t.Errorf("%s: vendored CLI missing at %s (npm install in the bridge, or the path moved): %v",
				spec.key, cli, err)
		}
	}
}
