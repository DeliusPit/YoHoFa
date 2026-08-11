# Deployment naar Proxmox LXC

Deze flow gebruikt een kleine Debian/Ubuntu LXC met SQLite op lokale disk.

## LXC Aanmaken

Maak in de Proxmox GUI een container aan via `Create CT`:

1. Template: Debian 12/13 of Ubuntu LTS.
2. Disk: minimaal 4 GB, liever 8 GB.
3. CPU: 1 core.
4. Memory: 512 MB of 1 GB.
5. Network: bridge `vmbr0`, DHCP of vast IP.
6. Start de container en log in als `root`.

## Deployen

Vereisten:

- lokaal: Node.js en npm
- in de LXC: Go en SSH-toegang voor root

Gebruik vanaf de projectmap op je ontwikkelmachine:

```sh
LXC_HOST=<lxc-ip> AUTH_PASSWORD='<sterk-wachtwoord>' ./scripts/install-lxc.sh
```

De standaard gebruikersnaam is `geldzaken`. Stel desgewenst ook `AUTH_USERNAME` in.

Het script bouwt de frontend lokaal, verstuurt de huidige werkmap via `tar` en SSH, bouwt de Go-binary in de LXC en configureert SQLite, systemd en HTTP Basic Authentication. Git is niet nodig.

Controleer daarna:

```sh
ssh root@<lxc-ip> systemctl status --no-pager geldzaken-api
curl -u geldzaken:'<sterk-wachtwoord>' http://<lxc-ip>:8080/healthz
```

## Data

SQLite-data staat in:

```text
/var/lib/geldzaken/geldzaken.db
```

Neem dit bestand mee in backups.
