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
| `fix(mcp): GetTextElements and SetTextElements over ADT, not the ZADT_VSP WebSocket` | Both tools went through the ZADT_VSP WebSocket, which dials the ICM port directly and never takes the SAProuter route — and the RFC tunnel carries request/response ADT calls only — so on an RFC-only system they timed out even with ZADT_VSP installed. They now use the text elements resource, like `texts_get`/`texts_set` and `vsp texts`. |
| `fix(mcp): SearchObject that finds nothing answers [], not null` | An empty result was serialized as `null`. |
| `fix(adt): GetInstalledComponents reads the Atom feed SAP answers with` | `/sap/bc/adt/system/components` answers with an Atom feed (7.40, 7.50), and the tool always failed with `expected element type <components> but have <feed>`. Each entry is read as component, release, SP level, support package (new field `package`) and description. |
| `fix(adt): vsp deploy writes a source that has warnings only` | Every syntax check message counted as an error, so a warning ("redundant conversion") stopped the deploy, and on create left an empty object behind. As in `WriteSource`, only severity E, A and X stop it; warnings are returned and printed. |
| `fix(adt): wrapSQL breaks lines with CR LF` | A freestyle statement longer than 255 characters is wrapped, but with a bare LF, and the service splits lines at CR LF only: it failed with "more than 255 characters in line 1" (7.50). Lines are broken with CR LF, and so is a line break the caller wrote. |
| `fix(saprfc): the RFC tunnel survives GENERATE_SUBPOOL_DIR_FULL` | The tunnel keeps one ABAP session, and each data preview query generates a temporary subroutine pool in it; after some thirty-six queries every query dumped until vsp restarted. A runtime error now drops (and closes) the connection, and that one dump — it happens before the query runs — is retried once on a fresh one. |
| `fix(adt): read a class include whose name is exactly thirty characters` | A thirty-character class name leaves no `=` padding (`/IWFND/CL_SODATA_POST_PRO_XLSXCM001`), and such an include was read as a program. |
| `fix(adt): function groups in a namespace` | The pool of `/BEV1/EM0` is `/BEV1/SAPLEM0` and its includes `/BEV1/LEM0…`; the prefixes were put before the namespace, so namespaced groups were not recognised and their modules' callees were not found. |
| `fix(mcp): find a function module by its URI, whatever the logon language` | `GetCallersOf` and the call graph with `object_type=FUNC` compared the search hit's name, which a Russian logon gets as "BAL_MSG_DISPLAY_ABAP (Функциональный модуль)"; the module was reported as missing. The name is taken from the URI. |
| `fix(adt): GetObjectStructure answers for more than classes` | Every name was read as a class, so a function group answered 400 "class does not exist" and a form an ADT error. Classes still use their objectstructure; interfaces, programs, function groups and forms (SFPF, SFPI) are read from the repository node structure, grouped by kind with each component's URI. `object_type` is optional: without it the name is looked up. Other types are refused, since for a table the node structure lists unrelated objects. |
| `fix(adt): a syntax check SAP did not run is not a clean source` | For an include with no main program, checkrun answers `notProcessed` with no messages, which read as no errors: `SyntaxCheck`, `WriteSource INCL` and deploy called unchecked code clean. That report is now a warning with SAP's text (a warning, since a new include has no main program yet). |
| `fix(adt): deploy refuses a file whose name and source name different objects` | deploy writes the object the source names; an include file that kept its main program's `REPORT` line would have replaced the main program. When an abapGit-style file name and the source disagree, deploy stops. |
| `fix(cli): adt request says a lock does not outlive the call` | Each `adt request` run is its own ADT session (over HTTP the cookie lives in memory, over the RFC tunnel the session is the connection), so a lock handle from one run is invalid in the next, `--stateful` or not. The help says so and points to WriteSource, EditSource and deploy; a LOCK request ends with a note on stderr. |

### Older releases (7.40, 7.50)

Found on SAP_BASIS 740 SP06 and checked against 750. Each commit builds and works
on its own, so upstream can take any of them separately.

| Commit | What it does |
|---|---|
| `fix(adt): read classes right on 7.40, and GetClassInfo on every release` | On 7.40 the class object structure types a method as `CLAS/OO` (7.50+: `CLAS/OM`), so `GetSource … method=` and method edits answered "method not found". `GetClassInfo` returned no methods or attributes on any release, and `isFinal: false` for final classes; it now reads the structure's type codes and final/abstract attributes, and superclass and interfaces from the source. |
| `fix(adt): table queries on releases with the classic SQL parser` | Before 7.40 SP08 there is no freestyle SQL, and data preview parses classic Open SQL: `SELECT a, b` and `ORDER BY a, b` fail, and so does a long statement with `IN ('A', 'B')`. Statements get blanks after commas and inside parentheses, and on a 400 one retry without the commas between columns. `RunQuery` (and with it `GetSystemInfo` and `vsp query`) answers a single-table SELECT through data preview when freestyle is missing. |
| `feat(adt): GetTable and GetStructure answer from DD02L/DD03L` | ADT serves DDIC table source only from 7.52. On a 404 the definition is written from DD02L/DD02T/DD03L in the shape of the DDL source, with a first line saying it is generated. |
| `fix(adt): a release without the text elements resource is an error, not an empty text pool` | 7.50 has no `/sap/bc/adt/textelements`: every document answers 404 "No application class found", and the read took that as an empty kind, so a program with a hundred texts read as none. That 404 is now told apart from a missing kind and reported as a missing resource. |
| `fix(adt): a line break in a data preview query is white space on 7.40 too` | 7.40 SP06 does not take a line break for white space: in `SELECT *\nFROM dd03l\nWHERE …` the table name and `WHERE` read as one name. A line break or tab outside quotes becomes a blank. |
| `fix(adt): note a 100-row answer to a large data preview request` | 7.40 SP06 data preview answers a request above a threshold between 5,000 and 9,999 rows with its default of 100 rows, no error and no total, so `all_rows` came back as 100 rows that looked complete. Such a result now carries a `Note`: it may not be all of it, ask for at most 5,000. |
| `fix(adt): report why a data preview failed, not the commas it retried without` | When the retry without commas failed too, the first attempt's error was returned — on 7.40 SP06 always the one about the commas ("explicit length specifications are necessary with types C, P, X and N in the OO context"), which hid the real fault, such as an unknown field. The retry's error now comes first. |
| `fix(adt): data preview on 7.40 puts a blank before a comma too` | A long statement with `IN ( 'A', 'B' )` failed with 'Following "', '" a blank is required', depending on where the commas fell and on the digits in `max_rows`. 7.40's `CL_ADT_DP_OPEN_SQL_HANDLER` cuts a statement of 255 characters or more into lines before tokens and starts the last line one character early; when that token is a comma, the quote before it is doubled (7.50 has the line commented out). With a blank before every comma the doubled character is a blank. |
| `feat(adt): who calls an object, from the cross-reference tables on 7.40` | 7.40 SP06 has no where-used list (`usageReferences` answers 404), so `GetCallersOf`, the callers call graph and dump impact failed. On a 404 they read CROSS, WBCROSSGT and D010INC, with TFDIR, TLIBG, TRDIR and TADIR for the owning object and package; the answer names this source and its caveats, and says which table could not be read or was cut at its row limit. `GetCallersOf` also takes `TABL`, `DTEL`, `TTYP`. |
| `feat(adt): where-used falls back to the tables on its conversion 500 too` | On 7.50 the where-used list answers 500 "Error while converting object references" for tables and function modules that SE84 answers; that 500 alone also goes to the tables, and `source` says why. |
| `fix(adt): GetFunctionGroup names the group's includes, and its sources are read on 7.40/7.50` | `GetFunctionGroup` returns the group's includes (new field `Includes`) beside its modules. A group's objectstructure answers 404 on 7.40 and 7.50, so `tr-boundaries`, the transport analysis and the CR audit could not read any function group; the sources are now listed from the repository node structure there. |

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
- Text elements cannot be read or written on 7.50 (checked), and so not on 7.40 either: ADT has no resource for
  them there, and vsp now says so instead of answering "no texts". A path over RFC
  (`RPY_PROGRAM_READ`, `RPY_TEXTELEMENTS_INSERT`) is not written yet.
- `RunReport`, `RunReportAsync`, `GetVariants`, `CallRFC` (the MCP tool; `vsp rfc call`
  works) and the AMDP debugger still need the ZADT_VSP WebSocket, which cannot be
  reached through a SAProuter.
- Callers read from the cross-reference tables (7.40, and the 7.50 500 above) are coarser than SE84: a declaration of a type counts as a use; a call of an inherited method counts for the class that defines it, so a superclass lists its subclasses' users (`CL_SALV_FORM_UIE_LABEL`: SE84 9, tables 277); `SUBMIT` shows up where SE84 is silent; a class caller has no method; dynamic calls are not recorded. Very used objects (MARA) are answered from the first 5,000 rows and marked `incomplete`.

## Syncing with upstream

```sh
git fetch upstream
git rebase upstream/main patched
go build ./... && go test ./pkg/saprfc/... ./pkg/adt/... ./pkg/graph/... ./pkg/config/... ./internal/mcp/...
git tag vX.Y.Z-patch.N && git push --force-with-lease origin patched --tags
```

Patches that upstream accepts are dropped from `patched` on the next rebase.
