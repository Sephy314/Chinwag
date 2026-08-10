# Chinwag CI/CD (GitHub Actions)

| Workflow | Trigger | What it does |
|---|---|---|
| `ci.yml` | push (any branch), pull_request, manual (`workflow_dispatch`) | Backend: `make test` + `make vet` · Frontend: `npm ci` → `npm test` → `npm run lint` → `npm run build` · K3s infra: `infra/test.sh` (manual only, self-hosted `k3s` runner) |
| `cd.yml` | push to `main`, manual (`workflow_dispatch`) | Self-hosted runner on the k3s node syncs `~/Chinwag` to `origin/main` and runs `infra/k3s/update.sh` (app images + manifests **and** the observability stack: Loki/Alloy/Prometheus/Grafana) |

## CI

Runs on GitHub-hosted `ubuntu-latest` runners. No live infra (Postgres/Redis/NATS) is
required — backend tests are unit tests with mocks, and the only live-DB test
(`projectionRepo_test.go`) self-skips when no DB is reachable.

> Note: the frontend lockfile is `package-lock.json` (`npm`). The stale
> `frontend/pnpm-lock.yaml` in the repo is **not** used — keep `package-lock.json`
> in sync when changing dependencies.

### K3s infrastructure test (`infra/test.sh`)

`infra/test.sh` is a real integration test: it applies the kustomize manifests to
a K3s cluster, waits for readiness, and runs runtime checks (Service DNS/TCP,
health endpoints, gateway proxy, Traefik Ingress). It is **not** a YAML-only test.

GitHub-hosted runners have **no access to a K3s cluster**, so the `k3s-infra` job
never runs on `ubuntu-latest`. It runs on the **same self-hosted runner as CD**
(the one on the k3s node, which has kubectl/helm access) on **every merge to
`main`** and on manual dispatch (`workflow_dispatch`) — i.e. it verifies the
deployed **production** cluster after each deploy. It is intentionally **not** run
on pull requests (the runner points at production and `test.sh` applies the
manifests).

Because `infra/k3s/secret.yaml` is gitignored, a fresh `actions/checkout` does not
contain it; the job copies it from the runner's **persistent CD checkout**
(`~/Chinwag`, see `cd.yml` `DEPLOY_DIR`) before running the test. If that path
differs on your runner, adjust the copy step in `ci.yml`.

> To test a **non-production** cluster instead, register a separate self-hosted
> runner with a `k3s` label pointing at a dev/test cluster and change the
> `k3s-infra` job's `runs-on` + `if` accordingly.

## CD — self-hosted runner (no SSH needed)

Instead of SSH-ing in, a **GitHub Actions runner is installed directly on the k3s
node**. The CD job runs on that runner, so `update.sh` executes natively with
`docker`, `k3s`, and `kubectl` all available — no inbound ports, no SSH keys, no
repo secrets.

`update.sh` also updates the **observability stack** (`./observability/install.sh`:
Loki, Alloy, Prometheus, Grafana via Helm — plus the `grafana-tls` cert and the
`grafana-dashboards` ConfigMap). The runner needs `helm` and a working kubeconfig;
`observability/install.sh` bootstraps both if missing (installs `helm`/standalone
`kubectl` to `~/.local/bin`, or uses `/etc/rancher/k3s/k3s.yaml`), so no extra
node setup is required beyond what's below.

### 1. Register a runner on the node

1. GitHub repo → **Settings → Actions → Runners → New self-hosted runner** — copy
   the registration token and the download/run commands shown there.
2. On the node (as the user that will run deploys):
   ```bash
   mkdir -p ~/actions-runner && cd ~/actions-runner
   # download the runner tarball for the node's arch (linux-x64 or linux-arm64)
   curl -o actions-runner.tar.gz -L <URL-from-GitHub>
   tar xzf actions-runner.tar.gz
   ./config.sh --url https://github.com/Sephy314/Chinwag --token <REG_TOKEN>
   # run as a background service
   sudo ./svc.sh install
   sudo ./svc.sh start
   ```
   The runner automatically gets the labels `self-hosted`, `linux`, and its arch —
   matching the `runs-on: [self-hosted, linux]` in `cd.yml`.

### 2. Node prerequisites (one time)

1. **Persistent checkout** at `~/Chinwag` (change `DEPLOY_DIR` in
   `cd.yml` if different). It must contain `infra/k3s/secret.yaml` (gitignored,
   so `git reset --hard` won't delete it):
   ```bash
   git clone git@github.com:Sephy314/Chinwag.git ~/Chinwag
   cp infra/k3s/secret.yaml.example infra/k3s/secret.yaml
   # …fill in real values…
   ```
2. **Passwordless sudo for `k3s`** — `update.sh` calls `sudo k3s ctr images import`
   and `sudo k3s kubectl`. A service-based runner has no TTY, so it must not prompt
   for a password. Add a NOPASSWD rule for the runner user:
   ```bash
   sudo visudo -f /etc/sudoers.d/chinwag
   # <runner-user> ALL=(root) NOPASSWD: /usr/local/bin/k3s
   ```
3. **`docker` access** for the runner user (add to the `docker` group).
4. **Readable kubeconfig** for the runner user — `update.sh`'s observability step
   (`observability/install.sh`) drives `helm`, which does **not** go through the
   `k3s` wrapper and needs a readable `KUBECONFIG`. Copy the k3s admin config to
   the runner user (it is also what `test.sh`'s `sudo k3s kubectl` fallback uses
   — NOPASSWD for `k3s` covers that path):
   ```bash
   sudo mkdir -p ~/.kube \
     && sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config \
     && sudo chown -R $(whoami):$(whoami) ~/.kube \
     && sudo chmod 600 ~/.kube/config
   ```
   `install.sh` also bootstraps `helm`/standalone `kubectl` into `~/.local/bin`
   if missing, so no package install is needed.

### 3. Branch protection (recommended)

On `main`, require pull requests and make the `CI` workflow a required status
check. Then CD only ever deploys tested code.

## Manual deploy

Run the CD workflow from the Actions tab (`Run workflow` button) to deploy the
current `main` without pushing.
