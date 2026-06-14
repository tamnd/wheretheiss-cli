---
title: "Configuration"
description: "Environment variables, defaults, and the data directory."
weight: 20
---

wheretheiss needs almost no configuration: it runs anonymously against public
data out of the box. The settings below let you tune politeness and storage.

## Defaults

| Setting | Default | Flag |
|---|---|---|
| Requests | paced and retried on 429/5xx | `--rate`, `--retries` |
| Per-request timeout | 30s | `--timeout` |
| On-disk cache | under the data directory | `--no-cache` to bypass |

## The data directory

Caches and any record store live under one data directory, chosen in this order:

1. `--data-dir`
2. `WHERETHEISS_DATA_DIR`
3. `$XDG_DATA_HOME/wheretheiss`
4. `~/.local/share/wheretheiss`

## Environment variables

Every flag has an environment fallback, prefixed `WHERETHEISS_` in
upper case with dashes as underscores. For example:

```bash
export WHERETHEISS_RATE=1s        # same as --rate 1s
export WHERETHEISS_DATA_DIR=~/data/wheretheiss
```

Flags win over environment variables, which win over the built-in defaults.

## Sending records to a store

`--db` tees every emitted record into a store as a side effect of reading, so a
session fills a local database without a separate import step:

```bash
wheretheiss page <path> --db out.db        # SQLite file
wheretheiss page <path> --db 'postgres://...'
```
