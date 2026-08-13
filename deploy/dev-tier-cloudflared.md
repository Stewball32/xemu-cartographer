# DEV tier — dev.norcal.pro cloudflared cutover

Staged steps for Stewart to make **dev.norcal.pro** live. Nothing here is applied
automatically — the live tunnel config is left untouched.

The dev tier is the live-reload-from-repo layer:

| tier | dir | PB port | web port | public host |
|------|-----|--------:|---------:|-------------|
| prod | `/var/lib/xemu-cartographer` | 8099 | (same) | lan.norcal.pro |
| beta | `~/xcarto-beta` (built snapshot) | 18099 | (same) | beta.norcal.pro |
| **dev** | `~/repos/xemu-cartographer` (working tree) | **19090** | **19099** (Vite) | **dev.norcal.pro** |

cloudflared routes `dev.norcal.pro` → the **Vite** port (19099); Vite proxies
`/api` + `/_` → the dev PocketBase (19090). The PB port stays private.

## 1. Add the ingress rule (before the `http_status:404` catch-all)

Edit `/etc/cloudflared/config.yml` and insert the dev hostname **above** the
final catch-all `- service: http_status:404` line:

```yaml
ingress:
  - hostname: lan.norcal.pro
    service: http://localhost:8099
  - hostname: beta.norcal.pro
    service: http://localhost:18099
  - hostname: dev.norcal.pro        # ← add this block
    service: http://localhost:19099 #   (Vite dev server)
  - service: http_status:404
```

## 2. Create the public DNS record for the tunnel

```sh
cloudflared tunnel route dns xemu-cartographer dev.norcal.pro
```

(One-time; adds a proxied CNAME `dev.norcal.pro` → the tunnel. Reversible by
deleting the record in the Cloudflare dashboard.)

## 3. Reload cloudflared to pick up the new ingress

```sh
sudo systemctl restart cloudflared
```

## 4. Start the dev tier (from the repo)

```sh
cd ~/repos/xemu-cartographer
./run-dev.sh                       # foreground (Ctrl-C stops both)
# or background:
nohup ./run-dev.sh > dev.log 2>&1 &
# stop:
pkill -f '.air.dev.toml'; pkill -f 'vite.config.dev'
```

Then `https://dev.norcal.pro` serves the working tree with HMR; edits reload
instantly. The PocketBase admin UI is at `https://dev.norcal.pro/_/`.

## Notes
- **No Discord bot on dev** — `run-dev.sh` unsets `DISCORD_BOT_TOKEN` (constant
  restarts would rate-limit the gateway / fight prod's bot). OAuth-only.
- Dev's `pb_data` is ephemeral (`tmp-dev/`, wiped on exit) and built with
  `-tags dev`, so the in-process seeder provides dev users/data on each start.
- HMR travels over the tunnel via `wss://dev.norcal.pro:443` (configured in
  `sveltekit/vite.config.dev.ts`); `allowedHosts` already includes
  `dev.norcal.pro`.
