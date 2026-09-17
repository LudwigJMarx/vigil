# Running vigil for real

vigil is one static binary and one SQLite file. There is no runtime to install,
no database server to run and no migration step to remember: the binary brings
its schema up to date when it opens the file, and refuses to open one written by
a newer version rather than guessing.

## Verified targets

Built with `CGO_ENABLED=0`, so the binary is static and the SQLite driver is
pure Go.

| OS | Architecture |
|---|---|
| linux | amd64, arm64 |
| darwin | amd64, arm64 |
| windows | amd64 |

## On a server, with systemd

```bash
sudo useradd --system --home /var/lib/vigil --create-home vigil
sudo install -m 0755 vigil /usr/local/bin/vigil
sudo -u vigil VIGIL_DB=/var/lib/vigil/vigil.db vigil token create --name browser
```

`/etc/systemd/system/vigil.service`:

```ini
[Unit]
Description=vigil signal layer
After=network-online.target

[Service]
User=vigil
Group=vigil
Environment=VIGIL_DB=/var/lib/vigil/vigil.db
Environment=VIGIL_ADDR=127.0.0.1:8099
Environment=VIGIL_LOG=json
ExecStart=/usr/local/bin/vigil serve
Restart=on-failure

# vigil needs one directory and one port. Everything else can be taken away.
NoNewPrivileges=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/vigil
ProtectKernelTunables=yes
ProtectControlGroups=yes
RestrictAddressFamilies=AF_INET AF_INET6
RestrictNamespaces=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now vigil
curl -s http://127.0.0.1:8099/healthz
```

## TLS

vigil speaks plain HTTP and will keep doing so. Terminating TLS is a solved
problem that every reverse proxy solves better than an application server, and
an application server that does it badly is worse than one that does not try.

Bind vigil to loopback and put a proxy in front. With Caddy the whole
configuration is:

```caddyfile
vigil.example.com {
	reverse_proxy 127.0.0.1:8099
}
```

With nginx, the part that matters:

```nginx
location / {
    proxy_pass http://127.0.0.1:8099;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

If you do bind vigil to a non-loopback address directly, it refuses to start
until at least one token exists, and it says on startup that the connection is
not encrypted.

## Backup

```bash
sudo -u vigil sqlite3 /var/lib/vigil/vigil.db ".backup '/var/backups/vigil-$(date +%F).db'"
```

vigil runs in WAL mode, so copying the `.db` file alone while the server is
running can miss committed data sitting in `-wal`. Use `.backup`, or stop the
service first and copy all three files.

Restoring is putting the file back. There is no other state.

## Upgrading

Replace the binary and restart. The schema migrates forward on open. Going back
to an older binary after a migration is refused with a message naming both
versions, because a silently downgraded database is a database that loses
columns.

## Sizing

One salesperson's book is a few thousand signals. The account list reads every
signal on every request, on purpose: a rule with a half-life of zero does not
fade, so scoring a recent window would quietly drop weight the operator
configured to be permanent. At the size vigil targets, the full read is cheaper
than a number that is wrong by design. If your instance grows past that, the
fix is a materialised score with an explicit refresh, not a window.
