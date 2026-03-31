# docketeer

A simple task management app that works for me. There are lots of other similar apps but I found that they were either too much or too little.

## how it works

There are tasks and ideas.

Tasks have:
- title
- description
- priority
- current state (todo, in progress, done, blocked)
- project
- due date  
- created time
- last updated time
- an array of timestamped notes

Ideas have:
- title
- description
- project
- created time
- last updated time
- an array of timestamped notes

Top level fields are all in one table and the notes are stored in a separate table with a reference to the parent task or idea.

## storage

The default storage is backed by a simple sqlite file.

## interface

The primary interaction is all done through a TUI powered by bubbletea V2. Docketeer tries to fit recommended patterns and utilize bubbles V2 components wherever possible.

Additionally, docketeer exposes a MCP interface which allows for a local agent to manage tasks on your behalf.
