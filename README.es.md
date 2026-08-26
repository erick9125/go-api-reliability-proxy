# Go API Reliability Proxy

Un proxy HTTP de inyección de fallos para probar la resiliencia de APIs con latencia, timeouts, fallos, rate limits, connection resets y reglas de tráfico configurables.

Go API Reliability Proxy se coloca entre tu aplicación y una API HTTP y introduce a propósito latencia, fallos de servidor, timeouts, cortes de conexión y otras condiciones adversas para validar reintentos, timeouts, idempotencia, fallbacks y recuperación.

[English](README.md)

## Problema

Al desarrollar un cliente HTTP normalmente pruebas esto:

```
request → API → 200 OK
```

En producción se parece más a:

```
request → latency → 429 → retry → timeout → 503 → network reset → eventual success
```

El proxy queda entre el cliente y la API:

```
Aplicación → Reliability Proxy → API real
```

Puede manipular el tráfico sin modificar la aplicación ni el backend.

## Características

- Reverse proxy HTTP con streaming del body
- CLI con `flag` de la standard library
- Configuración YAML
- Matching por path exacto y prefijo `/*`
- Matching por método HTTP
- La primera regla que coincide gana
- Latencia fija y aleatoria
- Fallos probabilísticos
- Respuestas HTTP sintéticas
- Timeout simulado
- Connection reset (HTTP/1.x)
- Passthrough si ninguna regla aplica
- Métricas internas
- Logs estructurados
- Apagado graceful
- Tests unitarios, de integración y race detector

El proxy **no** hace retries. Inyecta condiciones para que *tu* cliente las haga.

## Instalación

```bash
go install github.com/erick9125/go-api-reliability-proxy/cmd/reliability-proxy@latest
```

O descarga un binario de GitHub Releases. Verifica con `checksums.txt`.

## Inicio rápido

```bash
reliability-proxy \
  --target http://localhost:3000 \
  --config reliability.yaml \
  --listen :8080
```

El cliente apunta a `http://localhost:8080`. El backend real sigue en `http://localhost:3000`.

## Configuración

Los flags CLI tienen prioridad sobre el YAML, y el YAML sobre los valores por defecto.

La dirección de escucha por defecto es `127.0.0.1:8080` (no `0.0.0.0`).

Ejemplos en `examples/`. Una configuración inválida impide el arranque.

`--version` imprime `reliability-proxy v0.1.0`.

`--seed` repite las mismas decisiones probabilísticas con tráfico **secuencial**, que es lo que hace repetible una corrida guionada en CI. No puede hacer determinista el tráfico concurrente: las peticiones en paralelo toman valores del generador en el orden en que el scheduler les entrega el lock, así que la misma semilla asigna decisiones distintas a peticiones distintas. Si además se pasa una fuente aleatoria explícita por código, esta gana y el proxy registra un warning.

## Matching de reglas

Las reglas se evalúan en orden de declaración. Se aplica la primera coincidencia.

- `/users` es exacto
- `/users/*` es prefijo (`/users` y `/users/...`)
- Sin `methods`, coinciden todos los métodos
- No hay regex en 0.1

## Efectos

Orden: latencia → timeout → reset → failure → response.

Gana el primer efecto que corta la petición, así que una regla no puede declarar un efecto que nunca llegaría a ejecutarse. Las combinaciones donde un efecto tapa permanentemente a otro se rechazan al arrancar en vez de ignorarse en silencio: `timeout` junto a `reset`, `failure` o `response`, y un `reset` sin probabilidad (o con `probability: 1.0`) junto a `failure` o `response`. Siguen siendo válidas las combinaciones con un camino alcanzable: `latency` con cualquier cosa, `failure` + `response`, y un `reset` con probabilidad menor que 1.

- **Latencia:** espera cancelable con `context.Context`, luego continúa hacia upstream salvo otro efecto de corte.
- **Failure:** con probabilidad `p` responde un status sintético y no llama al backend.
- **Response:** siempre responde de forma sintética si la regla coincidió (y failure no cortó antes).
- **Timeout:** espera `duration` y, si el cliente sigue conectado, responde `504`.
- **Reset:** cierra la conexión con `Hijacker`. Pensado para HTTP/1.x.

Simulación de rate limit en 0.1: `429` + `Retry-After`. No hay token bucket.

## Métricas

- `GET /__reliability/health` → `{"status":"ok"}`
- `GET /__reliability/status` → contadores `requests`, `matched`, `faultsInjected`, `proxied`

El namespace `/__reliability/*` está reservado.

## Seguridad

- Bind por defecto en localhost
- Un solo target fijo por proceso; el request no elige el upstream
- No se registran Authorization, Cookie, bodies ni query strings completos
- No se persiste tráfico
- No lo expongas como proxy abierto

## Limitaciones

HTTP solamente. Un target. Sin UI, Kubernetes, Redis, gRPC, WebSockets, grabación de tráfico ni hot reload.

## Docker

```bash
docker run --rm -p 8080:8080 \
  -v ./reliability.yaml:/config.yaml \
  reliability-proxy \
  --config /config.yaml \
  --listen :8080
```

En Docker hay que escuchar en `:8080` para que el puerto publicado funcione.

## Tests

```bash
go test ./...
go test -race ./...
go vet ./...
```

## Roadmap

- **0.2:** hot reload, rate limiter real, matching por regex/headers/query, Prometheus
- **0.3:** throttling, respuestas parciales, secuencias de escenario
- **0.4:** múltiples upstreams, perfiles, OpenTelemetry

## Contribuir

Ver [CONTRIBUTING.md](CONTRIBUTING.md).

## Licencia

MIT. Ver [LICENSE](LICENSE).
