English | [Русский](FORK.ru.md)

# vibing-steampunk — RFC/SAProuter fork

This is a fork of [oisee/vibing-steampunk](https://github.com/oisee/vibing-steampunk)
that lets `vsp` work with SAP systems reachable **only over classic RFC through a
SAProuter** — no ADT/HTTPS port, no SAP NW RFC SDK, no pyrfc. It also carries
fixes for older releases (7.40, 7.50) where upstream's reads fail.

The branch `patched` is upstream `main` plus the commits listed below, rebased on
each upstream sync. Tags `vX.Y.Z-patch.N` mark the upstream release a build is based on.

## Patches

| Commit | What it does |
|---|---|
| `feat(rfc): route classic RFC through a SAProuter` | `open-rfc-go` already speaks `NI_ROUTE`, but `pkg/saprfc` never passed a route to `rfc.Destination.Router`. Adds `--saprouter`, `rfc_saprouter` in `.vsp.json`, and a `saprouter` parameter on the MCP `rfc` action. A `/W/<password>` hop is redacted in `--verbose` output. |
| `fix(adt): tunnel the ordinary ADT client over RFC too` | With only the first patch, just the explicit `rfc` surface used the route; `pkg/adt.Client` — behind `GetSource`, `GetClassInfo`, `SearchObject`, `Activate` and most other tools — still dialled HTTP and timed out. When `rfc_saprouter` is set, the ADT client now runs over the RFC tunnel (`SADT_REST_RFC_ENDPOINT`), and CSRF handling is skipped on that channel. Systems without a router are untouched. |
| `feat(rfc): opt-in CpicStreaming` | Writes and syntax checks through the tunnel failed above ~28000 bytes (`CPIC streaming is disabled…`). The limit was a hardcoded client flag in `open-rfc-go`, not SAP Basis. Adds `rfc_cpic_streaming` (off by default). Needs the [open-rfc-go fork](https://github.com/SkvallKu/open-rfc-go). |
| `fix(rfc): pass rfc_cpic_streaming from .vsp.json to 'vsp rfc call'` | `vsp rfc call` took the host, logon and route of a `.vsp.json` system but not `rfc_cpic_streaming`, so it still failed above 28000 bytes with the flag set. |
| `fix(adt): resolve package from object metadata` | When quickSearch omits the package of an object, it is read from the object's own metadata instead of being reported as missing. |
| `feat(mcp): start the server for a .vsp.json system with -s` | The server ignored `-s`: the RFC tunnel took its host from the `default` system and its logon from `SAP_USER`/`SAP_PASSWORD`, so one project could not run servers for several systems or clients. `vsp -s <name>` now takes the connection from that system. Opt-in: without `-s` nothing changes. |

### Older releases (7.40, 7.50)

Found on SAP_BASIS 740 SP06 and checked against 750. Each commit builds and works
on its own, so upstream can take any of them separately.

| Commit | What it does |
|---|---|
| `fix(adt): read classes right on 7.40, and GetClassInfo on every release` | On 7.40 the class object structure types a method as `CLAS/OO` (7.50+: `CLAS/OM`), so `GetSource … method=` and method edits answered "method not found". `GetClassInfo` returned no methods or attributes on any release, and `isFinal: false` for final classes; it now reads the structure's type codes and final/abstract attributes, and superclass and interfaces from the source. |
| `fix(adt): table queries on releases with the classic SQL parser` | Before 7.40 SP08 there is no freestyle SQL, and data preview parses classic Open SQL: `SELECT a, b` and `ORDER BY a, b` fail, and so does a long statement with `IN ('A', 'B')`. Statements get blanks after commas and inside parentheses, and on a 400 one retry without the commas between columns. `RunQuery` (and with it `GetSystemInfo` and `vsp query`) answers a single-table SELECT through data preview when freestyle is missing. |
| `feat(adt): GetTable and GetStructure answer from DD02L/DD03L` | ADT serves DDIC table source only from 7.52. On a 404 the definition is written from DD02L/DD02T/DD03L in the shape of the DDL source, with a first line saying it is generated. |

## Configuration

Route precedence: `--saprouter` > `rfc_saprouter` in `.vsp.json` >
`VSP_<SYS>_RFC_SAPROUTER` > `SAP_SAPROUTER`.

```jsonc
// .vsp.json
{
  "default": "prod",
  "systems": {
    "prod": {
      "url": "https://unused.example:44300",
      "user": "DEV", "client": "100",
      "rfc_host": "10.0.0.1",                        // application server, as the router sees it
      "rfc_sysnr": "00",
      "rfc_saprouter": "/H/router.example.com/S/3299",
      "rfc_cpic_streaming": true                     // optional: bodies > 28000 bytes
    }
  }
}
```

The password comes from `VSP_<SYS>_RFC_PASSWORD` or `SAP_PASSWORD` — never put it in the file.

Route strings are normalised to the `/H/…/H/` form `open-rfc-go` expects:
`/H/router/S/3299` → `/H/router/S/3299/H/`; an empty route means a direct connection.

**MCP, one system:** set `SAP_SAPROUTER` (and `SAP_PASSWORD`) in the server's `env` in `.mcp.json`.
The RFC tunnel reads `rfc_host`/`rfc_sysnr` from the `default` system of the
`.vsp.json` in the server's working directory, so keep one `.vsp.json` per project
folder with `default` pointing at that system.

**MCP, several systems or clients:** start each server with `-s <name>`. The server
then takes URL, client, language, user, `rfc_*` and the route from that system, and the
password from `VSP_<NAME>_PASSWORD` (RFC: `rfc_user`, `VSP_<NAME>_RFC_PASSWORD`, else the
same logon). `SAP_USER`, `SAP_PASSWORD`, `SAP_SAPROUTER` and the `default` system are not
used for the connection: they usually belong to another system, so a missing setting stops
the server instead of logging on with them.

- Precedence: command-line flags > the system > `SAP_*` variables and `.env`.
- Safety in the system only narrows the flags: `read_only`, `block_free_sql`,
  `transport_read_only` are added; `allowed_packages`/`allowed_transports` apply when the
  flag is absent, and a flag list must stay within the system's. `enable_transports` and
  `allow_transportable_edits` stay flags only.
- SSO and cookie systems are refused with `-s` for now (use the flags).

```jsonc
// .mcp.json — no secrets; passwords are VSP_DEV_PASSWORD / VSP_DATA_PASSWORD in .env
{ "mcpServers": {
  "vsp":      { "command": "vsp", "args": ["-s", "DEV", "--mode", "focused", "--allowed-packages", "$TMP"] },
  "vsp-data": { "command": "vsp", "args": ["-s", "DATA", "--mode", "focused", "--read-only",
                                           "--transport-read-only", "--disabled-groups", "RTDHXIGC"] }
} }
```

```sh
vsp -s prod rfc info --verbose
vsp -s prod rfc adt GET /sap/bc/adt/core/discovery
vsp -s prod source CLAS ZCL_SOMETHING
```

## Build

The fork depends on the `open-rfc-go` fork through a `replace` in `go.mod`, so
`go install …@latest` does **not** work (Go ignores `replace` there). Use a release
binary or build from a clone:

```sh
git clone -b patched https://github.com/SkvallKu/vibing-steampunk
cd vibing-steampunk
CGO_ENABLED=0 go build -o build/vsp ./cmd/vsp
```

To work on both repositories at once, clone them side by side and add a `go.work`
in the parent directory (do not commit it):

```sh
go work init ./vibing-steampunk ./open-rfc-go
```

## Limitations

- ADT over RFC is stateless per call; activation over the tunnel is limited by SAP
  itself, not by the patch.
- `vsp compat` still probes HTTP ports directly and hangs on RFC-only systems.
- On 7.40 the data preview reports no column length and never marks a key column,
  so `GetTableContents` shows `Length: 0` and `IsKey: false` there. That is what SAP
  sends; the data is right.

## Syncing with upstream

```sh
git fetch upstream
git rebase upstream/main patched
go build ./... && go test ./pkg/saprfc/... ./pkg/adt/... ./pkg/graph/... ./pkg/config/... ./internal/mcp/...
git tag vX.Y.Z-patch.N && git push --force-with-lease origin patched --tags
```

Patches that upstream accepts are dropped from `patched` on the next rebase.
