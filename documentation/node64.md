# Node64

Node64 (ipv64.net, Apache-2.0) is a single-binary homelab monitoring tool:
health checks with status pages, plus reverse proxy, DNS filter, DynDNS and
honeypot modules. AuraGo offers it in the Software Store as a Docker app.

## Install and log in

1. Install **Node64** from the Software Store on the Virtual Desktop. The image
   is small and is healthy a few seconds after the start.
2. The Store creates the administrator on the first start. The user name is
   `admin`; open the Store's credentials view for the generated password.
3. Open Node64 from the Desktop. Node64 forbids being embedded in another page,
   so it opens in its own browser tab (`http://127.0.0.1:<port>`, or the Tailscale
   address if you enabled it).

The user name and password are used when Node64 creates the administrator on
the first start; a restart on the existing data does not create it again. If you
change the password inside Node64 afterwards, the Store credentials view still
shows the original generated value.

## What the Store provides, and what it does not

- Only the web UI port is published, on this machine by default. Node64's DNS
  filter (53), reverse proxy (80/443) and similar listeners are **not** exposed
  by the Store; modules that need those ports will not be reachable from other
  machines.
- The container runs as an unprivileged user with a read-only root file system,
  a small temporary `/tmp`, all Linux capabilities dropped and
  `no-new-privileges`. Data lives in the Docker volume `data` (`/data`).
- Without capabilities, **ICMP ping checks do not work**. TCP and HTTP(S) checks
  work normally.
- The Store does not mount the Docker socket into Node64.
- LAN access is optional; the login protects the page, but use a strong password
  and a trusted network before exposing it.

## Update and uninstall

An Update pulls the pinned image and recreates the container; the data volume
is kept. Uninstall keeps the volume unless you choose **Delete data**, which also
removes the stored admin password. Reinstalling over a kept volume keeps the
existing Node64 administrator.

The Store pins the image by digest (Node64 1.4.0). New Node64 versions arrive
with an AuraGo release that updates the pin.
