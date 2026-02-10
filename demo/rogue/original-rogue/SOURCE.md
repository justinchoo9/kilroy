Upstream source provenance
==========================

This directory vendors the Rogue 5.4.4 source snapshot from:

- Repository: https://github.com/Davidslv/rogue
- Commit: `f4653c2a2ee6981a73abe9dfda055134285e1e79`

Why vendored here
-----------------

Kilroy Attractor run worktrees need these files available directly in the
repository checkout. Tracking this directory as a gitlink/submodule caused
worktrees to receive an empty `demo/rogue/original-rogue/` directory unless
submodules were initialized, which broke the Rogue port pipeline stages that
read these C sources.
