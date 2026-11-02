# Logging y observabilidad

## Diseño

`vuelingcli` es un proceso CLI de corta duración. Usa `log/slog` de la biblioteca estándar y un
logger inyectado desde `cmd/vueling/main.go`; la CLI y el cliente HTTP comparten configuración y
destino, y cada uno añade su `component`. El logger escribe en stderr. stdout conserva su contrato
actual: resultados de texto, JSON o TOON sin eventos operativos mezclados.

`observability.New(writer, config)` recibe el destino como `io.Writer`, por lo que la composición
puede sustituir stderr al integrar o probar el logger. El ejecutable usa stderr deliberadamente para
dejar stdout disponible para datos de máquina.

Los handlers estándar de `slog` permiten escrituras concurrentes seguras y codifican los registros
JSON. Cada evento lleva timestamp UTC, nivel en minúsculas, mensaje, `service=vuelingcli` y
`environment`. Los eventos añaden `component`, `event_name`, `operation`, `duration_ms`, `outcome`,
`status_code`, `error_type` y `error_code` cuando esos datos ayudan a diagnosticar la operación.

No hay base de datos, daemon, cola ni servicio de observabilidad que mantener. No se añaden archivos
de log, rotación local, SaaS, métricas ni OpenTelemetry. Cada proceso ejecuta un comando, puede hacer
peticiones HTTP y termina; duración, resultado, host remoto y estado HTTP aportan la señal necesaria
sin generar identificadores de traza artificiales. `request_id`, `correlation_id`, `trace_id` y `span_id` no se
emiten porque no existe una entrada HTTP ni un salto entre servicios que los propague.

## Configuración

| Variable | Valores | Predeterminado |
| --- | --- | --- |
| `VUELING_ENV` | etiqueta de entorno | `development` |
| `VUELING_LOG_LEVEL` | `trace`, `debug`, `info`, `warn`/`warning`, `error` | `info` |
| `VUELING_LOG_FORMAT` | `text`, `json` | `text`; `json` cuando `VUELING_ENV=production` |

`VUELING_LOG_FORMAT` puede cambiar el formato predeterminado de cualquier entorno. Un nivel o
formato no reconocido conserva el valor predeterminado y genera `logging.configuration.invalid`
indicando únicamente el nombre de la variable inválida.

```sh
# Desarrollo: texto legible, operaciones y errores de comando
VUELING_ENV=development VUELING_LOG_LEVEL=info vueling flights BCN BIO --date 2026-11-12

# Producción: JSON en stderr; detalle HTTP y de navegador bajo demanda
VUELING_ENV=production VUELING_LOG_LEVEL=debug vueling calendar BCN BIO --date 2026-11-12 --prices

# TRACE incluye el inicio del comando y de cada petición HTTP
VUELING_LOG_LEVEL=trace vueling status VY1234
```

Los registros se pueden recoger con el mecanismo del sistema que ejecute el proceso. Por ejemplo,
`vueling version 2>vueling.stderr`; la retención y rotación corresponden a ese mecanismo. Las
escrituras son síncronas para evitar una cola que se pierda al terminar el proceso. El nivel INFO
mantiene el volumen bajo; DEBUG y TRACE añaden un evento por cada petición o lectura del navegador.
Los errores de escritura del destino de `slog` se ignoran para que el logger no cambie el resultado
del comando. La salida stderr puede incluir además el diagnóstico de texto de la CLI cuando un
comando falla; los registros JSON del logger sí son líneas JSON independientes.

## Niveles y eventos

| Nivel | Uso en este CLI |
| --- | --- |
| TRACE | Inicio del comando y de una petición HTTP; desactivado por defecto. |
| DEBUG | Resultado, duración y estado de peticiones HTTP, lecturas de Chrome, apertura del navegador y operaciones de caché de sesión. |
| INFO | Inicio y fin del proceso, fin del comando y cancelación de un watch. |
| WARN | Reintento recuperable de `calendar --watch` o `status --watch`, tarifas cercanas opcionales no disponibles, y configuración inválida con fallback. |
| ERROR | Fallo del comando en su límite, con tipo de error y estado HTTP cuando se conoce. |
| FATAL/CRITICAL | No se usa: los errores normales se devuelven al CLI y no hay un subsistema recuperable cuya caída requiera terminar el proceso. |

Eventos actuales:

- `process.started`, `process.stopped`, `command.started`, `command.finished`.
- `http.request.started`, `http.request.finished`.
- `session.cache.read`, `session.cache.write`, `session.cache.remove`.
- `browser.page_read.finished`, `browser.open.finished`.
- `watch.retry`, `watch.stopped`, `flights.nearby_fares.unavailable`.

La petición HTTP registra método, host, código de estado, resultado y duración, nunca ruta,
parámetros ni cuerpo. El inicio y fin del comando contextualizan el trabajo del proceso; los watches
añaden `operation=calendar` o `operation=status`, intervalo de reintento y motivo codificado.

## Privacidad e integridad

- No se registran tokens de sesión, cabeceras, cookies, URL completas, parámetros de consulta,
  cuerpos HTTP, contenido de páginas, scripts ni rutas de caché.
- Las respuestas HTTP no exitosas se reducen a estado y a una indicación segura de bloqueo CDN;
  el cuerpo no se conserva en `StatusError` ni se presenta en su mensaje.
- Los errores de transporte conservan su causa para `errors.Is`/`errors.As`, pero su mensaje externo
  no incluye la URL. Los errores de Apple Events no repiten la salida de la página o del script.
- El handler sustituye por `[REDACTED]` atributos cuyos nombres indican token, contraseña,
  autorización, cookie, secreto, clave privada o credencial. Los caracteres de control se convierten
  en espacios y los valores de texto se limitan a 256 caracteres; el encoder JSON también escapa sus
  caracteres de formato.
- Los mensajes de eventos son constantes. No se adjuntan errores crudos, argumentos del usuario,
  URLs ni datos de negocio.

## Consultar eventos

Con `VUELING_ENV=production`, los registros del logger son JSON Lines y se pueden filtrar por
`event_name` o `operation`. Ejemplo del evento `command.finished`:

```json
{"timestamp":"2026-11-02T09:30:00Z","level":"info","message":"Command completed","service":"vuelingcli","environment":"production","component":"cli","event_name":"command.finished","operation":"version","duration_ms":0,"outcome":"success","exit_code":0}
```

Ejemplo del resultado de una llamada HTTP en DEBUG:

```json
{"timestamp":"2026-11-02T09:30:00Z","level":"debug","message":"HTTP request completed","service":"vuelingcli","environment":"production","component":"api","event_name":"http.request.finished","operation":"http_request","duration_ms":142,"outcome":"success","method":"GET","remote_host":"apiw.vueling.com","status_code":200}
```

Ejemplo de recuperación de un watch:

```json
{"timestamp":"2026-11-02T09:30:00Z","level":"warn","message":"Watch read failed; retry scheduled","service":"vuelingcli","environment":"production","component":"cli","event_name":"watch.retry","operation":"calendar","retry_in_ms":1800000,"outcome":"retrying","error_type":"*errors.errorString"}
```

Para una nueva operación, usa el logger que recibe su componente y registra un evento de resultado
con nombre estable, duración, `outcome` y solo atributos técnicos que no incluyan datos de entrada.
Deja que el límite superior registre el error una vez; los niveles inferiores pueden registrar
resultados DEBUG sin copiar el mensaje del error.
