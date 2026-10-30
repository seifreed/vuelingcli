# vuelingcli

CLI no oficial para consultar vuelos y tarifas públicas de Vueling desde la terminal, pensado también para agentes.

La herramienta será de solo lectura: permite buscar y consultar ofertas. No reserva, no paga y no modifica viajes.

## Desarrollo

```sh
go run ./cmd/vueling help
go run ./cmd/vueling version
```

## Skill para agentes

`.claude/skills/vueling-flights/` documenta cómo usar la CLI y sus límites para agentes.

Sin afiliación con Vueling ni respaldo por su parte.
