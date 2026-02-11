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

    def check_health(self):
        docs_dir = self.workspace.get_docs_dir()
        glossary = self.parse_glossary()
        
        report = {
            "docs_dir_exists": docs_dir.exists(),
            "glossary_exists": self.workspace.get_glossary_path().exists(),
            "term_count": len(glossary),
            "broken_links": []
        }
        
        project_root = self.workspace.find_project_root()
        for term, target in glossary.items():
            # Handle relative paths in links (strip anchors for existence check)
            path_part = target.split('#')[0]
            target_path = project_root / path_part
            if not target_path.exists():
                report["broken_links"].append(f"{term} -> {target}")
                
        return report
