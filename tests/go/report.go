package sdet

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Result struct {
	ID     string
	Target string
	Pass   bool
	Detail string
}

var (
	reportMu sync.Mutex
	results  []Result
)

func Record(id, target string, pass bool, detail string) {
	reportMu.Lock()
	defer reportMu.Unlock()
	results = append(results, Result{ID: id, Target: target, Pass: pass, Detail: detail})
	writeReport()
}

func writeReport() {
	root := getenv("KIT_ROOT", filepath.Join("..", ".."))
	dir := filepath.Join(root, "report")
	_ = os.MkdirAll(dir, 0o755)
	var b strings.Builder
	b.WriteString("# Invariants\n\n")
	b.WriteString("| ID | Target | Result | Detail |\n|---|---|---|---|\n")
	for _, r := range results {
		status := "FAIL"
		if r.Pass {
			status = "PASS"
		}
		b.WriteString("| " + r.ID + " | " + r.Target + " | " + status + " | " + r.Detail + " |\n")
	}
	_ = os.WriteFile(filepath.Join(dir, "invariants.md"), []byte(b.String()), 0o644)
}
