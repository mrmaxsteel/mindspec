import re
from pathlib import Path

class DocParser:
    def __init__(self, workspace):
        self.workspace = workspace

    def parse_glossary(self):
        glossary_path = self.workspace.get_glossary_path()
        if not glossary_path.exists():
            return {}

        terms = {}
        with open(glossary_path, 'r') as f:
            content = f.read()

        # Match markdown table rows: | **Term** | [Link](Target) |
        matches = re.finditer(r'\|\s*\*\*([^*]+)\*\*\s*\|\s*\[[^\]]+\]\(([^)]+)\)\s*\|', content)
        for match in matches:
            term = match.group(1).strip()
            target = match.group(2).strip()
            terms[term] = target
        return terms

    def check_health(self, strict=False):
        docs_dir = self.workspace.get_docs_dir()
        glossary = self.parse_glossary()
        project_root = self.workspace.find_project_root()

        report = {
            "docs_dir_exists": docs_dir.exists(),
            "glossary_exists": self.workspace.get_glossary_path().exists(),
            "term_count": len(glossary),
            "broken_links": [],
            "warnings": []
        }

        for term, target in glossary.items():
            # Handle relative paths in links (strip anchors for existence check)
            path_part = target.split('#')[0]
            target_path = project_root / path_part
            if not target_path.exists():
                report["broken_links"].append(f"{term} -> {target}")

        # Domain structure checks (warn by default, error in strict mode)
        domains_dir = docs_dir / "domains"
        context_map = docs_dir / "context-map.md"
        expected_domains = ["core", "context-system", "workflow"]
        domain_files = ["overview.md", "architecture.md", "interfaces.md", "runbook.md"]

        if not domains_dir.exists():
            report["warnings"].append("docs/domains/ directory not found")
        else:
            for domain in expected_domains:
                domain_dir = domains_dir / domain
                if not domain_dir.exists():
                    report["warnings"].append(f"docs/domains/{domain}/ not found")
                else:
                    for f in domain_files:
                        if not (domain_dir / f).exists():
                            report["warnings"].append(f"docs/domains/{domain}/{f} not found")

        if not context_map.exists():
            report["warnings"].append("docs/context-map.md not found")

        # In strict mode, warnings become errors
        if strict and report["warnings"]:
            report["strict_failures"] = list(report["warnings"])

        return report
