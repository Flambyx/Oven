# Oven 🔥

> Turn a YAML file into a bootable Linux ISO.

Oven is a CLI tool that builds custom bootable Linux ISOs from a simple YAML recipe. Declare your base distro, packages, files, and services — Oven handles the rest and outputs an ISO ready to install anywhere.

## Installation

```bash
git clone https://github.com/Flambyx/oven
cd oven
go build -o oven .
```

## Usage

```bash
oven cook                  # uses recipe.yaml in current directory
oven cook -f my-recipe.yaml
```

## Recipe format

```yaml
apiVersion: v0.1
name: my-iso

base:
  distro: ubuntu
  version: "24.04"

packages: #Not implemented
  - nginx
  - git

files: #Not implemented
  - src: ./files/nginx.conf
    dest: /etc/nginx/nginx.conf

services: #Not implemented
  enable:
    - nginx
  disable:
    - snapd

users:
  - name: username
    sudo: true
    ssh_keys:
      - "ssh-ed25519 AAAA..."

locale:
  timezone: America/New_York
  lang: en_US.UTF-8
```

## Supported distros

| Distro | Status |
|--------|--------|
| Ubuntu | ✅ |
| Arch   | 🔜 |
| Oracle Linux | 🔜 |

## License

Apache 2.0