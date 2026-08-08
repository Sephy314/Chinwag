# Chinwag CI/CD (GitHub Actions)

| Workflow | Trigger | What it does |
|---|---|---|
| `ci.yml` | push (any branch), pull_request | Backend: `make test` + `make vet` · Frontend: `npm ci` → `npm test` → `npm run lint` → `npm run build` |
| `cd.yml` | push to `main`, manual (`workflow_dispatch`) | Self-hosted runner on the k3s node syncs `/home/sephy314/Chinwag` to `origin/main` and runs `infra/k3s/update.sh` |

## CI

Runs on GitHub-hosted `ubuntu-latest` runners. No live infra (Postgres/Redis/NATS) is
required — backend tests are unit tests with mocks, and the only live-DB test
(`projectionRepo_test.go`) self-skips when no DB is reachable.

> Note: the frontend lockfile is `package-lock.json` (`npm`). The stale
> `frontend/pnpm-lock.yaml` in the repo is **not** used — keep `package-lock.json`
> in sync when changing dependencies.

## CD — self-hosted runner (no SSH needed)

Instead of SSH-ing in, a **GitHub Actions runner is installed directly on the k3s
node**. The CD job runs on that runner, so `update.sh` executes natively with
`docker`, `k3s`, and `kubectl` all available — no inbound ports, no SSH keys, no
repo secrets.

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

1. **Persistent checkout** at `/home/sephy314/Chinwag` (change `DEPLOY_DIR` in
   `cd.yml` if different). It must contain `infra/k3s/secret.yaml` (gitignored,
   so `git reset --hard` won't delete it):
   ```bash
   git clone git@github.com:Sephy314/Chinwag.git /home/sephy314/Chinwag
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

### 3. Branch protection (recommended)

On `main`, require pull requests and make the `CI` workflow a required status
check. Then CD only ever deploys tested code.

## Manual deploy

Run the CD workflow from the Actions tab (`Run workflow` button) to deploy the
current `main` without pushing.
