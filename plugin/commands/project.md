---
description: Show this session's project, or add this repository to a named project
argument-hint: "[project-name]"
---

If "$ARGUMENTS" is empty, call the cestafet `project` tool without a name and show the project,
its member repositories and this session's location.

Otherwise:

1. Call `whoami` and read `location`: the repository or folder that will be added. It follows the
   session's working directory as of the last prompt.
2. Tell the user which location will be added to project "$ARGUMENTS". If it is not the
   repository they mean, stop and say so.
3. Call `project` with `name` set to "$ARGUMENTS" and `location` set to that value.
4. If the result has `applied: false`, nothing changed: show its note and any listed sessions
   that would change project, and ask the user. Retry with `force: true` only if the user
   explicitly says so after seeing the list.
5. On success, say which repositories now belong to the project and that other sessions in this
   repository move over at their next prompt.
