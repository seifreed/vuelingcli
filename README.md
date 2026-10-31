<h1 align="center">vuelingcli</h1>

<p align="center">
  <strong>CLI no oficial de Vueling, pensada también para agentes — vuelos y tarifas desde la terminal usando tu Chrome local</strong>
</p>

<p align="center">
  <a href="https://github.com/seifreed/vuelingcli/releases"><img src="https://img.shields.io/github/v/tag/seifreed/vuelingcli?style=flat-square&logo=go&logoColor=white&label=version" alt="Versión"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-1.24%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Versión de Go"></a>
  <a href="https://github.com/seifreed/vuelingcli/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="Licencia"></a>
  <a href="https://github.com/seifreed/vuelingcli/actions/workflows/check.yml"><img src="https://img.shields.io/github/actions/workflow/status/seifreed/vuelingcli/check.yml?style=flat-square&logo=github&label=check" alt="Check"></a>
</p>

---

## Resumen

**vuelingcli** consulta vuelos y precios de Vueling desde la línea de comandos. `flights` lee las
ofertas renderizadas en la web de Vueling a través de tu sesión local de Chrome; `calendar --prices`
lee las tarifas del calendario de precios; el resto de comandos generan enlaces oficiales. Todos los
comandos con datos admiten `--json` o `--toon` para uso programático.

Es de **solo lectura**: no reserva, no paga, no selecciona tarifas ni modifica viajes.

```console
$ vueling flights SVQ AMS --date 12/11/2026 --cheapest --limit 2
https://tickets.vueling.com/booking/flightSearch?...
OUTBOUND 2026-11-12 VY1393 21:10–12:40 274 EUR | 21:10 SVQ Sevilla Conexión en BCN: 11 h 15 min Ámsterdam AMS 12:40 +1 día ...
OUTBOUND 2026-11-12 VY1383 06:45–12:40 282 EUR | 06:45 SVQ Sevilla Conexión en BCN: 1 h 40 min Ámsterdam AMS 12:40 ...

$ vueling calendar BCN MLA --date 12/11/2026 --return 03/12/2026 --prices --json
{"date":"12/11/2026","fare":"21 €","returnDate":"03/12/2026","returnFare":"28 €",...}
```

> La salida de la CLI está en inglés; el texto de los itinerarios es el que muestra Vueling.

### Características principales

| Característica | Descripción |
|---------|-------------|
| **Ofertas en vivo** | `flights` lee número de vuelo, horarios, tarifa e itinerario visibles, de ida y de vuelta |
| **Filtros y orden** | `--cheapest`, `--direct`, `--after`, `--before` y `--limit`, aplicados a cada sentido por separado |
| **Calendario de precios** | `calendar --prices` lee la tarifa del día de ida y de vuelta; `--month` lista el mes entero y marca el día más barato |
| **Enlaces oficiales** | `search`/`link` generan la URL de selección de vuelos, con pasajeros, descuentos, moneda y vuelo concreto |
| **Estado de vuelos** | `status VY1158` lee el estado público del vuelo (en hora, retrasado...) con salida y llegada |
| **Catálogo de aeropuertos** | `airports` busca en una copia local por ciudad o código IATA, sin peticiones de red |
| **Fechas flexibles** | `YYYY-MM-DD`, `DD/MM/YYYY`, `today`, `tomorrow`, `+N` o un día de la semana en inglés o español |
| **Salida para agentes** | `--json` y `--toon`; datos a stdout, errores de uso a stderr con código de salida `2` |

---

## Instalación

### Desde una release

Cada tag `v*` publica binarios para Linux y macOS (amd64 y arm64) y Windows (amd64), con un fichero
`SHA256SUMS`. Descarga el de tu plataforma desde
[Releases](https://github.com/seifreed/vuelingcli/releases), verifícalo y pon `vueling` en tu `PATH`:

```sh
shasum -a 256 -c SHA256SUMS --ignore-missing
tar xzf vueling_v0.1.0_darwin_arm64.tar.gz
```

### Con Go

Requiere Go 1.24 o posterior.

```sh
go install github.com/seifreed/vuelingcli/cmd/vueling@latest
vueling help
```

---

## Inicio rápido

```sh
# Ofertas de ida y vuelta, en JSON
vueling flights BCN MLA --date 14/01/2027 --return 28/01/2027 --json

# Solo directos, entre las 08:00 y las 20:00, los 3 más baratos
vueling flights BCN MLA --date 14/01/2027 --direct --after 08:00 --before 20:00 --cheapest --limit 3

# Tarifas de ida y vuelta en el calendario de precios
vueling calendar BCN MLA --date 14/01/2027 --return 28/01/2027 --prices --json

# Todas las tarifas del mes, con el día más barato marcado
vueling calendar BCN MLA --date 14/01/2027 --prices --month

# Enlace oficial de búsqueda para dos adultos, abierto en el navegador
vueling search Barcelona "Palma de Mallorca" --date friday --adults 2 --open

# Códigos de aeropuerto
vueling status VY1158 --date today
vueling airports malta
```

Las rutas aceptan códigos IATA o nombres de ciudad no ambiguos del catálogo local (entre comillas si
tienen varias palabras). `flights` y `search`/`link` admiten `--adults`, `--children`, `--infants` y
`--currency` (`EUR` por defecto); `flights` acepta además `--cdp-url`. Las tarifas del calendario no
son un precio final de reserva.

---

## Requisitos del navegador

`flights` y `calendar --prices` necesitan Chrome; los demás comandos no abren ni controlan el
navegador.

En macOS, `flights` usa Apple Events para abrir una pestaña, enviar el formulario de búsqueda y leer
ofertas desde el perfil actual de Chrome; hay que habilitar **View > Developer > Allow JavaScript
from Apple Events**. Esa opción permite que AppleScript ejecute JavaScript en páginas, así que no la
actives sin autorización.

En otros sistemas, `flights` requiere un endpoint DevTools local ligado a `localhost`/loopback, que
permite controlar el navegador. Usa `--cdp-url` para especificar un endpoint local; solo se aceptan
`localhost` o una IP de loopback. Chrome 136+ ignora el puerto de depuración por línea de comandos
con el perfil de datos predeterminado; no copies ese perfil como workaround. Consulta los
[cambios de depuración remota de Chrome](https://developer.chrome.com/blog/remote-debugging-port).

El comando extrae solo el texto visible de las ofertas, no lee cookies, tokens ni APIs privadas y no
selecciona tarifas. `calendar --prices` también lee el precio visible y requiere el mismo puente de
navegador.

Un CAPTCHA, una sesión caducada o una caída del servicio de Vueling pueden impedir que carguen los
resultados. Si `tickets.vueling.com` se queda en "Cargando..." o `flights` no encuentra resultados
renderizados, puede hacer falta iniciar sesión en Vueling en ese perfil de Chrome antes de reintentar.

---

## Skill para agentes y referencia de la CLI

`.claude/skills/vueling-flights/` incluye una skill que explica a los agentes cómo buscar en el
navegador y usar la CLI. La [referencia de CLI](.claude/skills/vueling-flights/references/cli-reference.md)
documenta cada opción, formato de fecha y esquema de salida `--json`/`--toon`. La skill está en
inglés.

---

## Licencia

Este proyecto se publica bajo licencia MIT. Consulta [LICENSE](LICENSE).

Sin afiliación con Vueling ni respaldo por su parte.
