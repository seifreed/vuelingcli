<h1 align="center">vuelingcli</h1>

<p align="center">
  <strong>CLI no oficial de Vueling, pensada también para agentes — vuelos, tarifas y estado de vuelos desde la terminal, sin abrir el navegador</strong>
</p>

<p align="center">
  <a href="https://github.com/seifreed/vuelingcli/releases"><img src="https://img.shields.io/github/v/tag/seifreed/vuelingcli?style=flat-square&logo=go&logoColor=white&label=version" alt="Versión"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-1.26%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Versión de Go"></a>
  <a href="https://github.com/seifreed/vuelingcli/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="Licencia"></a>
  <a href="https://github.com/seifreed/vuelingcli/actions/workflows/check.yml"><img src="https://img.shields.io/github/actions/workflow/status/seifreed/vuelingcli/check.yml?style=flat-square&logo=github&label=check" alt="Check"></a>
</p>

---

## Resumen

**vuelingcli** consulta vuelos y precios de Vueling desde la línea de comandos. `flights`,
`calendar --prices` y `status` consultan por HTTPS las mismas APIs JSON que usa la web de Vueling,
sin abrir pestañas ni leer cookies; el resto de comandos generan enlaces oficiales. Todos los
comandos con datos admiten `--json` o `--toon` para uso programático.

Es de **solo lectura**: no reserva, no paga, no selecciona tarifas ni modifica viajes.

```console
$ vueling flights SVQ AMS --date 12/11/2026 --cheapest --limit 2
https://tickets.vueling.com/booking/flightSearch?...
OUTBOUND 2026-11-12 VY1393 21:10–12:40 273.62 EUR | 1 escala vía BCN · VY1393+VY1050 | 21:10 SVQ → 12:40 AMS VY1393+VY1050 Conexión en BCN
OUTBOUND 2026-11-12 VY1383 06:45–12:40 281.62 EUR | 1 escala vía BCN · VY1383+VY1050 | 06:45 SVQ → 12:40 AMS VY1383+VY1050 Conexión en BCN

$ vueling calendar BCN MLA --date 12/11/2026 --return 03/12/2026 --prices --json
{"date":"12/11/2026","fare":"21 €","returnDate":"03/12/2026","returnFare":"28 €",...}
```

> La salida de la CLI está en inglés; el texto de los itinerarios es el que muestra Vueling.

### Características principales

| Característica | Descripción |
|---------|-------------|
| **Ofertas en vivo** | `flights` lee número de vuelo, horarios, tarifa (Basic y Optima, con maleta facturada) e itinerario, de ida y de vuelta; en los vuelos con conexión indica las escalas, el aeropuerto de conexión y los números de vuelo de cada tramo |
| **Filtros y orden** | `--cheapest`, `--direct`, `--after`, `--before` y `--limit`, aplicados a cada sentido por separado (`--return-after`/`--return-before` acotan solo la vuelta); `--nearby` añade el día más barato del mes según el calendario |
| **Precio con packs** | `select BCN BIO --date +9 --flight VY1460` (o `--cheapest`, y `--return … --return-cheapest` para ida y vuelta) hace lo que la web al pulsar un vuelo: desglose de tarifa, tasas y tasa de bebé, y el total con cada pack (Fly Light, Fly, Fly Pro, Fly Grande), sin reservar nada; termina con el enlace oficial para confirmar tú en vueling.com |
| **Sesión reutilizada** | las búsquedas reutilizan la sesión anónima entre ejecuciones y la renuevan como la web; `session`, `session refresh` y `session clear` la muestran, la extienden o la cierran |
| **Extras exactos** | `extras BCN BIO --date +9 --flight VY1460` da el precio exacto de cada tipo de asiento, maletas por peso, equipaje especial, mascota y Flex Pack de ese vuelo; para verlos mete el vuelo en el carrito de una sesión anónima propia (sin pasajeros ni pago) y la cierra al terminar |
| **Viaje flexible** | `trips BCN BIO --nights 3-5 --from … --to …` cruza el calendario de ida y vuelta y ordena las combinaciones de días por precio total |
| **Días de operación** | `schedule BCN ESU --months 6` muestra qué días vuela una ruta (días del mes, días de la semana, primer y último vuelo); `--days` añade vuelo, hora y precio |
| **Aviso de bajada** | `calendar … --prices --watch 60 --below 40` vigila una tarifa (mínimo cada 30 min) y termina cuando baja del umbral |
| **Calendario de precios** | `calendar --prices` lee la tarifa del día de ida y de vuelta; `--month` lista el mes entero y marca el día más barato |
| **Ofertas por origen** | `deals BCN` lista la tarifa más barata a cada destino desde un origen (como «Ofertas desde» de la web), con `--to`, `--from`, `--months`, `--max`, `--direct` y `--limit` |
| **Red de rutas** | `routes BIO` lista todos los destinos desde un origen, directos o con conexión, filtrables por `--direct` y `--country`, e indica si admiten descuento de residente o familia numerosa |
| **Enlaces oficiales** | `search`/`link` generan la URL de selección de vuelos, con pasajeros, descuentos, moneda y vuelo concreto |
| **Estado de vuelos** | `status VY1158` o `status BCN PMI` leen el estado público de un vuelo o de todos los de una ruta (en hora, en vuelo, aterrizado...) |
| **Precios de extras** | `fees maleta` muestra los rangos de precio de maletas, asientos, mascotas, cambios y demás servicios, online, por teléfono y en aeropuerto |
| **Avisos de incidencias** | `alerts` muestra el aviso de huelgas, meteorología o incidencias que Vueling publica en su web, si lo hay |
| **Catálogo de aeropuertos** | `airports` busca en una copia local por ciudad o código IATA, sin peticiones de red |
| **Fechas flexibles** | `YYYY-MM-DD`, `DD/MM/YYYY`, `today`/`hoy`, `tomorrow`/`mañana`, `+N` o un día de la semana en inglés o español |
| **Salida para agentes** | `--json` y `--toon`; datos a stdout, errores de uso a stderr con código de salida `2` |

---

## Instalación

### Desde una release

Cada tag semántico (`vMAJOR.MINOR.PATCH`, también con sufijo prerelease) publica binarios para Linux
y macOS (amd64 y arm64) y Windows (amd64), con un fichero `SHA256SUMS`. Descarga el de tu plataforma desde
[Releases](https://github.com/seifreed/vuelingcli/releases), verifícalo y pon `vueling` en tu `PATH`:

```sh
shasum -a 256 -c SHA256SUMS --ignore-missing
tar xzf vueling_v0.1.0_darwin_arm64.tar.gz
```

### Con Go

Requiere Go 1.26 o posterior.

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

# Además, el día más barato del mes de ida y del de vuelta
vueling flights BCN MLA --date 14/01/2027 --return 28/01/2027 --nearby

# Precio de un vuelo con cada pack (maleta, asiento, cambios), sin reservar
vueling select BCN BIO --date 10/11/2026 --flight VY1460

# Ida y vuelta con los vuelos directos más baratos de cada día
vueling select BCN BIO --date 10/11/2026 --cheapest --return 15/11/2026 --return-cheapest --direct

# Mantener viva la sesión anónima
vueling session refresh

# Precio exacto de asientos y maletas de un vuelo
vueling extras BCN BIO --date 10/11/2026 --flight VY1460

# Mejores días para 3-5 noches en noviembre
vueling trips BCN BIO --from 10/11/2026 --to 30/11/2026 --nights 3-5

# ¿Qué días vuela Barcelona–Essaouira?
vueling schedule BCN ESU --months 6

# Avisar cuando la ida baje de 40 €
vueling calendar BCN BIO --date 10/11/2026 --prices --watch 60 --below 40

# Tarifas de ida y vuelta en el calendario de precios
vueling calendar BCN MLA --date 14/01/2027 --return 28/01/2027 --prices --json

# Todas las tarifas del mes, con el día más barato marcado
vueling calendar BCN MLA --date 14/01/2027 --prices --month

# ¿A dónde ir barato desde Barcelona? Directos por menos de 30 €
vueling deals BCN --max 30 --direct

# ¿A qué ciudades de Italia vuela Vueling directo desde Bilbao?
vueling routes BIO --direct --country Italia

# ¿Cuánto cuesta facturar una maleta?
vueling fees maleta

# Enlace oficial de búsqueda para dos adultos, abierto en el navegador
vueling search Barcelona "Palma de Mallorca" --date friday --adults 2 --open

# Estado de un vuelo o de una ruta, y avisos de incidencias
vueling status VY1158 --date today
vueling status BCN PMI
vueling alerts

# Códigos de aeropuerto
vueling airports malta
```

Las rutas aceptan códigos IATA o nombres de ciudad no ambiguos del catálogo local (entre comillas si
tienen varias palabras). `flights` y `search`/`link` admiten `--adults`, `--children`, `--infants`, `--discount` (residente/familia numerosa) y
`--currency` (`EUR` por defecto); `flights`, `calendar --prices` y `status` aceptan además `--browser` (y `--cdp-url` con él). Las tarifas del calendario no
son un precio final de reserva.

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
