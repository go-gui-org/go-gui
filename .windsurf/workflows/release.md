---
description: Update changelog, commit, tag with patch bump, push
---

Release workflow for the current project. Steps:

1. Get last tag: !`git describe --tags --abbrev=0`
2. Get changes since last tag: !`git log $(git describe --tags --abbrev=0)..HEAD --oneline`
3. Determine next minor version by incrementing the minor number of the last tag
4. Update CHANGELOG.md with a new entry for the next version, summarizing changes
5. Commit CHANGELOG.md and any staged/modified tracked files with message: `changelog: add <version> (<summary>)`
6. Create annotated tag for the new version
7. Push commit and tag to origin

If $ARGUMENTS is provided, use it as additional context for the changelog entry.
