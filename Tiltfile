# -*- mode: Python -*-

kubectl_cmd = "kubectl"

# verify kubectl command exists
if str(local("command -v " + kubectl_cmd + " || true", quiet = True)) == "":
    fail("Required command '" + kubectl_cmd + "' not found in PATH")

# Use kustomize to build the install yaml files
install = kustomize('config/default')

# Update the root security group. Tilt requires root access to update the
# running process.
objects = decode_yaml_stream(install)
for o in objects:
    if o.get('kind') == 'Deployment' and o.get('metadata').get('name') == 'krok-controller':
        o['spec']['template']['spec']['containers'][0]['securityContext']['runAsNonRoot'] = False
        break

updated_install = encode_yaml_stream(objects)

# Apply the updated yaml to the cluster.
k8s_yaml(updated_install, allow_duplicates = True)

load('ext://restart_process', 'docker_build_with_restart')

local_resource(
    'krok-controller-binary',
    "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/manager ./main.go",
    deps = [
        "main.go",
        "go.mod",
        "go.sum",
        "api",
        "controllers",
        "pkg",
    ],
)

entrypoint = ['/manager']
dockerfile = 'tilt.dockerfile'
docker_build_with_restart(
    'ghcr.io/krok-o/krok-controller',
    '.',
    dockerfile = dockerfile,
    entrypoint = entrypoint,
    only=[
      './bin',
    ],
    live_update = [
        sync('./bin/manager', '/manager'),
    ],
)
