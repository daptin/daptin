# Kubernetes deployment

The manifests use Kustomize and target Kubernetes 1.25 or newer. The base is a
single Daptin replica with persistent local asset storage and an external
PostgreSQL database. The bundled PostgreSQL overlay is for evaluation and local
clusters, not production database hosting.

## External PostgreSQL

Create the namespace and the database connection secret before applying the
base. Keep the complete connection string in the Secret because Kubernetes does
not interpolate one environment variable into another.

```sh
kubectl apply -f kubernetes/base/namespace.yaml
kubectl -n daptin create secret generic daptin-database \
  --from-literal='connection-string=host=postgres.example.net port=5432 user=daptin password=replace-me dbname=daptin sslmode=require'
kubectl apply -k kubernetes/base
kubectl -n daptin rollout status deployment/daptin --timeout=5m
```

The repository base tracks `daptin/daptin:latest` so continuous validation
exercises the current source and container together. The normalized LLM gateway
requires Daptin `v0.13.0` or newer. For production, override the image in a
site-specific overlay and pin a tested version tag or digest. Restart the
deployment after publishing a new image when you want already-running pods to
adopt it:

```sh
kubectl -n daptin rollout restart deployment/daptin
kubectl -n daptin rollout status deployment/daptin --timeout=5m
```

If the cluster has no default StorageClass, set `storageClassName` in
`base/pvc.yaml`. The application volume contains local file assets; back it up
along with PostgreSQL.

## Local or evaluation cluster

The demo overlay creates PostgreSQL with development-only credentials. Its
official-image entrypoint starts briefly as root to set PVC ownership and then
drops to the `postgres` user; this is another reason not to treat the overlay as
a production database deployment.

```sh
kubectl apply -k kubernetes/overlays/demo-postgres
kubectl -n daptin rollout status statefulset/postgres --timeout=5m
kubectl -n daptin rollout status deployment/daptin --timeout=5m
kubectl -n daptin port-forward service/daptin 6336:80
curl --fail http://127.0.0.1:6336/ready
```

After configuring `llm_provider`, `llm_model`, and `llm_deployment`, use
`/llm/readyz` as a gateway-specific operational check. Do not replace the main
pod readiness probe with `/llm/readyz`: an intentionally empty catalog must not
prevent administrators from reaching a new deployment to configure it.

Do not reuse the generated demo password outside an isolated development
cluster.

## Ingress

The ingress overlay is an example for an nginx ingress controller. Change
`ingressClassName`, the hostname, and TLS configuration for the target cluster.
It expects the external database Secret described above.

```sh
kubectl apply -k kubernetes/overlays/ingress
```

## Scaling and upgrades

The base deliberately uses one replica and `Recreate`: the default asset store
is a ReadWriteOnce volume and Daptin initializes database schema at startup.
The base includes a headless Service and configures `DAPTIN_OLRIC_SEED` for
Olric peer discovery. Before scaling beyond one replica, configure shared
cloud/RWX asset storage, validate concurrent schema startup and migrations, and
change the replica count and rollout strategy in a site-specific overlay. Do
not add an HPA until that topology has been load- and failure-tested.

The process marks `/ready` unavailable before draining on SIGTERM. Keep
`terminationGracePeriodSeconds` greater than `DAPTIN_SHUTDOWN_TIMEOUT` plus
`DAPTIN_SHUTDOWN_READINESS_DELAY`. Back up and restore-test PostgreSQL and local
asset storage before changing the Daptin image.

## Validation

Render manifests locally before applying them:

```sh
kubectl kustomize kubernetes/base >/dev/null
kubectl kustomize kubernetes/overlays/demo-postgres >/dev/null
kubectl kustomize kubernetes/overlays/ingress >/dev/null
```

CI renders all three variants, validates them strictly against Kubernetes 1.30
schemas, builds the current Daptin image, and smoke-tests the `/ready` endpoint.
