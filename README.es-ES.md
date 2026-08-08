

<div align="center">

# pLLM

### High-Performance LLM Gateway Built in Go

[![CI Status](https://img.shields.io/github/actions/workflow/status/andreimerfu/pllm/ci.yml?branch=main&style=classic&logo=github-actions&label=CI)](https://github.com/andreimerfu/pllm/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/codecov/c/github/andreimerfu/pllm?style=classic&logo=codecov)](https://codecov.io/gh/andreimerfu/pllm)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=classic&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green?style=classic)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=classic&logo=docker)](https://hub.docker.com/r/amerfu/pllm)
[![Helm Chart](https://img.shields.io/badge/Helm-Chart-0F1689?style=classic&logo=helm)](https://github.com/andreimerfu/pllm/tree/main/deploy/helm)
[![OpenAI Compatible](https://img.shields.io/badge/OpenAI-Compatible-412991?style=classic&logo=openai)](https://platform.openai.com)

**Sustituto directo para OpenAI** · **Orquestación inteligente de modelos basada en rutas** · **Fiabilidad de grado empresarial**

[Inicio Rápido](#quick-start) · [Rutas](#routes--intelligent-model-orchestration) · [Documentación](docs/)

</div>

---

## ¿Qué es pLLM?

pLLM es una puerta de enlace (gateway) para LLM que se sitúa entre tu aplicación y los proveedores de LLM. En lugar de codificar un solo proveedor en tu aplicación, diriges las llamadas existentes de tu SDK de OpenAI hacia pLLM y obtienes enrutamiento inteligente, conmutación por error automática, equilibrio de carga, caché y observabilidad, sin cambiar una sola línea de código.

```
Your App (OpenAI SDK)  ──>  pLLM Gateway  ──>  OpenAI / Anthropic / Azure / Bedrock / Vertex AI / Grok / Cohere
```

Construida en Go para alto rendimiento, baja latencia y uso mínimo de recursos. Una sola instancia maneja miles de solicitudes concurrentes con un sobrecosto inferior a un milisegundo.

## ¿Por qué pLLM?

| | |
|:--|:--|
| **Cero cambios de código** | Utiliza el formato estándar de la API de OpenAI. Cambia `base_url` y listo. |
| **Multi-proveedor** | OpenAI, Anthropic, Azure OpenAI, AWS Bedrock, Google Vertex AI, Grok, Cohere: todos detrás de un solo punto final. |
| **Enrutamiento inteligente** | Las rutas son puntos finales virtuales de modelos que seleccionan el mejor modelo real utilizando estrategias configurables. |
| **Conmutación por error automática** | Si un proveedor falla, las solicitudes se reintentan transparentmente en alternativas saludables. Cero solicitudes fallidas. |
| **Alto rendimiento** | La concurrencia nativa de Go maneja miles de solicitudes paralelas en una sola instancia. Sin GIL, sin sobrecarga de intérprete. |
| **Control de costos** | Límites de presupuesto, seguimiento de uso, equilibrio de carga con múltiples claves y caché reducen el gasto en LLM. |
| **Lista para producción** | Autenticación JWT, control de tasa, métricas de Prometheus, traza distribuida, gráfico Helm, escalado automático. |

---

## Rutas — Orquestación Inteligente de Modelos

Las rutas son el concepto central que hace que pLLM sea potente. Una **ruta** es un punto final virtual de modelo que se asigna a múltiples modelos reales y utiliza una **estrategia** para decidir cuál maneja cada solicitud.

### El Problema

Las aplicaciones de LLM enfrentan compensaciones difíciles:

- **Dependencia del proveedor (Vendor lock-in)** — Codificar `gpt-4` significa que no puedes cambiar a Claude o Gemini sin cambios en el código.
- **Sin resiliencia** — Si OpenAI tiene una interrupción, tu aplicación se cae con ella.
- **Ineficiencia de costos** — Enviar todo el tráfico a un solo proveedor significa que no puedes optimizar el gasto entre proveedores con precios diferentes.
- **Paredes de límites de tasa** — Una sola clave de API tiene límites fijos de RPM/TPM que limitan tu rendimiento.
- **Sin adaptabilidad** — No puedes enrutar solicitudes sensibles a la latencia de forma diferente a las cargas de trabajo por lotes.

### La Solución: Rutas

Con las rutas, tu aplicación envía solicitudes a un nombre de modelo virtual (por ejemplo, `smart` o `fast`), y pLLM se encarga del resto:

```
App sends: model="smart"
                │
                ▼
        ┌──── Route: "smart" ────┐
        │  Strategy: least-latency│
        │                         │
        │  ┌─────────────────┐   │
        │  │ GPT-4 Turbo     │◄──┼── lowest latency? → selected
        │  │ Claude 3 Opus   │   │
        │  │ Azure GPT-4     │   │
        │  └─────────────────┘   │
        │                         │
        │  Fallbacks:             │
        │  → GPT-3.5 Turbo       │
        │  → Claude 3 Haiku      │
        └─────────────────────────┘
```

Si el modelo seleccionado falla, pLLM intenta automáticamente el siguiente modelo en la ruta. Si todos los modelos de la ruta fallan, pasa a la cadena de respaldo. Tu aplicación nunca verá un error siempre que haya alguna alternativa disponible.

### Estrategias de Enrutamiento

Cada ruta utiliza una de cuatro estrategias para seleccionar un modelo:

| Estrategia | Comportamiento | Ideal para |
|:---------|:---------|:---------|
| **priority** (predeterminado) | Siempre selecciona el modelo con mayor prioridad que esté saludable | Optimización de costos: prefiere modelos baratos, recurre a los costosos |
| **least-latency** | Selecciona el modelo con el menor tiempo de respuesta observado (rastreado mediante Redis en todas las instancias del gateway) | Aplicaciones sensibles a la latencia: siempre enruta al proveedor más rápido |
| **weighted-round-robin** | Distribuye el tráfico proporcionalmente según el peso (WRR suave como nginx) | Distribución de carga: divide el tráfico 70/30 entre proveedores |
| **random** | Selecciona un modelo al azar | Distribución simple, pruebas de caos |

### Configuración de Rutas

Las rutas pueden definirse en `config.yaml` o crearse dinámicamente a través de la API de administración.

**config.yaml:**

```yaml
routes:
  - name: "Smart Models"
    slug: "smart"
    description: "Routes to the best available high-quality model"
    strategy: "least-latency"
    models:
      - model_name: "gpt-4-turbo"
        weight: 50
        priority: 1
      - model_name: "claude-3-opus"
        weight: 50
        priority: 2
      - model_name: "azure-gpt-4"
        weight: 50
        priority: 3
    fallback_models: ["gpt-35-turbo", "claude-3-haiku"]
    enabled: true

  - name: "Fast Models"
    slug: "fast"
    description: "Speed-optimized models for low-latency use cases"
    strategy: "weighted-round-robin"
    models:
      - model_name: "gpt-35-turbo"
        weight: 60
      - model_name: "claude-3-haiku"
        weight: 40
    enabled: true
```

**API de Administración:**

```bash
curl -X POST http://localhost:8080/api/routes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production LLM",
    "slug": "prod-llm",
    "strategy": "weighted-round-robin",
    "models": [
      {"model_name": "gpt-4-turbo", "weight": 70, "priority": 1},
      {"model_name": "claude-3-opus", "weight": 30, "priority": 2}
    ],
    "fallback_models": ["gpt-35-turbo"]
  }'
```

Luego úsalo desde cualquier cliente compatible con OpenAI:

```python
client = OpenAI(base_url="http://localhost:8080/v1", api_key="your-key")
response = client.chat.completions.create(
    model="prod-llm",  # ← this is a route, not a real model
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### Por qué las Rutas Importan para las Aplicaciones LLM

**1. Desacopla tu aplicación de los proveedores.** Tu código hace referencia a `smart` o `fast`, no a `gpt-4` o `claude-3-opus`. Cambia proveedores, añade nuevos o modifica estrategias sin tocar el código de la aplicación.

**2. Elimina puntos únicos de fallo.** Todos los proveedores de LLM sufren interrupciones. Las rutas con cadenas de conmutación por error significan que tu aplicación permanece activa incluso cuando proveedores individuales fallen. El sistema de conmutación por error de tres niveles (reintento de instancia → siguiente modelo en la ruta → modelos de respaldo) hace que el tiempo de inactividad cero sea un objetivo realista.

**3. Optimiza los costos dinámicamente.** Usa una ruta de prioridad que prefiera a tu proveedor más barato y solo escale a modelos costosos cuando el barato no esté disponible o tenga límite de tasa. O usa el round-robin ponderado para distribuir el gasto entre proveedores con diferentes niveles de precios.

**4. Supera los límites de tasa.** Una sola clave de API de OpenAI podría limitarse a 60 RPM. Define una ruta con el mismo modelo en 5 claves de API (o 5 implementaciones de Azure), y pLLM distribuirá el tráfico entre todas ellas, multiplicando efectivamente tu rendimiento.

**5. Adáptate a las condiciones del mundo real.** La estrategia de menor latencia mide continuamente los tiempos de respuesta de los proveedores (distribuidos entre todas las instancias del gateway mediante Redis) y enruta al más rápido. Si un proveedor se degrada, el tráfico se desplaza automáticamente.

**6. Gestiona las rutas en tiempo de ejecución.** Crea, actualiza y monitorea rutas a través de la API de administración y el panel sin reiniciar el gateway. Visualiza estadísticas de distribución de tráfico para entender hacia dónde van las solicitudes y cuánto cuestan.

---

## Características

### Compatibilidad
- **100% Compatible con OpenAI** — Sustituto directo, sin cambios de código
- **Multi-Proveedor** — OpenAI, Anthropic, Azure, Bedrock, Vertex AI, Grok, Cohere
- **Streaming** — Respuestas en streaming en tiempo real para todos los proveedores

### Enrutamiento y Fiabilidad
- **Rutas** — Puntos finales virtuales de modelos con selección basada en estrategias
- **4 Estrategias de Enrutamiento** — Prioridad, menor latencia, round-robin ponderado, aleatorio
- **Conmutación por Error de 3 Niveles** — Reintento de instancia → respaldo de modelo de ruta → cadena de respaldo
- **Seguimiento de Salud** — Puntuaciones de salud en tiempo real con disyuntores (circuit breakers)
- **Equilibrio de Carga Multi-Clave** — Distribuye la carga entre múltiples claves de API

### Seguridad y Control de Acceso
- **Autenticación JWT** — Acceso basado en roles con soporte para Dex OIDC
- **Gestión de Claves API** — Permisos por clave y seguimiento de uso
- **Control de Tasa** — Controles por usuario, por modelo, por punto final
- **Gestión de Presupuesto** — Límites de gasto por usuario y por equipo

### Observabilidad
- **Métricas de Prometheus** — Tasas de solicitud, latencias, uso de tokens, costos
- **Traza Distribuida** — Integración con OpenTelemetry
- **Estadísticas de Rutas** — Distribución de tráfico, desglose de costos por ruta
- **Panel de Administración** — Interfaz web para monitoreo y configuración
- **Swagger UI** — Documentación interactiva de la API

---

## Inicio Rápido

### Docker Compose (Desarrollo)

```bash
# Clone and setup
git clone https://github.com/andreimerfu/pllm.git && cd pllm
cp .env.example .env

# Add your API key
echo "OPENAI_API_KEY=sk-your-key-here" >> .env

# Launch
docker compose up -d

# Test
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "Hello!"}]}'
```

### Kubernetes con Helm (Producción)

```bash
# Add the Helm repository
helm repo add pllm https://andreimerfu.github.io/pllm
helm repo update

# Install
helm install pllm pllm/pllm \
  --set pllm.secrets.jwtSecret="your-jwt-secret" \
  --set pllm.secrets.masterKey="sk-master-your-key" \
  --set pllm.secrets.openaiApiKey="sk-your-openai-key"
```

<details>
<summary><b>Valores de Helm para producción</b></summary>

```yaml
pllm:
  secrets:
    jwtSecret: "your-super-secret-jwt-key-min-32-chars"
    masterKey: "sk-master-production-key"
    openaiApiKey: "sk-your-openai-key"
    anthropicApiKey: "sk-ant-your-anthropic-key"

replicaCount: 3
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: pllm.yourdomain.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: pllm-tls
      hosts:
        - pllm.yourdomain.com

serviceMonitor:
  enabled: true

postgresql:
  enabled: true
  auth:
    database: pllm
    username: pllm
    password: "your-secure-db-password"

redis:
  enabled: true
  auth:
    enabled: true
    password: "your-secure-redis-password"
```

</details>

<details>
<summary><b>Dependencias externas (servicios gestionados en la nube)</b></summary>

```yaml
postgresql:
  enabled: false
redis:
  enabled: false

pllm:
  config:
    database:
      host: "your-rds-instance.amazonaws.com"
      port: 5432
      name: pllm
      sslMode: require
    redis:
      host: "your-redis-cluster.cache.amazonaws.com"
      port: 6379
      tls: true

  secrets:
    databasePassword: "your-db-password"
    redisPassword: "your-redis-password"
    jwtSecret: "your-jwt-secret"
    masterKey: "sk-master-key"
    openaiApiKey: "sk-openai-key"
```

</details>

### Docker Independiente

```bash
docker run -d \
  --name pllm \
  -p 8080:8080 \
  -e OPENAI_API_KEY=sk-your-key \
  -e JWT_SECRET=your-jwt-secret \
  -e MASTER_KEY=sk-master-key \
  amerfu/pllm:latest
```

### Desarrollo Local

```bash
# Prerequisites: Go 1.23+, Node.js, PostgreSQL, Redis
git clone https://github.com/andreimerfu/pllm.git && cd pllm

# Start dependencies
docker compose up postgres redis -d

# Install and run
go mod download
cd web && npm ci && cd ..
make dev  # Hot reload with air
```

### Puntos Finales

| Endpoint | Descripción |
|:---------|:------------|
| `http://localhost:8080/v1` | API compatible con OpenAI |
| `http://localhost:8080/swagger` | Documentación interactiva de la API |
| `http://localhost:8080/ui` | Panel de administración |
| `http://localhost:8080/docs` | Documentación |
| `http://localhost:8080/metrics` | Métricas de Prometheus |
| `http://localhost:8080/health` | Verificación de estado (Health check) |

---

## Integración

pLLM funciona con cualquier cliente compatible con OpenAI. Cambia `base_url` y listo.

### Python

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-api-key",
    base_url="http://localhost:8080/v1"
)

response = client.chat.completions.create(
    model="smart",  # route name or model name
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### Node.js

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: 'your-api-key',
  baseURL: 'http://localhost:8080/v1'
});

const response = await client.chat.completions.create({
  model: 'smart',
  messages: [{ role: 'user', content: 'Hello!' }]
});
```

### LangChain

```python
from langchain.chat_models import ChatOpenAI

llm = ChatOpenAI(
    openai_api_base="http://localhost:8080/v1",
    openai_api_key="your-api-key",
    model="smart"
)
```

---

## Configuración

### Variables de Entorno

```bash
# .env
OPENAI_API_KEY=sk-your-key-here
ANTHROPIC_API_KEY=your-anthropic-key
AZURE_API_KEY=your-azure-key

# Multi-key load balancing
OPENAI_API_KEY_2=sk-second-key
OPENAI_API_KEY_3=sk-third-key
```

### Configuración de Modelos (config.yaml)

```yaml
model_list:
  - model_name: my-gpt-4
    params:
      model: gpt-4
      api_key: ${OPENAI_API_KEY}

  - model_name: my-claude
    params:
      model: claude-3-opus-20240229
      api_key: ${ANTHROPIC_API_KEY}
```

### Configuración del Enrutador

```yaml
router:
  routing_strategy: "least-latency"
  enable_failover: true
  instance_retry_attempts: 2
  failover_timeout_multiple: 1.5
  circuit_breaker_enabled: true

  model_fallbacks:
    gpt-4: gpt-35-turbo
    claude-3-opus: claude-3-sonnet
```

### Ejemplos de Rutas

<details>
<summary><b>Optimización de costos — preferir baratos, recurre a costosos</b></summary>

```yaml
routes:
  - name: "Cost Optimized"
    slug: "cost-opt"
    strategy: "priority"
    models:
      - model_name: "gpt-35-turbo"
        priority: 1    # try first (cheapest)
      - model_name: "gpt-4-turbo"
        priority: 2    # fall back if needed
      - model_name: "claude-3-opus"
        priority: 3    # last resort
```

</details>

<details>
<summary><b>Redundancia de proveedores — sobrevive a cualquier interrupción de un solo proveedor</b></summary>

```yaml
routes:
  - name: "Reliable"
    slug: "reliable"
    strategy: "priority"
    models:
      - model_name: "openai-gpt4"
        priority: 1
      - model_name: "azure-gpt4"
        priority: 2
      - model_name: "anthropic-claude"
        priority: 3
    fallback_models: ["local-llm"]
```

</details>

<details>
<summary><b>Distribución de carga — divide el tráfico entre proveedores</b></summary>

```yaml
routes:
  - name: "Balanced"
    slug: "balanced"
    strategy: "weighted-round-robin"
    models:
      - model_name: "openai-gpt4"
        weight: 40    # 40% of traffic
      - model_name: "azure-gpt4"
        weight: 35    # 35% of traffic
      - model_name: "anthropic-claude"
        weight: 25    # 25% of traffic
```

</details>

<details>
<summary><b>Multiplicación del límite de tasa — mismo modelo, múltiples claves</b></summary>

```yaml
routes:
  - name: "High Throughput GPT-4"
    slug: "ht-gpt4"
    strategy: "weighted-round-robin"
    models:
      - model_name: "gpt4-key1"
        weight: 50
      - model_name: "gpt4-key2"
        weight: 50
```

Cada `model_name` apunta a una clave de API diferente para el mismo modelo subyacente, duplicando efectivamente tu límite de tasa.

</details>

---

## Arquitectura

```
┌─────────────────────────────────────────────────────────────┐
│                        pLLM Gateway                         │
│                                                             │
│  Request ──> Auth ──> Rate Limit ──> Cache Check            │
│                                         │                   │
│                                    Route Resolution         │
│                                         │                   │
│                              ┌──── Strategy ────┐           │
│                              │  priority         │           │
│                              │  least-latency    │           │
│                              │  weighted-rr      │           │
│                              │  random           │           │
│                              └──────────────────┘           │
│                                         │                   │
│                              Instance Selection             │
│                              Health Check + Failover        │
│                                         │                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐      │
│  │  OpenAI  │ │ Anthropic│ │  Azure   │ │ Bedrock  │ ...  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘      │
│                                                             │
│  PostgreSQL (state) · Redis (cache, latency, locks, queues) │
│  Prometheus (metrics) · OpenTelemetry (tracing)             │
└─────────────────────────────────────────────────────────────┘
```

**Tecnologías:** Go · Router Chi · GORM + PostgreSQL · Redis · Prometheus · Swagger

---

## Escalado para Alto Volumen

El gateway en sí rara vez es el cuello de botella: lo son los proveedores de LLM. Una sola instancia de pLLM maneja miles de solicitudes concurrentes, pero una sola implementación de OpenAI podría limitarse a 60-100 RPM.

Para escalar:

1. **Múltiples implementaciones del mismo modelo** — Crea varias implementaciones de Azure OpenAI o usa múltiples claves de API, y colócalas detrás de una ruta de round-robin ponderado.
2. **Redundancia multi-proveedor** — Usa el mismo modelo desde diferentes proveedores (OpenAI + Azure + Bedrock) para multiplicar el rendimiento y añadir resiliencia.
3. **Distribución geográfica** — Implementa modelos en diferentes regiones y usa el enrutamiento de menor latencia para minimizar los tiempos de respuesta.

```yaml
# Example: 5 GPT-4 deployments behind one route
routes:
  - name: "High-Scale GPT-4"
    slug: "gpt4"
    strategy: "weighted-round-robin"
    models:
      - model_name: "azure-gpt4-east-1"
        weight: 20
      - model_name: "azure-gpt4-east-2"
        weight: 20
      - model_name: "azure-gpt4-west-1"
        weight: 20
      - model_name: "bedrock-gpt4-1"
        weight: 20
      - model_name: "bedrock-gpt4-2"
        weight: 20
    fallback_models: ["gpt-35-turbo"]
```

Tu aplicación sigue enviando `model="gpt4"`. pLLM se encarga del resto.

---

## Gráfico Helm

Disponible en varios registros:

| Registro | Comando |
|:---------|:--------|
| GitHub Pages | `helm repo add pllm https://andreimerfu.github.io/pllm` |
| Docker Hub (OCI) | `helm install pllm oci://registry-1.docker.io/amerfu/pllm` |
| ArtifactHub | [Ver en ArtifactHub](https://artifacthub.io/packages/helm/pllm/pllm) |

```bash
# List versions
helm search repo pllm/pllm --versions

# Upgrade
helm upgrade pllm pllm/pllm -f your-values.yaml

# Rollback
helm rollback pllm 1
```

---

## Monitoreo

Métricas de Prometheus en `/metrics`, con ServiceMonitor opcional para autodescubrimiento en Kubernetes.

| Endpoint | Descripción |
|:---------|:------------|
| `/health` | Verificación básica de estado |
| `/ready` | Listado completo (todas las dependencias) |
| `/metrics` | Exportación de métricas de Prometheus |
| `/api/routes/{id}/stats` | Estadísticas de tráfico y costo por ruta |

---

## Contribuciones

- [Issues de GitHub](https://github.com/andreimerfu/pllm/issues) — Informes de errores y solicitudes de funciones
- [Documentación](docs/) — Guías y referencias
- Se aceptan pull requests

## Licencia

[Licencia MIT](LICENSE)

---

<div align="center">

**[Star en GitHub](https://github.com/andreimerfu/pllm)**

</div>
