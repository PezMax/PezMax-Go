# Luminous CI/CD

`/.github/workflows/ci.yml` validates every push to `main` and `Luminous` and
every pull request. It tests and builds Go, the Vue admin site, and the Windows
desktop client; it also builds the three server images and runs the isolated
edge egress regression. Only a successful **push** run for the exact latest
`Luminous` SHA is eligible for deployment. `main`, pull requests, and manual
workflow runs never deploy.

The server is reachable only on a private address, so GitHub-hosted runners do
not SSH to it. A systemd timer on `10.177.2.79` polls GitHub's public Actions
API every five minutes. The poller checks the CI result, fetches that commit
through the server's local Clash proxy, and extracts it into
`/home/Luminous/pezmax-cd/releases/<sha>`. It builds immutable SHA-tagged
`kadmin`, `admin-web`, and `edge` images before replacing any container. The
server's existing `/home/Luminous/PezMax-Go/.env` is linked into each release;
secrets never enter the repository or GitHub Actions.

The poller runs Compose with `--no-deps` for the three application services.
It does not recreate PostgreSQL, Redis, MinIO, or their named volumes. It
checks the backend, both Nginx health checks, and the admin site's API proxy.
If an update fails, it restores the previously running application images.
Schema migrations and desktop installers are not automated by this workflow.

## PostgreSQL pgvector (one-time)

Compose pins PostgreSQL to `pgvector/pgvector:pg16` (same 16 major version as
before, same data directory). New volumes create `vector` and `pg_trgm`
automatically through `k_admin/sql/extensions.sql`. An existing volume does not
re-run init scripts, so enable the extension once:

```sh
cd /home/Luminous/PezMax-Go
sudo install -m 755 deploy/cd/enable-pgvector.sh /usr/local/sbin/pezmax-pgvector
sudo /usr/local/sbin/pezmax-pgvector --dry-run
sudo /usr/local/sbin/pezmax-pgvector
```

The script resolves the Compose project from the `pezmax-go-postgres` container
labels, dumps the application database to `/var/backups/pezmax`, pulls the
pgvector image, recreates only the `postgres` service, runs
`CREATE EXTENSION vector/pg_trgm`, verifies HNSW index creation, and restores
the previous image if any step fails. The data volume is never deleted. Expect
a few seconds of database downtime; run it in a maintenance window.

Useful flags: `--check` exits non-zero when the extension is missing,
`--skip-backup` skips the dump (not recommended), and
`PEZMAX_DB_BACKUP_DIR` / `PEZMAX_DB_IMAGE` / `PEZMAX_DB_CONTAINER` override
defaults. Docker image pulls go through the Docker daemon, not the server's
Clash HTTP proxy; if the registry is unreachable, configure the daemon proxy
first. The poller only warns when the extension is missing and does not
recreate the database automatically.

Verify:

```sh
docker exec pezmax-go-postgres sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT extversion FROM pg_extension WHERE extname = '"'"'vector'"'"'"'
```

## Server setup

Run once on the server after the workflow has been pushed and passed:

```sh
cd /home/Luminous/PezMax-Go
sudo install -m 755 deploy/cd/deploy-luminous.sh /usr/local/sbin/pezmax-cd
sudo install -m 644 deploy/cd/pezmax-cd.service /etc/systemd/system/
sudo install -m 644 deploy/cd/pezmax-cd.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo /usr/local/sbin/pezmax-cd --dry-run
sudo systemctl enable --now pezmax-cd.timer
sudo systemctl start pezmax-cd.service
```

The server's bundled `/usr/local/bin/git` lacks HTTPS support, so the poller
uses `/usr/bin/git` and `PEZMAX_GIT_PROXY` (default
`http://127.0.0.1:7890`). The service unit sets `PEZMAX_BUILD_PROXY` to
`http://host.docker.internal:7890` for Docker build steps and
`PEZMAX_DEBIAN_MIRROR` to the directly reachable Aliyun Debian mirror. Apt
still validates signed Debian repository metadata and package hashes. The build needs
Docker Compose v2, Python 3,
`curl`, `tar`, `flock`, and adequate disk space. The admin site is exposed on
port `18088` at the same bind address as the Go API; override the port with
`PEZMAX_ADMIN_WEB_PORT` in the server `.env` if needed.

```sh
sudo systemctl status pezmax-cd.timer
sudo journalctl -u pezmax-cd.service -n 100 --no-pager
cat /home/Luminous/pezmax-cd/state/current.sha
sudo docker compose --profile app --profile edge ps
```

To pause deployment without stopping the application, disable the timer:

```sh
sudo systemctl disable --now pezmax-cd.timer
```

The installed poller is deliberately a fixed copy rather than a script
executed from an unverified new commit. After changing the poller or systemd
units, review them and reinstall the changed files with the setup commands.
