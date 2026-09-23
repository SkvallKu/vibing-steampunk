English | [Русский](FORK.ru.md)

# vibing-steampunk — RFC/SAProuter fork

This is a fork of [oisee/vibing-steampunk](https://github.com/oisee/vibing-steampunk)
that lets `vsp` work with SAP systems reachable **only over classic RFC through a
SAProuter** — no ADT/HTTPS port, no SAP NW RFC SDK, no pyrfc.

The branch `patched` is upstream `main` plus the commits listed below, rebased on
each upstream sync. Tags `vX.Y.Z-patch.N` mark the upstream release a build is based on.

## Patches

| Commit | What it does |
|---|---|
| `feat(rfc): route classic RFC through a SAProuter` | `open-rfc-go` already speaks `NI_ROUTE`, but `pkg/saprfc` never passed a route to `rfc.Destination.Router`. Adds `--saprouter`, `rfc_saprouter` in `.vsp.json`, and a `saprouter` parameter on the MCP `rfc` action. A `/W/<password>` hop is redacted in `--verbose` output. |
| `fix(adt): tunnel the ordinary ADT client over RFC too` | With only the first patch, just the explicit `rfc` surface used the route; `pkg/adt.Client` — behind `GetSource`, `GetClassInfo`, `SearchObject`, `Activate` and most other tools — still dialled HTTP and timed out. When `rfc_saprouter` is set, the ADT client now runs over the RFC tunnel (`SADT_REST_RFC_ENDPOINT`), and CSRF handling is skipped on that channel. Systems without a router are untouched. |
| `feat(rfc): opt-in CpicStreaming` | Writes and syntax checks through the tunnel failed above ~28000 bytes (`CPIC streaming is disabled…`). The limit was a hardcoded client flag in `open-rfc-go`, not SAP Basis. Adds `rfc_cpic_streaming` (off by default). Needs the [open-rfc-go fork](https://github.com/SkvallKu/open-rfc-go). |
| `fix(adt): resolve package from object metadata` | When quickSearch omits the package of an object, it is read from the object's own metadata instead of being reported as missing. |

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

**MCP:** set `SAP_SAPROUTER` (and `SAP_PASSWORD`) in the server's `env` in `.mcp.json`.
The RFC tunnel reads `rfc_host`/`rfc_sysnr` from the `default` system of the
`.vsp.json` in the server's working directory, so keep one `.vsp.json` per project
folder with `default` pointing at that system.

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

## Syncing with upstream

```sh
git fetch upstream
git rebase upstream/main patched
go build ./... && go test ./pkg/saprfc/... ./pkg/adt/... ./pkg/config/... ./internal/mcp/...
git tag vX.Y.Z-patch.N && git push --force-with-lease origin patched --tags
```

Patches that upstream accepts are dropped from `patched` on the next rebase.
