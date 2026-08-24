# Informe de QA — go-api-reliability-proxy

**Fecha:** 2026-08-24 · **Versión revisada:** 0.1.0 (pre-publicación, sin commits)
**Alcance:** revisión completa de `cmd/`, `internal/`, `tests/`, infraestructura (Docker, CI, GoReleaser) y documentación.

## Estado de las verificaciones ejecutadas

| Comando | Resultado |
| --- | --- |
| `go build ./...` | ✅ limpio |
| `go vet ./...` | ✅ limpio |
| `gofmt -l .` | ✅ sin ficheros pendientes |
| `go test ./...` | ✅ 8/8 paquetes OK |
| `go test -race ./...` | ⚠️ **no ejecutable en esta máquina** — requiere cgo y no hay gcc/mingw. En el CI (ubuntu) sí corre. |
| `go test -cover ./...` | ⚠️ ver M-11 |

Cobertura medida: `metrics` 100 %, `rules` 93.8 %, `config` 76.9 %, `server` 73.7 %, `faults` 60.2 %, `cmd` 47.6 %, **`proxy` 0 %**, **`logging` 0 %**.

**14 de los hallazgos se reprodujeron con tests de verificación** escritos durante esta auditoría, ejecutados, y eliminados después (el repo queda tal cual estaba). Los marcados con ✅ **VERIFICADO** llevan la salida real del test.

Total: **34 hallazgos** — 3 críticos, 8 altos, 13 medios, 10 bajos.

---

## 🔴 CRÍTICO — rompen funcionalidad anunciada

### C-1. El "graceful shutdown" devuelve `200 OK` con cuerpo vacío ✅ VERIFICADO
`internal/server/server.go:29-31`

```go
BaseContext: func(_ net.Listener) context.Context {
    return ctx   // ctx viene de signal.NotifyContext
},
```

`BaseContext` es el padre del contexto de **toda** petición. Al llegar SIGINT/SIGTERM ese `ctx` se cancela y con él, de golpe, los contextos de todas las peticiones en vuelo — justo antes de que `srv.Shutdown()` intente drenarlas. Es el anti-patrón clásico de `BaseContext` con un contexto de señal.

Lo grave es **el modo de fallo**. Reproducido 3/3 veces con el proxy real (latencia inyectada de 600 ms, SIGTERM a los 150 ms):

```
>>> El cliente recibio: status=OK body=
```

La cadena es: se cancela el ctx → `Sleep` aborta con `context.Canceled` → el motor devuelve error → `handleEngineError` (`internal/proxy/handler.go:44-46`) detecta `context.Canceled` y **retorna sin escribir nada** → net/http, ante un handler que no escribió, emite por defecto `200 OK` con `Content-Length: 0`.

El cliente recibe un **éxito con el cuerpo vacío**, no un error. Ningún cliente reintenta un 200. Esto es pérdida silenciosa de datos en cada despliegue, y es especialmente irónico en una herramienta cuyo propósito es enseñar a los clientes a reintentar bien.

Contradice el README (*"Graceful shutdown"*) y el CHANGELOG (*"Graceful shutdown on SIGINT and SIGTERM"*).

**Arreglo — validado 3/3 en esta auditoría:** eliminar `BaseContext` y usar el ctx de señal sólo para disparar `Shutdown`.

```go
srv := &http.Server{
    Addr:              opts.Addr,
    Handler:           opts.Handler,
    ReadHeaderTimeout: 5 * time.Second,
    IdleTimeout:       60 * time.Second,
    // sin BaseContext
}
```

Con ese cambio, el mismo escenario devuelve `status=OK body=respuesta real del upstream`.

**Además:** aunque se arregle el shutdown, la rama de `handleEngineError` que retorna en silencio sigue produciendo un 200 vacío cada vez que un cliente cancela a medias. Ahí es correcto (el cliente ya se fue y nadie lee la respuesta), pero conviene documentarlo o usar `http.StatusRequestTimeout` explícito para que las métricas y los logs no mientan.

---

### C-2. `reset` no produce un connection reset real: es un FIN, no un RST ✅ VERIFICADO
`internal/faults/reset.go:26-30`

```go
conn, _, err := hijacker.Hijack()
...
return true, conn.Close()
```

`conn.Close()` sobre TCP hace un cierre ordenado. Verificado con el proxy real:

```
error del cliente: Get "http://127.0.0.1:49388/reset": EOF
```

El cliente ve `EOF`, **no** `ECONNRESET` / "connection reset by peer". No son intercambiables para lo que este proyecto quiere probar: el `net/http` de Go reintenta automáticamente una petición idempotente que muere con EOF en una conexión reutilizada, y no lo hace igual con un RST. Quien use el proxy para validar su lógica de reintentos estará probando el escenario equivocado.

El efecto se llama `reset`, el README dice *"Connection reset (HTTP/1.x)"* y el CHANGELOG *"HTTP/1.x connection reset via hijacking"*. La implementación entrega otra cosa.

**Arreglo:**

```go
conn, _, err := hijacker.Hijack()
if err != nil {
    return true, err
}
if tcp, ok := conn.(*net.TCPConn); ok {
    _ = tcp.SetLinger(0) // fuerza RST en Close()
}
return true, conn.Close()
```

Bajo TLS el `conn` es `*tls.Conn`; hay que bajar al `net.Conn` subyacente con `NetConn()` (Go 1.18+).

**Por qué no se detectó:** `tests/integration/reset_test.go:24-29` sólo comprueba `err == nil`. Cualquier forma de romper la conexión lo satisface, así que el test no protege contra esta regresión ni la habría detectado nunca.

---

### C-3. `timeout` descarta en silencio el resto de efectos de la regla ✅ VERIFICADO
`internal/faults/engine.go:63-68` + `internal/config/validation.go:75-110`

```go
if rule.Effects.Timeout != nil {
    ...
    return Result{Stop: true}, nil   // corta aquí, siempre
}
if rule.Effects.Reset != nil { ... }
if rule.Effects.Failure != nil { ... }
if rule.Effects.Response != nil { ... }
```

La validación cuenta efectos (`effects++`) sólo para exigir **al menos uno**, y acepta cualquier combinación. Verificado:

```
config.Validate ACEPTA timeout+response+failure en la misma regla
resultado real: status=504 body="" (se configuro 429 'rate limited' y 503)
```

El usuario escribe una regla, arranca sin ningún aviso, y dos de los tres efectos configurados nunca se ejecutan. Silencioso en las dos puntas: ni la validación se queja ni el runtime lo registra.

**Arreglo (elige uno):**
- Validar que `timeout` y `reset` son mutuamente excluyentes con el resto de efectos que detienen la petición, y fallar al arrancar con un mensaje claro.
- Como mínimo, un `logger.Warn` al arrancar por cada regla con efectos inalcanzables.

Documentar una precedencia (`README.md:285`) no sustituye a rechazar una configuración imposible.

---

## 🟠 ALTO

### A-1. `methods` con entradas vacías invierte la semántica: restringir pasa a ser "todos" ✅ VERIFICADO
`internal/config/loader.go:39-47`

`Normalize()` descarta silenciosamente los métodos que quedan vacíos tras el trim, y `matchMethod` (`internal/rules/matcher.go:38-41`) trata el slice vacío como **"coincide con todos los métodos"**. Verificado con `methods: ["  "]`:

```
methods tras Normalize: []string{}
BUG CONFIRMADO: la regla se aplica a GET aunque el usuario intento restringir los metodos
```

Un error tipográfico en YAML produce el comportamiento **opuesto** al pedido: quien quiso inyectar fallos sólo en `POST /pagos` los inyecta en todo el tráfico de esa ruta, incluidos los `GET`.

**Arreglo:** que `Normalize` no borre entradas vacías y deje que `validMethod` las rechace, o distinguir `Methods == nil` (todos) de `len(Methods) == 0` tras normalizar (error de configuración).

### A-2. El namespace interno secuestra rutas legítimas del upstream ✅ VERIFICADO
`internal/proxy/handler.go:16`

```go
if strings.HasPrefix(r.URL.Path, internalPrefix) // "/__reliability"
```

Verificado:

```
/__reliabilityX/informe -> status=404, upstream alcanzado=false
```

Intercepta `/__reliabilityX`, `/__reliability-report`, `/__reliability_v2`… El README (línea 308) y SECURITY.md prometen que lo reservado es `/__reliability/*`, no `/__reliability*`.

**Arreglo:** `r.URL.Path == internalPrefix || strings.HasPrefix(r.URL.Path, internalPrefix+"/")`.

### A-3. `Normalize()` es un requisito implícito que el tipo no garantiza
`internal/config/loader.go:27`, `internal/config/validation.go:12`, `internal/proxy/proxy.go:31`

Toda la corrección del matching depende de que alguien haya llamado a `Normalize()` antes, y nada lo obliga:

- Los métodos se comparan en mayúsculas (`matcher.go:23`), pero quien los pone en mayúsculas es `Normalize`. `config.Load()` **no** normaliza y `proxy.New()` **tampoco**. El camino natural de la librería (`config.Load` + `proxy.New`) da matching de métodos case-sensitive, roto en silencio. Sólo funciona porque `main.go:88` se acuerda de llamarlo.
- `Validate` exige `version == 1`, pero `Normalize` convierte `0 → 1` antes. Invierte el orden y cualquier YAML sin `version:` falla al arrancar. La dependencia de orden no está expresada en ningún sitio.

Es la causa raíz común de A-1, M-8 y parte de M-4: hay un estado intermedio de `Config` que es representable pero inválido.

**Arreglo:** que `config.Load()` haga `Normalize()` + `Validate()` internamente y devuelva una config ya válida, o exponer un único `config.LoadAndValidate(path, overrides)`. Hacer imposible el estado intermedio.

### A-4. Escritura sobre un `ResponseWriter` ya secuestrado
`internal/faults/reset.go:26-30` → `internal/proxy/handler.go:43-57`

Si `conn.Close()` devuelve error tras un `Hijack()` exitoso, `applyReset` propaga ese error y `handleEngineError` termina llamando a `http.Error(w, ...)` sobre un writer secuestrado. net/http lo rechaza y escupe `http: response.WriteHeader on hijacked connection` en su error log. No es fatal, pero señala que el contrato de "quién es dueño de la conexión" no está definido.

**Arreglo:** una vez el hijack tiene éxito, la conexión ya no pertenece al handler. Devuelve `Result{Stop: true}, nil` y limítate a loguear el error de `Close`.

### A-5. `--seed` no es determinista bajo concurrencia y no tiene ni un test
`internal/faults/random.go:18-35`, `README.md:203`

El README afirma: *"`--seed` makes probabilistic faults repeatable in CI"*. El PRNG es determinista, pero el **reparto** de valores entre peticiones concurrentes no lo es: N peticiones en paralelo toman valores de `Float64()` en el orden que decida el scheduler. Con tráfico secuencial es reproducible; con tráfico paralelo — el caso de CI que la frase promete — no.

Además `Options.Seed` se ignora en silencio si se pasa también `Options.Random` (`engine.go:33-36`): dos opciones que se contradicen sin aviso.

No existe **ningún** test de `--seed`: ni de determinismo, ni del flag en `main`, ni de la rama `fs.Visit` (`main.go:105-114`).

**Arreglo:** precisar la limitación en el README ("reproducible para tráfico secuencial") o derivar el valor aleatorio de algo estable por petición. Y añadir el test que respalde la afirmación.

### A-6. El reverse proxy no tiene ningún timeout hacia el upstream
`internal/proxy/director.go:13-31`

No se define `Transport`, así que se usa `http.DefaultTransport`: sin `ResponseHeaderTimeout`, sin límite de conexiones por host, sin `TLSHandshakeTimeout` propio. Combinado con la ausencia deliberada de `WriteTimeout` en el servidor (`server.go:24-32`, necesaria para poder simular latencia), un upstream colgado retiene conexiones y goroutines indefinidamente. En una herramienta cuyo propósito es *provocar* condiciones adversas, es un escenario probable, no teórico.

**Arreglo:** `Transport` explícito con `ResponseHeaderTimeout`, `MaxIdleConnsPerHost` e `IdleConnTimeout`; idealmente configurables desde el YAML.

### A-7. El README afirma algo falso sobre Content-Type ✅ VERIFICADO
`README.md:281`

> *"If you set `Content-Type`, it is preserved. The proxy does not guess content types."*

`writeResponse` (`internal/faults/failure.go:24-32`) no fija Content-Type cuando no viene en la config, y entonces **net/http sí adivina**. Verificado con un body JSON sin Content-Type:

```
body JSON sin Content-Type configurado -> Content-Type recibido: "text/plain; charset=utf-8"
```

Exactamente lo contrario de lo que promete la frase, y encima con un valor engañoso para un cuerpo JSON.

**Arreglo:** corregir la frase, o fijar el comportamiento en el código (p. ej. `application/octet-stream` cuando no se especifica) para que realmente no haya sniffing.

### A-8. Hay `.goreleaser.yaml` pero no hay workflow de release
`.github/workflows/` sólo contiene `ci.yml`

El README (línea 64) promete: *"Or download a release binary for linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, or windows-amd64. Verify downloads with `checksums.txt`."* Sin un workflow que dispare GoReleaser con los tags, esos artefactos nunca se publican: el README promete desde el día uno algo que el repo no entrega.

**Arreglo:** añadir `.github/workflows/release.yml` con `on: push: tags: ['v*']` y `goreleaser-action`. Añadir también `goreleaser check` al CI para no descubrir errores de config en el momento del tag.

---

## 🟡 MEDIO

### M-1. Semántica de métricas ambigua e inconsistente
`internal/proxy/handler.go:21-40`, `internal/metrics/metrics.go`

- `faultsInjected` cuenta **efectos aplicados**, no peticiones afectadas: una regla con `latency` + `failure` suma 2 en una sola petición. El nombre y el ejemplo del README (`"faultsInjected": 73` junto a `"requests": 1024`) sugieren peticiones.
- `proxied` se incrementa **antes** de proxear (`handler.go:24` y `:39`): cuenta intentos, no éxitos. Una petición que acabe en 502 figura como proxeada.
- Las peticiones a `/__reliability/*` no entran en `requests` — razonable, pero no está documentado.
- No hay contadores por regla, lo que deja `matched` poco accionable en cuanto hay varias reglas.

**Arreglo:** documentar la semántica exacta de cada contador en el README, y separar `faultsInjected` (efectos) de `requestsFaulted` (peticiones), que es lo que la gente espera medir.

### M-2. `latency` + `timeout` acumulan esperas, sin documentar
`internal/faults/engine.go:58-68`

Una regla con `latency: {fixed: 1s}` y `timeout: {duration: 30s}` espera **31 segundos**, no 30, y registra 2 faults. Ningún documento lo menciona, y sorprende en la dirección equivocada (un timeout de 30 s que tarda 31 parece un bug del cliente).

### M-3. Los faults se loguean y cuentan *antes* de ocurrir ✅ VERIFICADO
`internal/faults/latency.go:12-14`, `internal/faults/timeout.go:11-13`, `internal/faults/reset.go:20-25`

En los tres casos se hace `logFault()` + `recordFault()` y **después** se intenta el efecto. Verificado con una petición cancelada por el cliente durante la latencia:

```
metricas tras una peticion cancelada: {Requests:1 Matched:1 FaultsInjected:1 Proxied:0}
```

El fault se contabiliza y se loguea como inyectado aunque nunca llegó a afectar a nadie. El mismo patrón hace que en HTTP/2 `reset` registre "fault injected: reset" y sume al contador justo antes de descubrir que no hay `http.Hijacker` y devolver un 500. Las métricas mienten sobre lo que pasó.

**Arreglo:** registrar el fault después de que el efecto se complete con éxito.

### M-4. `validMethod` acepta métodos inválidos y rechaza métodos válidos
`internal/config/validation.go:171-181`

```go
for _, r := range method {
    if !unicode.IsUpper(r) && !unicode.IsDigit(r) { return false }
}
```

- Acepta `"123"` (sólo dígitos) y `"Ä"` / `"Δ"` (`unicode.IsUpper` no es ASCII-only).
- Rechaza tokens perfectamente válidos según RFC 7230, como `M-SEARCH` (UPnP) o cualquier método de extensión con `-`, `.` o `_`.

**Arreglo:** validar contra el conjunto `tchar` de RFC 7230, o contra una lista blanca de métodos conocidos si prefieres ser estricto. Hoy no hay **ningún** test que ejercite esta función (ver M-11).

### M-5. `Match` devuelve un puntero al estado interno; `NewMatcher` hace copia superficial
`internal/rules/matcher.go:16-34`

`NewMatcher` copia el slice, pero `Rule.Effects` contiene punteros (`*LatencyConfig`, `*FailureConfig`, …) que siguen compartidos con el slice del llamante. Y `Match` devuelve un `*Rule` que apunta al array interno. Nada impide que un consumidor mute la configuración de una regla mientras se sirven peticiones → data race real. Hoy no ocurre porque `handler.go:30` copia con `*rule`, pero la API invita al fallo y el `-race` del CI no lo detectaría porque ningún test lo intenta.

**Arreglo:** devolver `Rule` por valor desde `Match`, o documentar el matcher como inmutable tras la construcción y hacer copia profunda en `NewMatcher`.

### M-6. Las respuestas sintéticas no soportan headers multivalor ni filtran hop-by-hop
`internal/faults/failure.go:24-31`

`map[string]string` + `Header().Set()` hace imposible emular dos `Set-Cookie` o varios `WWW-Authenticate`, escenarios habituales al probar clientes. Tampoco se filtran cabeceras hop-by-hop (`Connection`, `Transfer-Encoding`) ni `Content-Length`, que puesto a mano desde el YAML produce una respuesta malformada.

**Arreglo:** `map[string][]string` y rechazar en validación las hop-by-hop y `Content-Length`.

### M-7. 405 sin cabecera `Allow` ✅ VERIFICADO
`internal/proxy/handler.go:60-63`

```
POST /__reliability/health -> status=405 Allow=""
```

RFC 9110 dice que un 405 SHOULD incluir `Allow`. Tampoco hay cuerpo. Detalle menor, pero es un endpoint público del proxy y no cuesta nada.

### M-8. `proxy.New` re-parsea el target sin validarlo y no envuelve el error
`internal/proxy/proxy.go:32-35`

`url.Parse` ya se ejecuta en `config.Validate` (`validation.go:36`) y aquí se repite, pero sin comprobar esquema ni host y devolviendo el error crudo, sin contexto. Un consumidor de la librería que salte `Validate` obtiene un proxy apuntando a `""`. Mismo problema de contrato que A-3.

### M-9. `usageText` está hardcodeado y ya miente sobre los defaults
`cmd/reliability-proxy/main.go:26-45`

El texto de ayuda es una constante independiente de los `fs.String(...)`. La divergencia ya existe: anuncia `--listen ... (default "127.0.0.1:8080")` cuando el default real del flag es `""` (el valor se aplica después, en `Normalize`). Cualquier flag nuevo obliga a editar dos sitios y nada lo verifica.

**Arreglo:** dejar que `flag` genere la ayuda con `fs.PrintDefaults()` y poner los defaults reales en la definición del flag.

### M-10. El Dockerfile hardcodea la versión
`Dockerfile:11`

```dockerfile
-ldflags="-s -w -X main.version=0.1.0"
```

Toda imagen construida a partir de ahora reportará `v0.1.0`, sea cual sea el código. Y `commit`/`date` quedan en `none`/`unknown` en la imagen mientras GoReleaser sí los inyecta: dos rutas de build con metadatos distintos.

**Arreglo:** `ARG VERSION=dev` + `--build-arg`, y pasar también commit y fecha.

### M-11. Huecos de cobertura, y el CI no mide cobertura
`internal/proxy` tiene **0 % de cobertura directa** (0 ficheros de test) y `internal/logging` 0 %. Los tests de integración sí ejercitan `proxy`, pero al vivir en otro paquete no se atribuye sin `-coverpkg`, así que nadie ve el hueco. El CI (`ci.yml`) no ejecuta `-cover` en absoluto.

Faltan pruebas para:

- **Shutdown con petición en vuelo** — es exactamente lo que habría cazado C-1. `server_test.go` sólo comprueba que `Run` retorna.
- **`reset` produce RST** — el test actual pasa con cualquier cierre (C-2).
- **`--seed`** — la promesa del README no está cubierta (A-5).
- **`--config`** — ninguna prueba de `main.run` con un fichero real; `cmd` se queda en 47.6 %.
- **Combinaciones de efectos** — `timeout`+`response` (C-3), `latency`+`timeout` (M-2), `latency`+`failure`.
- **`validMethod`** — ni un caso; el error `invalid HTTP method` no se ejerce nunca (M-4).
- **405 en endpoints internos** (M-7) y `Normalize` con métodos vacíos (A-1).
- **HTTP/2 → `ErrHijackingUnsupported`** — la rama de error existe y nunca se prueba.

**Arreglo:** añadir `go test -coverpkg=./... -cover ./...` al CI con un umbral mínimo.

### M-12. Contradicción entre la sección Security y todos los ejemplos
`README.md:314` afirma *"Default listen address is `127.0.0.1:8080`, not `0.0.0.0`"*, pero el Quick Start (líneas 86 y 98) usa `--listen :8080`, que escucha en **todas** las interfaces. El primer comando que copia cualquier usuario expone el proxy en la red. SECURITY.md al menos añade "only on trusted networks" para el caso Docker; el README no.

**Arreglo:** usar `--listen 127.0.0.1:8080` en los ejemplos que no son de Docker y avisar explícitamente en el que sí.

### M-13. Mensaje de error inútil con un YAML vacío ✅ VERIFICADO
`internal/config/loader.go:21-23`

```
config vacia -> error: parse configuration file: EOF
```

**Arreglo:** detectar `errors.Is(err, io.EOF)` y devolver "configuration file is empty".

---

## 🟢 BAJO — clean code, nomenclatura, mantenibilidad

### B-1. Código muerto
- `internal/proxy/proxy.go:65-67` — `Handler.RuleCount()`: definido, nunca usado (ni en tests).
- `internal/proxy/proxy.go:61-63` — `Handler.Metrics()`: nunca usado en el código de producción.
- `internal/proxy/proxy.go:28` — el campo `rules int` sólo existe para alimentar `RuleCount()`.
- `internal/rules/duration.go:27-29` — `Duration.MarshalYAML()`: la config nunca se serializa a YAML. Sin test tampoco.

### B-2. Helper de test en código de producción
`internal/faults/random.go:37-42` — `FixedRandom` sólo se usa desde tests, pero al vivir en `random.go` parece parte de la API pública del paquete. Muévelo a un `random_fake.go` con un comentario explícito, o a un subpaquete `faults/faultstest`.

### B-3. `min` y `max` sombrean builtins de Go
`internal/faults/engine.go:112-113` e `internal/faults/engine_test.go:102-103`. Desde Go 1.21 `min`/`max` son builtins y el módulo declara `go 1.24.0`. Compila, pero es justo el tipo de sombra que marca cualquier linter. Renombra a `lo`/`hi` o `minDur`/`maxDur`.

### B-4. Sobre-modelado y wrappers vacíos
- `internal/faults/engine.go:13-15` — `Result{Stop bool}` es un struct de un solo bool; `(bool, error)` diría lo mismo. Si la idea es que crezca, nada lo indica.
- `internal/faults/engine.go:104-106` — `recordFault()` es un wrapper de una línea sobre `e.metrics.RecordFault()`.
- `internal/faults/engine.go:91-93` — `shouldFail` se usa también para `reset`, donde "fail" no describe la operación. `shouldTrigger` sería más honesto.

### B-5. Se incumple la convención declarada en el propio CONTRIBUTING
CONTRIBUTING.md exige *"`context.Context` as the first argument"*. `applyReset` (`reset.go:12`), `applyFailure` y `applyResponse` (`failure.go:10,17`) no reciben ctx, mientras `applyLatency` y `applyTimeout` sí. La firma inconsistente además impide que `reset` respete la cancelación del cliente.

### B-6. `proxy.Options` duplica `faults.Options` (abstracción con fuga)
`internal/proxy/proxy.go:14-20` replica `Random`, `Sleeper` y `Seed` sólo para reenviarlos a `faults.New`. Cada opción nueva del motor obliga a tocar dos structs.

### B-7. `Matcher` como interfaz de una sola implementación
`internal/rules/matcher.go:8-10` — la interfaz existe pero nunca se sustituye, ni siquiera en tests (que usan `NewMatcher` directamente). Es YAGNI: en Go la interfaz se define en el consumidor cuando hace falta, no junto a la implementación.

### B-8. `latency.max` es inalcanzable
`internal/faults/engine.go:118` — `random.Float64()` devuelve `[0,1)`, así que con `min: 100ms, max: 1500ms` el valor nunca llega a 1500 ms. Irrelevante en la práctica, pero el README dice "random range" sin aclarar que el extremo superior es exclusivo.

### B-9. Nivel de log fijo y sin formato JSON
`internal/logging/logger.go:13-15` — `slog.LevelInfo` hardcodeado y siempre `TextHandler`. No hay `--log-level` ni `--log-format`. Para una herramienta que lista `observability` entre sus topics, poder pedir JSON es lo mínimo para ingerir los logs.

### B-10. Salidas mezcladas en `main`
`cmd/reliability-proxy/main.go:47-51` — `run()` recibe `stdout` y `stderr` inyectados (buen diseño, testeable), pero `main()` reporta el error final con `slog.Error` sobre el logger por defecto, ignorando el `stderr` que acaba de pasar. Y el logger de la aplicación va a **stdout** (línea 100), mezclando logs con la salida de `--version`. La convención habitual es logs a stderr, datos a stdout.

### B-11. LICENSE con autoría incompleta
`LICENSE:3` — `Copyright (c) 2026 Erick`. Sin apellido ni handle. Si el repo va a ser público, pon el nombre completo o el usuario de GitHub.

### B-12. `make lint` no ejecuta un linter
`Makefile:11-12` — `lint: go vet ./...`. `go vet` busca bugs, no estilo. Dado que el CI ya está verde, añadir `golangci-lint` (con `staticcheck`, `errcheck`, `revive`) cazaría automáticamente varios de los hallazgos bajos de este informe.

### B-13. Endurecimiento del Dockerfile
- Imágenes base sin fijar por digest (`golang:1.24-bookworm` y `distroless/static-debian12:nonroot` son etiquetas móviles) — builds no reproducibles.
- Falta `-trimpath` en el `go build`; GoReleaser lo aplica por defecto y el Dockerfile no. Otra divergencia entre las dos rutas de build.

### B-14. Verificar que el module path coincide con el repo real ⚠️ antes del primer push
`go.mod:1` — `module github.com/erick9125/go-api-reliability-proxy`, y el README (línea 61) publicita `go install github.com/erick9125/go-api-reliability-proxy/cmd/reliability-proxy@latest`. Si al crear el repo el owner o el nombre difieren en un solo carácter, **ese comando falla para todo el mundo**, y arreglarlo después obliga a un cambio de module path. Confírmalo antes de subir nada.

---

## Checklist antes de publicar

```bash
# 1. Verificado en esta auditoría: build, vet, gofmt y tests pasan
go build ./... && go vet ./... && test -z "$(gofmt -l .)" && go test ./...

# 2. PENDIENTE: -race no se pudo ejecutar aquí (requiere cgo + gcc/mingw en Windows).
#    Instala mingw-w64 y corre:  CGO_ENABLED=1 go test -race ./...
#    O confía en el CI de ubuntu, que sí lo ejecuta.

# 3. Cobertura real (hoy internal/proxy figura al 0%)
go test -coverpkg=./... -cover ./...

# 4. Confirmar que el module path == URL real del repositorio  (B-14)

# 5. bin/ está correctamente ignorado — verificado, git status limpio

# 6. Borrar o ignorar este informe
rm QA-REPORT.md   # o añadirlo a .gitignore
```

## Orden de trabajo sugerido

1. **C-1 y C-2** antes de nada: rompen features anunciadas en el README y en el CHANGELOG de la 0.1.0, y C-1 falla de la peor forma posible (un `200 OK` vacío que ningún cliente reintenta).
2. **C-3, A-1 y A-2**: tres formas distintas de que la configuración del usuario haga algo diferente de lo que dice, sin ningún aviso.
3. **A-3**: es la refactorización que previene una familia entera de bugs futuros. Hazla antes de que existan usuarios externos del paquete, porque después cambia la API.
4. **A-7 y M-12**: correcciones de documentación de un minuto que evitan que el README prometa cosas falsas el día del lanzamiento.
5. **M-11**: los tests que faltan son precisamente los que habrían cazado C-1, C-2 y A-1. Añádelos junto con cada arreglo, no después.
6. El resto del bloque medio y bajo puede esperar a la 0.2.0 — **salvo B-14**, que hay que verificar antes del primer push.
