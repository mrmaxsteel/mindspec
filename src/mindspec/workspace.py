import os
from pathlib import Path

class Workspace:
    def __init__(self, root_path=None):
        self.root_path = Path(root_path or os.getcwd()).resolve()
        
    def find_project_root(self):
        """Finds the project root by looking for INIT.md or .git"""
        current = self.root_path
        while current != current.parent:
            if (current / "INIT.md").exists() or (current / ".git").exists():
                return current
            current = current.parent
        return self.root_path

    def get_docs_dir(self):
        return self.find_project_root() / "docs"

    def get_glossary_path(self):
        return self.find_project_root() / "GLOSSARY.md"
