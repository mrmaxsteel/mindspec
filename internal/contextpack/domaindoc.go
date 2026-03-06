package contextpack

import (
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// DomainDoc holds the content of a domain's documentation files.
type DomainDoc struct {
	Domain           string
	Overview         string
	Architecture     string
	Interfaces       string
	Runbook          string
	OverviewPath     string
	ArchitecturePath string
	InterfacesPath   string
	RunbookPath      string
}

// ReadDomainDocs reads the 4 standard doc files from a domain directory.
// Missing files result in empty strings, not errors.
func ReadDomainDocs(root, domain string) (*DomainDoc, error) {
	dir := workspace.DomainDir(root, domain)
	doc := &DomainDoc{Domain: domain}

	overviewPath := filepath.Join(dir, "overview.md")
	architecturePath := filepath.Join(dir, "architecture.md")
	interfacesPath := filepath.Join(dir, "interfaces.md")
	runbookPath := filepath.Join(dir, "runbook.md")

	doc.Overview = readFileOrEmpty(overviewPath)
	doc.Architecture = readFileOrEmpty(architecturePath)
	doc.Interfaces = readFileOrEmpty(interfacesPath)
	doc.Runbook = readFileOrEmpty(runbookPath)
	doc.OverviewPath = filepath.ToSlash(relPath(root, overviewPath))
	doc.ArchitecturePath = filepath.ToSlash(relPath(root, architecturePath))
	doc.InterfacesPath = filepath.ToSlash(relPath(root, interfacesPath))
	doc.RunbookPath = filepath.ToSlash(relPath(root, runbookPath))

	return doc, nil
}

func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
