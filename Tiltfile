load("ext://helm_resource", "helm_repo", "helm_resource")
load("ext://restart_process", "docker_build_with_restart")
load("ext://namespace", "namespace_create")

# Set default trigger mode to manual
trigger_mode(TRIGGER_MODE_MANUAL)

# Disable analytics
analytics_settings(False)

# Disable secrets scrubbing
secret_settings(disable_scrub=True)

# Allow only gangplank k8s context
allow_k8s_contexts("kind-gangplank")

# Create the namespaces
namespace_create("gangplank")
namespace_create("dex")

k8s_resource(
  new_name="namespaces",
  objects=["gangplank:namespace","dex:namespace"],
)

helm_repo(
  name="dex",
  url="https://charts.dexidp.io",
  resource_name="dex-repo",
)

helm_resource(
  name="dex",
  chart="dex/dex",
  release_name="dex",
  namespace="dex",
  flags=["--values", "./configs/helm-values/dex.yaml", "--version=0.16.0"],
  deps=["namespaces", "dex-repo"],
)

k8s_resource(
  workload="dex",
  port_forwards=["5556:5556"],
)

helm_resource(
  name="gangplank",
  chart="./deployments/helm",
  release_name="gangplank",
  image_deps=["gangplank"],
  image_keys=[
    ("image.repository", "image.tag"),
  ],
  namespace="gangplank",
  deps=["namespaces"],
  flags=['--values', "./configs/helm-values/gangplank.yaml"],
)

docker_build_with_restart(
  ref="gangplank",
  context=".",
  dockerfile="Dockerfile-dev",
  live_update=[
    sync("./cmd", "/src/cmd"),
    sync("./internal", "/src/internal"),
    sync("./static", "/src/static"),
    sync("./templates", "/src/templates"),
    sync("./go.mod", "/src/go.mod"),
    sync("./go.sum", "/src/go.sum"),
  ],
  entrypoint=["go", "run", "./cmd/gangplank"],
)

k8s_resource(
  workload="gangplank",
  port_forwards=["8080:8080"],
  trigger_mode=TRIGGER_MODE_AUTO
)
