providers = {
    "krok-controller": {
        "context": ".",
        "image": "gcr.io/krok-o/krok-controller",
        "live_reload_deps": [
            "main.go",
            "api",
            "controllers",
            "../../go.mod",
            "../../go.sum",
        ],
        "label": "krok-controller"
    }
}

os_name = str(local("go env GOOS")).rstrip("\n")
os_arch = str(local("go env GOARCH")).rstrip("\n")

def deploy_controller():
    deployment_kind = "provider"
    p = providers.get("krok-controller")
    label = p.get("label")

    port_forwards = get_port_forwards()

    build_go_binary(
        context = p.get("context"),
        reload_deps = p.get("live_reload_deps"),
        go_main = p.get("go_main", "main.go"),
        binary_name = "manager",
        label = label,
    )

    build_docker_image(
        image = p.get("image"),
        context = p.get("context"),
        binary_name = "manager",
        port_forwards = port_forwards,
    )

def get_port_forwards():
    port_forwards = []

    port_forwards.append(port_forward(9998, 9998))

    return port_forwards

# Gotten from the CAPI TiltFile.
def build_go_binary(context, reload_deps, go_main, binary_name, label):
    build_env = "CGO_ENABLED=0 GOOS=linux GOARCH={arch}".format(
        arch = os_arch,
    )
    ldflags = "-extldflags \"-static\""
    gcflags = ""
    build_cmd = "{build_env} go build -gcflags '{gcflags}' -ldflags '{ldflags}' -o .tiltbuild/bin/{binary_name} {go_main}".format(
        build_env = build_env,
        gcflags = gcflags,
        go_main = go_main,
        ldflags = ldflags,
        binary_name = binary_name,
    )

    # Prefix each live reload dependency with context. For example, for if the context is
    # test/infra/docker and main.go is listed as a dep, the result is test/infra/docker/main.go. This adjustment is
    # needed so Tilt can watch the correct paths for changes.
    live_reload_deps = []
    for d in reload_deps:
        live_reload_deps.append(context + "/" + d)
    local_resource(
        label.lower() + "_binary",
        cmd = "cd {context};mkdir -p .tiltbuild/bin;{build_cmd}".format(
            context = context,
            build_cmd = build_cmd,
        ),
        deps = live_reload_deps,
        labels = [label, "ALL.binaries"],
    )


tilt_helper_dockerfile_header = """
# Tilt image
FROM golang:1.19.0 as tilt-helper
# Support live reloading with Tilt
RUN go install github.com/go-delve/delve/cmd/dlv@latest
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh && chmod +x /go/bin/dlv && \
    touch /process.txt && chmod 0777 /process.txt `# pre-create PID file to allow even non-root users to run the image`
"""

tilt_dockerfile_header = """
FROM gcr.io/distroless/base:debug as tilt
WORKDIR /
COPY --from=tilt-helper /process.txt .
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY --from=tilt-helper /go/bin/dlv .
COPY $binary_name .
"""

def build_docker_image(image, context, binary_name, port_forwards):
    dockerfile_contents = "\n".join([
        tilt_helper_dockerfile_header,
        tilt_dockerfile_header,
    ])

    docker_build(
        ref = image,
        context = context + "/.tiltbuild/bin/",
        dockerfile_contents = dockerfile_contents,
        build_args = {"binary_name": binary_name},
        target = "tilt",
        only = binary_name,
        live_update = [
            sync(context + "/.tiltbuild/bin/" + binary_name, "/" + binary_name),
            run("sh /restart.sh"),
        ],
    )


k8s_yaml(['config/crd/bases/delivery.krok.app_krokcommandruns.yaml', 'config/crd/bases/delivery.krok.app_krokcommands.yaml', 'config/crd/bases/delivery.krok.app_krokevents.yaml', 'config/crd/bases/delivery.krok.app_krokrepositories.yaml'])
# TODO: Add Deployment yaml files and use k8s_resource...
deploy_controller()
