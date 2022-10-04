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

And, since I would create them programmatically, the event will pause until the source is reconciled.

```go
  if err := wait.PollImmediate(2*time.Second, 1*time.Minute,
    func() (done bool, err error) {
      namespacedName := types.NamespacedName{
        Namespace: gitRepository.GetNamespace(),
        Name:      gitRepository.GetName(),
      }
      if err := kubeClient.Get(ctx, namespacedName, gitRepository); err != nil {
        return false, err
      }
      return meta.IsStatusConditionTrue(gitRepository.Status.Conditions, apimeta.ReadyCondition), nil
    }); err != nil {
    fmt.Println(err)
  }
```

Just tested manually. This works nicely. Flux will create a URL for the artifact and I can specify the exact commit SHA.
I also need to reference a secret that is the authentication required for GitRepository to work.
