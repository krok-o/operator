# operator

A k8s native operator for Krok.

## Concepts

- register a repository with commands
- a hook is created
- launch a server to handle the hooks
- what is the process when an `push` event comes in?
  - Create an `Event`
  - The `Event` is reconciled
  - The `Event` reconciler creates a CommandRun
  - The `CommandRun` will launch the commands as Jobs and gather all output from them
  - It updates its owner Event which will update its Repository Owner
