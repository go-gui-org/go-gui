---
description: any input (code, docs, papers, images) - knowledge graph - clustered communities - HTML + JSON + audit report
---

# /graphify

Turn any folder of files into a navigable knowledge graph with community detection, an honest audit trail, and three outputs: interactive HTML, GraphRAG-ready JSON, and a plain-language GRAPH_REPORT.md.

## Usage

```
/graphify                                             # full pipeline on current directory → Obsidian vault
/graphify <path>                                      # full pipeline on specific path
/graphify https://github.com/<owner>/<repo>           # clone repo then run full pipeline on it
/graphify <path> --mode deep                          # thorough extraction, richer INFERRED edges
/graphify <path> --update                             # incremental - re-extract only new/changed files
/graphify <path> --directed                            # build directed graph (preserves edge direction: source→target)
/graphify <path> --cluster-only                       # rerun clustering on existing graph
/graphify <path> --no-viz                             # skip visualization, just report + JSON
/graphify <path> --svg                                # also export graph.svg (embeds in Notion, GitHub)
/graphify <path> --graphml                            # export graph.graphml (Gephi, yEd)
/graphify <path> --neo4j                              # generate graphify-out/cypher.txt for Neo4j
/graphify <path> --neo4j-push bolt://localhost:7687   # push directly to Neo4j
/graphify <path> --mcp                                # start MCP stdio server for agent access
/graphify <path> --watch                              # watch folder, auto-rebuild on code changes (no LLM needed)
/graphify <path> --wiki                               # build agent-crawlable wiki (index.md + one article per community)
/graphify <path> --obsidian --obsidian-dir ~/vaults/my-project  # write vault to custom path (e.g. existing vault)
/graphify add <url>                                   # fetch URL, save to ./raw, update graph
/graphify add <url> --author "Name"                   # tag who wrote it
/graphify add <url> --contributor "Name"              # tag who added it to the corpus
/graphify query "<question>"                          # BFS traversal - broad context
/graphify query "<question>" --dfs                    # DFS - trace a specific path
/graphify query "<question>" --budget 1500            # cap answer at N tokens
/graphify path "AuthModule" "Database"                # shortest path between two concepts
/graphify explain "SwinTransformer"                   # plain-language explanation of a node
```

## What graphify is for

graphify is built around Andrej Karpathy's /raw folder workflow: drop anything into a folder - papers, tweets, screenshots, code, notes - and get a structured knowledge graph that shows you what you didn't know was connected.

Three things it does that Claude alone cannot:
1. **Persistent graph** - relationships are stored in `graphify-out/graph.json` and survive across sessions. Ask questions weeks later without re-reading everything.
2. **Honest audit trail** - every edge is tagged EXTRACTED, INFERRED, or AMBIGUOUS. You know what was found vs invented.
3. **Cross-document surprise** - community detection finds connections between concepts in different files that you would never think to ask about directly.

Use it for:
- A codebase you're new to (understand architecture before touching anything)
- A reading list (papers + tweets + notes → one navigable graph)
- A research corpus (citation graph + concept graph in one)
- Your personal /raw folder (drop everything in, let it grow, query it)

## What You Must Do When Invoked

If no path was given, use `.` (current directory). Do not ask the user for a path.

If path argument starts with `https://github.com/` or `http://github.com/`, treat it as a GitHub URL — run Step 0 before anything else, then continue with the resolved local path.

Follow these steps in order. Do not skip steps.

### Step 0 - Clone GitHub repo(s) (only if a GitHub URL was given)

**Single repo:**
```bash
# Clone the repo to a temporary location and use that as the target path
TEMP_DIR=$(mktemp -d)
git clone <github-url> "$TEMP_DIR"
# Use $TEMP_DIR as the target for subsequent steps
```

**Multiple repos (cross-repo graph):**
```bash
# Clone each repo to separate temp directories
TEMP_DIR1=$(mktemp -d)
TEMP_DIR2=$(mktemp -d)
git clone <url1> "$TEMP_DIR1"
git clone <url2> "$TEMP_DIR2"
# Run graphify on each, then merge the resulting graphs
```

### Step 1 - Install graphify if needed

```bash
# Check if graphify is available
if ! command -v graphify &> /dev/null; then
    echo "Installing graphify..."
    # Install via pip (most common method)
    pip install graphify
fi
```

### Step 2 - Run graphify on the target path

```bash
# Execute graphify with the provided options
graphify <target-path> <options>
```

### Step 3 - Review outputs

graphify generates three main outputs:
1. **graphify-out/graph.html** - Interactive visualization
2. **graphify-out/graph.json** - GraphRAG-ready JSON data
3. **graphify-out/GRAPH_REPORT.md** - Plain-language analysis report

Review these outputs and provide a summary of the key findings, communities detected, and any interesting connections discovered.
