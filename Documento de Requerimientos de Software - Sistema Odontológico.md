# DOCUMENTO DE ESPECIFICACIÓN DE REQUERIMIENTOS DE SOFTWARE (SRS / BRD)

**Proyecto:** Sistema Integral de Gestión Odontológica y Expediente Clínico Digital (SIGO-Dental)  
**Destinatario:** Departamento de Tecnología de la Información (TI) / Equipo de Arquitectura y Desarrollo  
**Versión:** 1.0  
**Fecha:** 29 de octubre de 2026  
**Estado:** Para Revisión Técnica y Estimación  

---

## 1. CONTROL DE VERSIONES Y AUTORIZACIONES

### 1.1 Historial de Cambios

| Versión | Fecha | Autor / Rol | Descripción del Cambio |
| :--- | :--- | :--- | :--- |
| 1.0 | 29/10/2026 | Líder Funcional / Clínica Odontológica | Levantamiento y consolidación inicial de requerimientos funcionales y técnicos. |

### 1.2 Firmas de Aprobación

| Rol | Nombre | Firma | Fecha |
| :--- | :--- | :--- | :--- |
| Sponsor del Negocio (Director Médico) | | | |
| Product Owner / Analista Funcional | | | |
| Arquitecto de Soluciones / TI Lead | | | |

---

## 2. INTRODUCCIÓN Y ALCANCE

### 2.1 Propósito
El presente documento define formalmente los requerimientos funcionales, no funcionales, reglas de negocio e interfaces de integración para el diseño, desarrollo, pruebas y puesta en producción del software clínico-administrativo para clínicas odontológicas.

### 2.2 Objetivos del Negocio
* Digitalizar el 100% de los expedientes clínicos, odontogramas y recetas odontológicas.
* Reducir el ausentismo de pacientes mediante recordatorios automáticos multicanal (WhatsApp/SMS/Email).
* Centralizar el control de inventario de insumos clínicos por consultorio y profesional.
* Garantizar el cumplimiento normativo en materia de protección de datos personales de salud (Ley de Protección de Datos Personales / HIPAA / regulaciones sanitarias locales).

### 2.3 Alcance del Sistema
* **Dentro del alcance:** Portal web y responsive para personal clínico y administrativo, odontograma interactivo 2D (con soporte para notación FDI y Universal), agenda de citas por sillón/odontólogo, emisión de planes de tratamiento y presupuestos, facturación electrónica y módulo de telemetría/auditoría.
* **Fuera del alcance (Fase 1):** App móvil nativa (iOS/Android) para pacientes (se abordará mediante portal PWA en Fase 2); procesamiento directo de imágenes 3D DICOM (se utilizará visualización y almacenamiento de archivos planos/PACS externo en esta etapa).

---

## 3. PERFILES DE USUARIO Y MATRIZ RACI / ROLES

| Rol | Descripción | Permisos Clave |
| :--- | :--- | :--- |
| **Administrador TI / Sistema** | Gestión de infraestructura, usuarios y auditoría. | Configuración global, logs, parametrización de catálogos y backups. |
| **Recepcionista / Cajero** | Operación de recepción y pagos. | Agendamiento, registro de pacientes, cobro y facturación. |
| **Odontólogo General / Especialista** | Atención clínica al paciente. | Registro clínico, odontograma, plan de tratamiento, recetas, evolución. |
| **Asistente Dental** | Soporte en consultorio. | Registro de signos vitales, preparación de insumos, toma de notas clínicas. |
| **Auditor / Contador** | Conciliación financiera e inventarios. | Reportes de corte de caja, liquidación de honorarios, inventario. |

---

## 4. REQUERIMIENTOS FUNCIONALES (RF)

### 4.1 Módulo: Gestión de Pacientes y Filiación
* **RF-01 (Registro Unificado):** El sistema debe permitir el alta, consulta y modificación de la ficha única del paciente (datos demográficos, contacto de emergencia, tutor legal si es menor de edad, seguro dental/convenio).
* **RF-02 (Anamnesis y Alertas Médicas):** Registro obligatorio de historial médico general (enfermedades sistémicas, alergias a fármacos, antecedentes quirúrgicos, coagulopatías). Si existe una condición crítica (ej. alergia a penicilina, anticoagulados), el sistema debe desplegar un *badge* de advertencia de alta visibilidad en la cabecera del expediente.

### 4.2 Módulo: Expediente Clínico y Odontograma
* **RF-03 (Odontograma Gráfico Interactivo):**
  * Representación anatómica visual de dentición permanente (32 dientes) y temporal/decidua (20 dientes).
  * Soporte de nomenclatura internacional FDI y nomenclatura Universal americana.
  * Marcado por superficies dentales (mesial, distal, oclusal/incisal, vestibular, lingual/palatino).
  * Catálogo visual estandarizado: caries (rojo), restauraciones existentes (azul), endodoncia realizada/requerida, implantes, prótesis fijas/removibles, exodoncias y sellantes.
* **RF-04 (Periodontograma Digital):** Módulo para captura de profundidad al sondaje (6 puntos por diente), margen gingival, cálculo de nivel de inserción clínica (NIC), furca, movilidad y sangrado al sondaje.
* **RF-05 (Evolución y Notas SOEP):** Bitácora cronológica inmutable de notas de evolución (Subjetivo, Objetivo, Evaluación, Plan). Cada nota debe quedar bloqueada contra edición trascurrido un plazo configurable (por defecto 24 horas) y requerir firma digital del profesional.
* **RF-06 (Consentimiento Informado Digital):** Generación de formatos legales de consentimiento por procedimiento, con captura de firma biométrica en tablet/pantalla táctil o validación mediante código OTP enviado al paciente.

### 4.3 Módulo: Planes de Tratamiento y Presupuestos
* **RF-07 (Generación de Presupuestos Multinivel):** Posibilidad de armar tratamientos por fases/sesiones, calculando honorarios, costo de laboratorio y descuentos autorizados.
* **RF-08 (Aprobación y Transición a Citas):** El sistema debe permitir registrar la aprobación total o parcial del presupuesto por parte del paciente, convirtiendo los ítems autorizados en tareas agendables.

### 4.4 Módulo: Agenda Médica y Turnos
* **RF-09 (Agendamiento Multirrecinto):** Calendario con vista por día/semana filtrable por consultorio/sillón dental y por profesional especialista.
* **RF-10 (Control de Estados de Cita):** Estados del turno: Programado, Confirmado, En Sala de Espera, En Atención, Atendido, Cancelado, No Asistió (No-Show).
* **RF-11 (Notificaciones Automáticas):** Envío de mensajes transaccionales de confirmación y recordatorio 48h y 24h antes del turno vía API (WhatsApp Business / SMS / Email).

### 4.5 Módulo: Facturación, Caja e Inventario
* **RF-12 (Punto de Venta y Facturación Electrónica):** Emisión de comprobantes fiscales electrónicos autorizados por la autoridad tributaria local, admitiendo pagos mixtos (efectivo, tarjeta crédito/débito, transferencias, seguros).
* **RF-13 (Liquidación de Honorarios a Profesionales):** Cálculo configurable de comisiones a odontólogos (porcentaje fijo, monto fijo por procedimiento o porcentaje tras deducir costos de laboratorio/materiales).
* **RF-14 (Descargo Automático de Insumos Clínicos):** Deducción automática de stock de insumos y materiales (resinas, anestesias, guantes) al completar un procedimiento clínico según la receta estándar de consumo configurada.

---

## 5. REQUERIMIENTOS NO FUNCIONALES (RNF)

### 5.1 Rendimiento y Disponibilidad
* **RNF-01 (Tiempo de Respuesta):** El tiempo de carga del expediente clínico y renderizado del odontograma debe ser inferior a $1.5\text{ segundos}$ bajo condiciones de red estándar (conexión $\ge 15\text{ Mbps}$).
* **RNF-02 (Disponibilidad / SLA):** El sistema debe garantizar un uptime mínimo de $99.9\%$ en horario operativo clínico (07:00 a 21:00 hora local).
* **RNF-03 (Concurrencia):** Capacidad base para soportar 150 usuarios concurrentes sin degradación de latencia en la primera fase.

### 5.2 Seguridad y Protección de Datos
* **RNF-04 (Cifrado de Datos):**
  * Cifrado en tránsito mediante TLS 1.3 con certificados SSL vigentes.
  * Cifrado en reposo para bases de datos y blobs de almacenamiento utilizando algoritmos robustos (AES-256).
* **RNF-05 (Autenticación y Autorización):**
  * Control de acceso basado en roles (RBAC) y principio de menor privilegio.
  * Autenticación Multifactor (MFA) obligatoria para acceso a expedientes clínicos e información financiera.
  * Cierre automático de sesión tras 15 minutos de inactividad en estaciones de trabajo clínicas.
* **RNF-06 (Trazabilidad y Auditoría):** Log inmutable de auditoría para cada acción CRUD sobre historias clínicas (quién consultó, modificó o exportó qué registro, con timestamp ISO-8601 y dirección IP de origen).

### 5.3 Usabilidad y Compatibilidad
* **RNF-07 (Diseño Responsive):** Interfaz optimizada para pantallas de escritorio ($1920 \times 1080$), laptops ($1366 \times 768$) y tablets (iPad / Android $\ge 10.5''$) para uso directo en sillón odontológico.
* **RNF-08 (Soporte de Navegadores):** Compatibilidad con las últimas dos versiones estables de Google Chrome, Mozilla Firefox, Microsoft Edge y Safari.

---

## 6. REGLAS DE NEGOCIO (RN)

* **RN-01 (Inmutabilidad del Historial Clínico):** Ningún procedimiento registrado como "Realizado" puede ser borrado de la base de datos. Cualquier corrección debe generarse mediante una nota de adenda firmada por el profesional responsable.
* **RN-02 (Cierre de Caja Ciego):** Los recepcionistas no podrán ver el total teórico del sistema antes de ingresar el conteo físico de dinero en efectivo durante el cierre diario de caja.
* **RN-03 (Validación de Sobreposición de Sillones):** El sistema no debe permitir agendar dos citas simultáneas en el mismo sillón dental físico, independientemente de si los odontólogos son diferentes.

---

## 7. ARQUITECTURA SUGERIDA E INTEGRACIONES TÉCNICAS

```text
+-------------------------------------------------------------+
|                        CAPA CLIENTE                         |
|     Web App / PWA (React / Angular / Vue + Tailwind CSS)    |
+------------------------------+------------------------------+
                               | HTTPS / WSS / JWT
+------------------------------v------------------------------+
|                    API GATEWAY / INGRESS                    |
+------------------------------+------------------------------+
                               |
        +----------------------+----------------------+
        |                                             |
+-------v---------------------+             +---------v-------------------+
|  SERVICIO CLÍNICO & AGENDA  |             | SERVICIO FACTURACIÓN/STOCK  |
|  (Microservicio / Modular)  |             |  (Microservicio / Modular)  |
+-------+---------------------+             +---------+-------------------+
        |                                             |
+-------v---------------------------------------------v-------------------+
|                         PERSISTENCIA Y STORAGE                          |
| - Base de Datos Transaccional (PostgreSQL / MySQL)                      |
| - Cache & Mensajería (Redis)                                            |
| - Almacenamiento de Archivos (S3 / Azure Blob: Rx, Fotos, PDFs)         |
+-------------------------------------------------------------------------+
```

### 7.1 Puntos de Integración Requeridos (APIs Externas)
* **API de Facturación Fiscal / Electrónica:** Integración REST para el timbrado/emisión de facturas y retenciones según la legislación nacional.
* **Pasarela de Pagos:** Integración con proveedores locales de cobro digital (Stripe, dLocal o pasarelas bancarias autorizadas).
* **Gateway de Mensajería:** Conexión con Meta Cloud API (WhatsApp Business) y Twilio para SMS/Email transaccional.
* **PACS / Visor DICOM (Futuro):** Protocolos DICOM/HL7 para recepción directa de imágenes radiológicas panorámicas y periapicales.

---

## 8. ENTREGABLES, CRITERIOS DE ACEPTACIÓN Y FASES

### 8.1 Fases Propuestas para el Equipo de TI
1. **Fase 1 - Core Administrativo y Turnos (Sprints 1-4):** Pacientes, agendamiento de citas, catálogo médico y facturación básica.
2. **Fase 2 - Core Clínico (Sprints 5-8):** Odontograma interactivo, periodontograma, recetas, consentimientos informados y firma digital.
3. **Fase 3 - Finanzas Avanzadas, Inventarios y Reportes (Sprints 9-11):** Descargo de insumos, comisiones médicas y reportes BI.
4. **Fase 4 - Pruebas UAT, Auditoría de Seguridad y Salida a Producción (Sprint 12).**

### 8.2 Criterios Generales de Aceptación Técnica
* Cobertura mínima de pruebas unitarias y de integración del $80\%$ en endpoints críticos del backend.
* Escaneo de seguridad DAST/SAST sin vulnerabilidades de severidad Alta o Crítica (OWASP Top 10).
* Entrega de documentación técnica de APIs (especificación OpenAPI / Swagger v3.0).
