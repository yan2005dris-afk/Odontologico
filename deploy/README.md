# Stack de despliegue — SIGO-Dental

Todos los comandos de esta guía se ejecutan **desde `deploy/`**.

```bash
cd deploy
cp .env.example .env      # y completar los valores (ver más abajo)
docker compose up -d --build
docker compose ps         # 7 servicios en healthy
```

## Servicios

| Servicio | Imagen / build | Puerto publicado | Rol |
| :--- | :--- | :--- | :--- |
| `postgres` | `postgres:16-alpine` | — | Almacenamiento transaccional |
| `redis` | `redis:7-alpine` | — | Sesiones idle (15 min) y colas |
| `rustfs` | `rustfs/rustfs:1.0.4` | — | Almacenamiento de objetos S3 (radiografías, fotos, consentimientos firmados) |
| `clinical-agenda` | `services/clinical-agenda/Dockerfile` | — | Identidad, pacientes, clínico, tratamientos, agenda, auditoría |
| `billing-stock` | `services/billing-stock/Dockerfile` | — | Facturación (POS), liquidaciones, inventario |
| `api-gateway` | `services/api-gateway/Dockerfile` | `8080` (HTTP), `8443` (TLS, sin certificados) | Ingress: JWT/MFA, RBAC, rate limit, OpenAPI |
| `frontend` | `frontend/Dockerfile` | `3000` | Portal Next.js (App Router) |

Redes: `sigo_internal` (interna, sólo entre servicios) y `sigo_edge`
(publica `api-gateway` y `frontend`).

## Variables de entorno obligatorias

`deploy/.env.example` es la plantilla. Las variables de RustFS reemplazan las
de MinIO:

```bash
# RustFS / S3 Object Storage (Radiographs, Clinical Photos, Signed Consents)
RUSTFS_ACCESS_KEY=sigo_rustfs_admin
RUSTFS_SECRET_KEY=sigo_rustfs_secret_change_me
RUSTFS_VOLUMES=/data
S3_BUCKET=sigo-clinical-blobs
S3_ENDPOINT=http://rustfs:9000
```

Y el bloque del portal:

```bash
# Frontend Portal (Next.js App Router PWA)
FRONTEND_PORT=3000
NEXT_PUBLIC_API_GATEWAY_URL=http://localhost:8080
GATEWAY_INTERNAL_URL=http://api-gateway:8080
SESSION_COOKIE_SECURE=false
```

`GATEWAY_INTERNAL_URL` es la URL que usan los *server components* y las
*server actions* del portal para hablar con el gateway por la red interna. El
navegador nunca lo ve: el portal es same-origin, así que no hay CORS y el
gateway no queda expuesto al cliente.

`SESSION_COOKIE_SECURE` debe ser `true` cuando el portal se publique detrás
de TLS. Con el stack local (HTTP plano) tiene que ser `false`, porque un
cookie `Secure` no se guarda en `http://localhost:3000`.

## RustFS: credenciales canónicas

RustFS acepta `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`. `MINIO_ROOT_USER` /
`MINIO_ROOT_PASSWORD` se mapean a las variables **deprecadas**
`RUSTFS_ROOT_USER` / `RUSTFS_ROOT_PASSWORD`: usá las canónicas.

## Portal: credenciales de desarrollo

El login es de **dos pasos** (`RNF-05`): credenciales → desafío MFA → sesión.

1. `POST /api/v1/auth/login` con `{email, password}` devuelve
   `{mfa_required, challenge_token, expires_in_seconds}`.
2. `POST /api/v1/auth/mfa/verify` con `{challenge_token, totp_code}` (6 dígitos)
   devuelve `{access_token, session_id, user, idle_timeout_seconds}`.

El portal en `http://localhost:3000/login` pide primero el correo y la
contraseña, y después el código TOTP de 6 dígitos.

El usuario que siembra `clinical-agenda` al arrancar
(`services/clinical-agenda/cmd/api/main.go:59-70`) es:

| Campo | Valor |
| :--- | :--- |
| Correo | `dra.gomez@sigo-dental.com` |
| Contraseña | `ClinicalPass#2026` |
| Rol RBAC | `dentist` |
| Secreto TOTP (base32) | `JBSWY3DPEHPK3PXP` |

El secreto es un TOTP RFC 6238 estándar, así que se puede pegar en cualquier
app de autenticador (Aegis, Google Authenticator, 1Password) para obtener el
código de 6 dígitos. Para generarlo al vuelo desde la terminal:

```bash
python3 -c "
import base64,hmac,hashlib,struct,time
k=base64.b32decode('JBSWY3DPEHPK3PXP')
h=hmac.new(k,struct.pack('>Q',int(time.time())//30),hashlib.sha1).digest()
o=h[-1]&15
print(str((struct.unpack('>I',h[o:o+4])[0]&0x7fffffff)%1000000).zfill(6))"
```

**Advertencias operativas:**

- **El repositorio de usuarios es en memoria.** Cada reinicio del contenedor
  vuelve a sembrar el usuario y **borra pacientes, turnos, facturas y
  liquidaciones**. Los datos no sobreviven a un `docker compose restart`.
- **Sólo existe un rol sembrado (`dentist`).** Las pantallas de
  administración (`/users`), auditoría (`/audit`) y facturación (`/billing`)
  requieren `admin`, `auditor` y `receptionist` respectivamente; con el
  usuario único no se pueden abrir. `wi-15` siembra los 5 roles canónicos.
- El wire format es **`snake_case`**: `challenge_token`, `totp_code`,
  `access_token`, `idle_timeout_seconds`.

## Migración de blobs MinIO → RustFS

El formato on-disk de RustFS **no** es compatible con MinIO. Renombrar el
volumen no sirve: hay que copiar los objetos.

1. Con el stack viejo todavía arriba, exportá los objetos con `mc`:

   ```bash
   docker run --rm --network host \
     -e MC_HOST_minio=http://<usuario-minio>:<clave-minio>@127.0.0.1:9000 \
     minio/mc sh -c '
       mc alias set rustfs http://127.0.0.1:9000 "$RUSTFS_ACCESS_KEY" "$RUSTFS_SECRET_KEY" &&
       mc mb --ignore-existing rustfs/sigo-clinical-blobs &&
       mc mirror --overwrite minio/sigo-clinical-blobs rustfs/sigo-clinical-blobs
     '
   ```

2. Verificá el objeto count antes y después:

   ```bash
   mc ls --recursive rustfs/sigo-clinical-blobs | wc -l
   ```

3. Recién ahí levantá el stack nuevo (`docker compose up -d --build`): el
   volumen `rustfs_clinical_blobs` queda con los objetos copiados.
