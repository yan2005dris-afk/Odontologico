# SIGO-Dental

**Sistema Integral de Gestión Odontológica y Expediente Clínico Digital**

Plataforma clínica y administrativa para clínicas dentales: historia clínica
digital, odontograma 2D, periodontograma, agenda multi-sillón, facturación POS,
liquidaciones e inventario.

El detalle funcional vive en el
[Documento de Requerimientos de Software](./Documento%20de%20Requerimientos%20de%20Software%20-%20Sistema%20Odontol%C3%B3gico.md)
y el estado de avance en [`pdd/board.md`](./pdd/board.md).

## Stack

| Capa | Tecnología |
| :--- | :--- |
| Portal web | Next.js (App Router), TypeScript, Tailwind CSS |
| API Gateway / Ingress | Go 1.24 — TLS 1.3, JWT + MFA, RBAC, rate limit, OpenAPI |
| Servicio clínico y agenda | Go 1.24 — identidad, pacientes, clínico, tratamientos, agenda, auditoría |
| Servicio facturación y stock | Go 1.24 — POS, liquidaciones, inventario |
| Datos | PostgreSQL 16, Redis 7, RustFS (almacenamiento de objetos S3) |
| Despliegue | Docker Compose — 7 servicios + túnel de Cloudflare opcional |

## Requisitos

- **Docker Engine 24+** con **Compose v2.20+** (`docker compose version`).
- ~3 GB de RAM libres para los 7 servicios.
- Puertos del host libres: **18777** (portal), **18080** (gateway HTTP),
  **18443** (gateway TLS).

> Los puertos del host **no son los habituales** (3000 / 8080 / 8443) a
> propósito: el stack está pensado para quedar prendido junto a otros
> proyectos locales sin pelearse por los puertos. Los puertos internos de los
> contenedores (`3000`, `8080`, `8443`) no cambian nunca; sólo cambia el lado
> del host, y se ajustan con `FRONTEND_PORT`, `GATEWAY_HTTP_PORT` y
> `GATEWAY_HTTPS_PORT`.

---

## Despliegue local con Docker

Todos los comandos se ejecutan **desde `deploy/`**.

### 1. Preparar la configuración

```bash
cd deploy
cp .env.example .env
```

`deploy/.env.example` es la plantilla y **ya funciona tal cual** para
desarrollo local: los valores por defecto levantan el stack con datos de
demostración y sin MFA. Revisá [`deploy/.env.example`](./deploy/.env.example)
antes de tocarlo: todo lo que pongas ahí **sobrescribe** el valor por defecto
de `compose.yaml`.

### 2. Levantar el stack

```bash
docker compose up -d --build
```

La primera vez compila las tres imágenes Go y la del frontend (Next.js), así
que tarda varios minutos. Las siguientes reutilizan la caché.

### 3. Verificar

```bash
docker compose ps          # los 7 servicios deben estar "healthy"
curl -s http://localhost:18080/healthz
curl -sI http://localhost:18777/ | head -1
```

### 4. Entrar al portal

Abrí **<http://localhost:18777/login>**. El stack local apaga el MFA
(`MFA_REQUIRED=false`) y muestra el panel **«Modo desarrollo»** con un botón
por rol: pulsás uno y entrás sin calcular ningún TOTP.

Las credenciales de los 5 roles canónicos y sus secretos TOTP están en
[`deploy/README.md`](./deploy/README.md).

### 5. Detener, reiniciar o resetear

```bash
docker compose stop                 # para los servicios, conserva los datos
docker compose down                 # borra los contenedores, conserva los volúmenes
docker compose down -v              # borra TODO, incluidos los volúmenes
docker compose up -d --build        # vuelve a levantar
```

> **Ojo:** los repositorios de `clinical-agenda` y `billing-stock` son **en
> memoria**. Cada arranque arranca vacío y vuelve a sembrar el juego de datos
> de demostración. Si necesitás conservar los datos, hay que cambiar la
> persistencia a PostgreSQL.

---

## Publicar con Cloudflare Tunnel

El stack trae un servicio `cloudflared` (perfil `tunnel`) que publica el
portal en un hostname de Cloudflare sin abrir puertos en el router.

### 1. Crear el túnel en el panel de Cloudflare

En **Cloudflare → Zero Trust → Networks → Tunnels → Create a tunnel**, elegí
`cloudflared` y copiá el token que te muestra.

### 2. Pegar el token

En `deploy/.env`:

```bash
CLOUDFLARE_TUNNEL_TOKEN=eyJhIjoi...
```

### 3. Configurar el hostname público

En el mismo panel, en **Public Hostname**, creá el registro apuntando a:

```
http://frontend:3000
```

Ojo con el destino: es el **nombre del servicio de compose**, no
`localhost:3000` — el túnel corre dentro de la red de Docker y no ve el host.
Si tu ingress apunta a `http://gateway:8080`, también funciona: el servicio
`api-gateway` publica un alias de red llamado `gateway` en `sigo_edge`.

No hace falta exponer el gateway: el portal es same-origin y sus server
actions hablan con el gateway por la red interna.

### 4. Levantar

```bash
docker compose --profile tunnel up -d
docker compose logs -f cloudflared
```

El portal queda disponible en el hostname configurado (HTTPS gestionado por
Cloudflare). Para bajarlo: `docker compose --profile tunnel stop cloudflared`.

### Antes de exponerlo a internet

El stack local nace para desarrollo, no para exposición pública. Si lo vas a
dejar prendido y accesible por internet, poné estas cuatro cosas:

| Variable | Valor | Por qué |
| :--- | :--- | :--- |
| `NEXT_PUBLIC_DEV_ROLE_LOGIN` | `false` | El selector de rol **mete las contraseñas en el bundle del navegador**. Se inlinea al compilar: cambiarla exige `docker compose build frontend`. |
| `MFA_REQUIRED` | `true` | Con el MFA apagado, la contraseña sola abre la sesión. |
| `SESSION_COOKIE_SECURE` | `true` | Obligatorio detrás de HTTPS: con `false` el cookie viaja sin `Secure`. |
| `ALLOWED_ORIGIN` | `https://<tu-hostname>` | El origen real del portal, para el CORS del gateway. |

Y cambiá `JWT_SECRET`, `POSTGRES_PASSWORD` y `RUSTFS_SECRET_KEY`: los valores
de la plantilla son públicos y están en este repositorio.

---

## Comandos de desarrollo

```bash
# API Gateway
cd services/api-gateway && go test ./... && go vet ./... && go build -o bin/api-gateway ./cmd/gateway

# Servicio clínico y agenda
cd services/clinical-agenda && go test ./... && go vet ./... && go build -o bin/clinical-agenda ./cmd/api

# Servicio facturación y stock
cd services/billing-stock && go test ./... && go vet ./... && go build -o bin/billing-stock ./cmd/api

# Frontend
cd frontend && pnpm test && pnpm lint && pnpm build
```

## Estructura

```
deploy/          Stack de despliegue (compose.yaml, .env.example, README.md)
services/        api-gateway · clinical-agenda · billing-stock (Go, hexagonal)
frontend/        Portal Next.js (App Router)
openapi/         Contrato OpenAPI v1 publicado por el gateway
pdd/             Contexto del proyecto, board y work items
docs/            Informes y reportes
```

## Documentación

- [`deploy/README.md`](./deploy/README.md) — referencia del stack: variables,
  credenciales de desarrollo, datos sembrados y Tunnel de Cloudflare.
- [`AGENTS.md`](./AGENTS.md) — reglas del repositorio para agentes de código.
- [`REPORTE-ESTADO-APLICACION.md`](./REPORTE-ESTADO-APLICACION.md) — estado
  verificado del código frente al SRS.
