# dinnerwise dev infra.
#
# Currently deploys only the local Grafana observability stack (otel-lgtm) to
# the configured k8s cluster, with port-forwards so a locally-run dinnerwise
# server can export OTLP to localhost and you can browse Grafana at
# localhost:3000. The Go server still runs locally (go run ./cmd/server); see
# README / .env for the OTEL_ env it needs.
#
#   tilt up      # bring the stack up (Grafana :3000, OTLP :4317/:4318)
#   tilt down    # tear it down

# Tilt guards non-local k8s contexts by default. The observability stack is a
# throwaway dev deployment, so allow whichever context you currently have
# selected (`kubectl config current-context`).
allow_k8s_contexts(k8s_context())

k8s_yaml(kustomize('deploy/otel-lgtm'))

k8s_resource(
    'otel-lgtm',
    port_forwards=[
        '3000:3000',   # Grafana UI -> http://localhost:3000 (admin/admin)
        '4317:4317',   # OTLP gRPC
        '4318:4318',   # OTLP HTTP
    ],
    labels=['observability'],
)
