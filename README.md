# operator

A k8s native operator for Krok.

## Concepts

- register a repository with commands
- a hook is created
- launch a server to handle the hooks
- what is the process when an `push` event comes in?
  - Create an `Event`
  - The `Event` is reconciled
  - The `Event` will launch the commands as Jobs and gather all output from them
  - It updates its owner Event which will update its Repository Owner


## Source

I need a clean way to get to the source for all commands which need the source to perform something.
That could be achieved by pulling in Flux source-controller and creating a GitRepository object during Event creation.
Creating a GitRepository object would trigger source-controller to reconcile the source and make it available in cluster
as an archive. The GitRepository would deal with the specific REF which the event was triggered for. That can be
retrieved from the payload.

Could be a prerequisite to run this on a cluster:

```shell
flux install \
  --namespace=krok-system \
  --network-policy=false \
  --components=source-controller
```

Then, all I have to do is use GitRepository objects as stated above.

I reimplement this with my own source-controller.
