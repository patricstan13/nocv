# Go test project

`project` is NOCV's one checked-in representative Go project. Its packages
cover structural discovery, imports, semantic dependency views, signature
changes, interface contracts, method promotion, and compiler-overlay rechecks.

The project is valid Go. Tests that need syntax errors, incomplete type
information, or other exceptional compiler states create those states in a
temporary directory instead of adding broken source here.
