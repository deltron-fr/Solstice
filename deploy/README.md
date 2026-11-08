# Deploying Solstice

The production stack runs the Go API and the static React app on an Ubuntu/Debian VM. GitHub Actions builds and pushes Linux x86-64 images to the private `deltronfr/solstice` Docker Hub repository. Caddy is included in the frontend image and provides HTTPS, SPA routing, the `/api` proxy, and the `/solana-rpc` proxy. The Anchor program runs on Solana and is not a container.

Docker Compose has no `build:` entries: it only pulls images. Run Compose on the VM; running it on a laptop would pull and store the images there. The VM necessarily stores the images it runs.

## 1. Prepare Docker Hub and GitHub

Create a **private** Docker Hub repository named `solstice` under `deltronfr`. Add a Docker Hub access token as the GitHub repository secret `DOCKERHUB_TOKEN`, with permission to push to that repository. The workflow uses username `deltronfr`.

Add these GitHub repository variables:

| Variable | Value |
| --- | --- |
| `SOLSTICE_DOMAIN` | Your public hostname, without `https://` |
| `SOLSTICE_SOLANA_PROGRAM_ID` | The deployed Solstice program ID on devnet |
| `SOLSTICE_USDC_MINT` | The accepted devnet test token mint |

The program ID and mint are public addresses. The frontend image embeds them during its build, and the deployment script checks that the backend `.env` uses the same values.

Add these GitHub repository secrets:

| Secret | Value |
| --- | --- |
| `VM_HOST` | VM IP address or SSH hostname |
| `VM_USER` | SSH deploy user |
| `VM_SSH_KEY` | Private SSH key for that user |
| `VM_SSH_KNOWN_HOSTS` | Verified `known_hosts` entry for `VM_HOST` |
| `DOCKERHUB_TOKEN` | Docker Hub token with push access |

Create `VM_SSH_KNOWN_HOSTS` from the VM host key, then verify its fingerprint against the VM provider console before saving it. The deploy workflow enforces strict host-key checking.

## 2. Prepare the VM

Install Docker Engine and the Compose plugin using Docker's [official Ubuntu instructions](https://docs.docker.com/engine/install/ubuntu/) (or the matching Debian instructions). Allow inbound TCP ports 80 and 443, plus UDP 443 for HTTP/3. Keep SSH access restricted to your administration source where possible.

Create the deployment directory and make it writable by the SSH deploy user. Replace `deploy-user` with the account saved as `VM_USER` in GitHub:

```sh
sudo install -d -o deploy-user -g deploy-user -m 0750 /opt/solstice
```

Log in to Docker Hub on the VM as that deploy user with a separate token that has **read-only** access to the private repository:

```sh
docker login --username deltronfr
```

At the prompt, paste the read token. Do not put it in shell history or in `.env`.

From the repository root, copy the sample environment file to `/opt/solstice/.env` on the VM (replace the example account and hostname), then edit the domain, devnet program ID, and test mint on the VM:

```sh
scp deploy/production.env.example deploy-user@your-vm-host:/opt/solstice/.env
ssh deploy-user@your-vm-host
cd /opt/solstice
chmod 600 /opt/solstice/.env
```

Set `SOLSTICE_DOMAIN` to the hostname whose DNS already points to this VM. Set `SOLSTICE_ALLOWED_ORIGINS` to exactly `https://<hostname>`. Keep `SOLSTICE_COOKIE_SECURE=true`. Use the same program ID and mint as the GitHub variables. Do not use mainnet funds or a production USDC mint for this devnet deployment.

Caddy obtains and renews the HTTPS certificate automatically. DNS must resolve publicly to the VM, and ports 80 and 443 must be reachable for certificate issuance. Caddy stores certificates in a persistent Docker volume.

## 3. Deploy

Push to `main` or run **Test, publish, and deploy Solstice** manually from GitHub Actions with the `main` branch selected. The workflow tests Go and the frontend, builds and pushes `backend-main` and `frontend-main` plus immutable commit tags, copies the Compose manifest and deploy script to the VM, verifies domain/program/mint consistency, and starts the services. CI builds on GitHub-hosted runners and does not load images onto your laptop.

After the first deployment, on the VM:

```sh
cd /opt/solstice
docker compose ps
docker compose logs --tail=100 backend frontend
```

Useful commands from `/opt/solstice`:

```sh
docker compose up -d       # pulls current main tags and starts services
docker compose pull        # download images without restarting services
docker compose logs -f     # follow application logs
docker compose down        # stop services; named data volumes are retained
```

The backend applies embedded SQLite migrations at startup. Its database and uploaded listing images live in the `solstice_data` volume. Back up that volume before releases that change the schema, and keep off-VM backups for anything you need to retain. `docker compose down -v` deletes persistent volumes and must not be used for routine shutdown.

## 4. Image tags and rollback

The moving `backend-main` and `frontend-main` tags are used for routine deployment. Each CI build also publishes commit-specific tags such as `backend-<commit-sha>` and `frontend-<commit-sha>`. For a manual rollback, set `SOLSTICE_IMAGE_TAG` in `/opt/solstice/.env` to the same prior commit SHA for both services, then run `docker compose up -d --pull always --wait`. Only roll back application code when its schema is compatible with the migrations already applied; the workflow does not reverse database migrations.

## Scope

This deploys the marketplace app configured for Solana devnet. It does not deploy or upgrade the Anchor program. Program upgrades remain a separate, explicit on-chain operation. Solana RPC is reachable through the same-origin `/solana-rpc` endpoint, so the browser does not need direct DNS/network access to the RPC host. If using a provider URL with a private API key, remember that the public proxy can be called by site visitors; use an endpoint or provider policy suitable for that exposure.
