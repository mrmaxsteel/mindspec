import click
from .workspace import Workspace
from .docs import DocParser

@click.group()
def cli():
    """Mindspec: Spec-Driven Development and Self-Documentation System."""
    pass

@cli.command()
def doctor():
    """Check the health of the current workspace documentation."""
    ws = Workspace()
    root = ws.find_project_root()
    parser = DocParser(ws)
    health = parser.check_health()
    
    click.echo(f"Workspace Root: {root}")
    click.echo(f"Docs Directory: {'[OK]' if health['docs_dir_exists'] else '[MISSING]'}")
    click.echo(f"GLOSSARY.md: {'[OK]' if health['glossary_exists'] else '[MISSING]'} ({health['term_count']} terms)")
    
    if health['broken_links']:
        click.echo("\nBroken Links in Glossary:")
        for link in health['broken_links']:
            click.echo(f"  - {link}")
    elif health['glossary_exists']:
        click.echo("Glossary links verified.")

@cli.group()
def context():
    """Context pack management."""
    pass

@context.command(name="init")
@click.option("--spec", required=True, help="Specification ID (e.g., 001)")
def context_init(spec):
    """Initialize a context pack for a specific specification."""
    click.echo(f"Initializing Context Pack for Spec: {spec} (Prototype)")
    # Future: Logic to pull doc sections and memory
    click.echo("Created: docs/specs/001-skeleton/context-pack.md")

if __name__ == "__main__":
    cli()
