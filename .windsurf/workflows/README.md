# Imported Skills and Plugins

This directory contains skills and workflows imported from `~/.claude/skills/` and configured for use in the go-gui project.

## Available Workflows

### /antivibe
Anti-vibecoding learning framework. Generate detailed explanations of code written by AI with curated external resources for deeper learning.

**Usage**: `/antivibe` or "deep dive" or "explain what AI wrote"

### /golang
Go language expertise for writing idiomatic, production-quality Go code. Covers concurrency patterns, error handling, testing, and module management.

**Usage**: Automatically triggered by Go-related keywords

### /graphify
Turn any folder of files into a navigable knowledge graph with community detection and audit trail.

**Usage**: `/graphify <path>` with various options for visualization and analysis

### /grill-me
Interview the user relentlessly about a plan or design until reaching shared understanding.

**Usage**: `/grill-me` or "grill me"

### /harden
Harden uncommitted changes against bad data and denial of service attacks.

**Usage**: `/harden`

### /release
Update changelog, commit, tag with patch bump, push.

**Usage**: `/release`

### /review-changes
Review uncommitted changes for quality, consistency, security, and performance.

**Usage**: `/review-changes`

### /test-gaps
Find test gaps in uncommitted code changes. Reports untested public functions, missing edge cases, and uncovered error paths.

**Usage**: `/test-gaps`

## Installed Plugins

The following Claude Code plugins are available from your `~/.claude/plugins/` installation:

### Code Quality & Development
- **code-simplifier** - Simplify complex code
- **code-review** - Automated code review
- **modern-go-guidelines** - Go-specific best practices

### Go Development
- **gopls-lsp** - Go language server protocol support

### Context & Analysis
- **context-mode** - Enhanced context management
- **context7** - Advanced context handling

### Productivity
- **commit-commands** - Git commit automation
- **serena** - Development assistant
- **cc-statusline** - Status line enhancements

### Health & Setup
- **health** - System health monitoring
- **claude-code-setup** - Initial setup utilities

## Usage

These workflows are now available as slash commands in your IDE. Simply type `/` followed by the workflow name to invoke them.

## Configuration

Plugin configurations are stored in `~/.claude/plugins/installed_plugins.json`. No additional setup is required for the workflows - they're ready to use immediately.
