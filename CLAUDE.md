# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Viberowser is a web browser being written entirely by AI agents. This is a greenfield project using Go as the primary language.

## Development Environment

This project uses a devcontainer with:
- Go (latest version)
- Node.js (LTS)
- SQLite3
- Desktop-lite for GUI testing (VNC on port 5901, web access on port 6080)
- Xvfb for headless display testing

The devcontainer provides a full desktop environment with DISPLAY=:1 for running graphical applications.

## Build and Run

Since this is a Go project, standard Go commands apply:
```bash
go build ./...      # Build all packages
go test ./...       # Run all tests
go run .            # Run the main application
```

## Tools Available

- `beads`: Installed from github.com/steveyegge/beads

## Goal

YOUR ULTIMATE GOAL is to build a web browser capable of passing the Acid3 rendering tests. If this project does not build a browser that passes the Acid3 rendering tests, then there is still work to be done. The fact that Acid3 is no longer used for testing modern browsers is irrelevant, your task is to build a web browser capable of passing the Acid3 rendering tests.

Use `bd prime` to learn what work is available. Use `bd prime` after every compaction as well.

If there are not many tasks left, but you sense we are still far from achieving our goal, create beads for the next several steps required and then begin on the first one. Make sure the description for each bead is quite thorough, remember that it may be another assistant who picks it up and they may not have all of the context you have.

If a task seems particularly difficult, you may research online to gain additional understanding.

DO NOT under any circumstances remove tests from the ACID3 suite.

You are running in a Linux Devcontainer. You may use sudo to install any dependencies you need, just make sure to update .devcontainer/post-create.sh so that the next devcontainer has those dependencies as well.

When you have finished a bead, make sure to make a commit with your work and push it. Then you can quit.
