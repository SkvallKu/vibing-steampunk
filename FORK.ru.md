[English](FORK.md) | Русский

> Перевод [FORK.md](FORK.md). При расхождении верна английская версия.

# vibing-steampunk — форк с RFC/SAProuter

Форк [oisee/vibing-steampunk](https://github.com/oisee/vibing-steampunk), с которым
`vsp` работает с SAP-системами, доступными **только по classic RFC через SAProuter**:
без ADT/HTTPS-порта, без SAP NW RFC SDK и без pyrfc. Кроме того, в нём есть
исправления для старых релизов (7.40, 7.50), на которых чтение в апстриме падает.

Ветка `patched` — это `main` апстрима плюс коммиты ниже; при каждой синхронизации
она ребейзится. Теги `vX.Y.Z-patch.N` показывают, на каком релизе апстрима основана сборка.

## Патчи

| Коммит | Что делает |
|---|---|
| `feat(rfc): route classic RFC through a SAProuter` | `open-rfc-go` умеет `NI_ROUTE`, но `pkg/saprfc` не передавал маршрут в `rfc.Destination.Router`. Добавлены `--saprouter`, `rfc_saprouter` в `.vsp.json` и параметр `saprouter` у MCP-экшена `rfc`. Хоп `/W/<пароль>` в `--verbose` скрывается. |
| `fix(adt): tunnel the ordinary ADT client over RFC too` | С одним первым патчем маршрут использовала только явная `rfc`-поверхность; `pkg/adt.Client`, на котором построены `GetSource`, `GetClassInfo`, `SearchObject`, `Activate` и большинство инструментов, по-прежнему ходил по HTTP и падал по таймауту. Теперь при заданном `rfc_saprouter` ADT-клиент идёт через RFC-туннель (`SADT_REST_RFC_ENDPOINT`), CSRF на этом канале пропускается. Системы без роутера не затронуты. |
| `feat(rfc): opt-in CpicStreaming` | Запись и проверка синтаксиса через туннель падали на телах больше ~28000 байт (`CPIC streaming is disabled…`). Ограничение — захардкоженный флаг клиента в `open-rfc-go`, а не SAP Basis. Добавлен `rfc_cpic_streaming` (по умолчанию выключен). Нужен [форк open-rfc-go](https://github.com/SkvallKu/open-rfc-go). |
| `fix(rfc): pass rfc_cpic_streaming from .vsp.json to 'vsp rfc call'` | `vsp rfc call` брал из системы `.vsp.json` хост, логин и маршрут, но не `rfc_cpic_streaming`, поэтому с включённым флагом всё равно падал на телах больше 28000 байт. |
| `fix(adt): resolve package from object metadata` | Если quickSearch не вернул пакет объекта, пакет берётся из метаданных самого объекта, а не считается отсутствующим. |
| `feat(mcp): start the server for a .vsp.json system with -s` | Сервер игнорировал `-s`: RFC-туннель брал хост из системы `default`, а логин — из `SAP_USER`/`SAP_PASSWORD`, поэтому в одном проекте нельзя было поднять серверы на несколько систем или мандантов. Теперь `vsp -s <имя>` берёт подключение из этой системы. Включается явно: без `-s` ничего не меняется. |

### Старые релизы (7.40, 7.50)

Найдено на SAP_BASIS 740 SP06 и проверено на 750. Каждый коммит собирается и работает
сам по себе, так что апстрим может принять любой из них отдельно.

| Коммит | Что делает |
|---|---|
| `fix(adt): read classes right on 7.40, and GetClassInfo on every release` | На 7.40 object structure класса помечает метод как `CLAS/OO` (на 7.50+ — `CLAS/OM`), поэтому `GetSource … method=` и правка метода отвечали «method not found». `GetClassInfo` на любом релизе возвращал пустые методы и атрибуты, а у final-классов — `isFinal: false`; теперь он читает коды типов и атрибуты final/abstract из структуры, а суперкласс и интерфейсы — из исходника. |
| `fix(adt): table queries on releases with the classic SQL parser` | До 7.40 SP08 нет freestyle SQL, а data preview разбирает классический Open SQL: падают `SELECT a, b` и `ORDER BY a, b`, а также длинный запрос с `IN ('A', 'B')`. В запросе расставляются пробелы после запятых и внутри скобок, а при ответе 400 — один повтор без запятых между колонками. `RunQuery` (а с ним `GetSystemInfo` и `vsp query`) при отсутствии freestyle выполняет SELECT по одной таблице через data preview. |
| `feat(adt): GetTable and GetStructure answer from DD02L/DD03L` | Исходник DDIC-таблиц ADT отдаёт только с 7.52. При ответе 404 описание собирается из DD02L/DD02T/DD03L в форме DDL-исходника, а первая строка говорит, что оно сгенерировано. |

## Настройка

Приоритет маршрута: `--saprouter` > `rfc_saprouter` в `.vsp.json` >
`VSP_<SYS>_RFC_SAPROUTER` > `SAP_SAPROUTER`.

```jsonc
// .vsp.json
{
  "default": "prod",
  "systems": {
    "prod": {
      "url": "https://unused.example:44300",
      "user": "DEV", "client": "100",
      "rfc_host": "10.0.0.1",                        // сервер приложений, как его видит роутер
      "rfc_sysnr": "00",
      "rfc_saprouter": "/H/router.example.com/S/3299",
      "rfc_cpic_streaming": true                     // по желанию: тела > 28000 байт
    }
  }
}
```

Пароль берётся из `VSP_<SYS>_RFC_PASSWORD` или `SAP_PASSWORD` — в файл его не пишите.

Строка маршрута приводится к виду `/H/…/H/`, который ждёт `open-rfc-go`:
`/H/router/S/3299` → `/H/router/S/3299/H/`; пустой маршрут — прямое соединение.

**MCP, одна система:** задайте `SAP_SAPROUTER` (и `SAP_PASSWORD`) в `env` сервера в `.mcp.json`.
RFC-туннель берёт `rfc_host`/`rfc_sysnr` из системы `default` в `.vsp.json` рабочей
папки сервера, поэтому держите по одному `.vsp.json` на папку проекта, с `default`
на эту систему.

**MCP, несколько систем или мандантов:** запускайте каждый сервер с `-s <имя>`. Сервер
берёт URL, мандант, язык, пользователя, `rfc_*` и маршрут из этой системы, пароль — из
`VSP_<ИМЯ>_PASSWORD` (RFC: `rfc_user`, `VSP_<ИМЯ>_RFC_PASSWORD`, иначе тот же логин).
`SAP_USER`, `SAP_PASSWORD`, `SAP_SAPROUTER` и система `default` для подключения не
используются: обычно они относятся к другой системе, поэтому при нехватке настройки сервер
не стартует, а не входит под ними.

- Приоритет: флаги командной строки > система > переменные `SAP_*` и `.env`.
- Настройки безопасности системы только сужают флаги: `read_only`, `block_free_sql`,
  `transport_read_only` добавляются; `allowed_packages`/`allowed_transports` действуют,
  если флага нет, а список во флаге должен укладываться в список системы.
  `enable_transports` и `allow_transportable_edits` — только флагами.
- Системы с SSO и cookie с `-s` пока не поддерживаются (задавайте флагами).

```jsonc
// .mcp.json — без секретов; пароли — VSP_DEV_PASSWORD / VSP_DATA_PASSWORD в .env
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

## Сборка

Форк зависит от форка `open-rfc-go` через `replace` в `go.mod`, поэтому
`go install …@latest` **не работает** (там Go игнорирует `replace`). Берите бинарник
из релиза или собирайте из клона:

```sh
git clone -b patched https://github.com/SkvallKu/vibing-steampunk
cd vibing-steampunk
CGO_ENABLED=0 go build -o build/vsp ./cmd/vsp
```

Чтобы работать с обоими репо одновременно, склонируйте их рядом и создайте `go.work`
в родительской папке (не коммитьте его):

```sh
go work init ./vibing-steampunk ./open-rfc-go
```

## Ограничения

- ADT поверх RFC — stateless на каждый вызов; активация через туннель ограничена
  самим SAP, а не патчем.
- `vsp compat` по-прежнему опрашивает HTTP-порты напрямую и зависает на RFC-only системах.
- На 7.40 data preview не сообщает длину колонки и никогда не помечает ключевые поля,
  поэтому `GetTableContents` показывает там `Length: 0` и `IsKey: false`. Так отвечает
  SAP; сами данные верные.

## Синхронизация с апстримом

```sh
git fetch upstream
git rebase upstream/main patched
go build ./... && go test ./pkg/saprfc/... ./pkg/adt/... ./pkg/graph/... ./pkg/config/... ./internal/mcp/...
git tag vX.Y.Z-patch.N && git push --force-with-lease origin patched --tags
```

Патчи, принятые в апстрим, выпадают из `patched` при следующем ребейзе.
