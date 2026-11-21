---
description: Show this session's project, or add this repository to a named project
argument-hint: "[project-name]"
---

If "$ARGUMENTS" is empty, call the cestafet `project` tool without a name and show the project
and its member repositories.

Otherwise call `project` with `name` set to "$ARGUMENTS" and confirm which repositories now
belong to that project. Mention that other sessions in this repository move over at their next
prompt.
