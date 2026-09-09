from datetime import datetime
from typing import List, Optional
from pydantic import BaseModel, Field

class ContainerMetricSchema(BaseModel):
    id: str = Field(..., description="ID corto del contenedor (12 caracteres)")
    name: str = Field(..., description="Nombre del contenedor")
    image: str = Field(..., description="Imagen del contenedor")
    status: str = Field(..., description="Estado del contenedor (running, exited, etc.)")
    cpu_percent: float = Field(default=0.0, description="Porcentaje de uso de CPU")
    ram_bytes: int = Field(default=0, description="RAM real en bytes sin caché inactiva")
    ram_limit_bytes: int = Field(default=0, description="Límite máximo de RAM en bytes")
    egress_bytes_sec: float = Field(default=0.0, description="Tasa de salida de red en bytes/segundo")
    ingress_bytes_sec: float = Field(default=0.0, description="Tasa de entrada de red en bytes/segundo")
    pids: int = Field(default=0, description="Número de procesos activos")
    timestamp: datetime = Field(default_factory=datetime.utcnow, description="Fecha y hora de la métrica")

class TelemetryBatchSchema(BaseModel):
    host_id: str = Field(..., description="Identificador único del host/nodo")
    timestamp: int = Field(..., description="Timestamp Unix del muestreo")
    containers: List[ContainerMetricSchema] = Field(default_factory=list, description="Lista de métricas de contenedores")

class HostSummarySchema(BaseModel):
    host_id: str
    last_seen_at: datetime
    container_count: int
    total_cpu_percent: float
    total_ram_bytes: int
    total_egress_bytes_sec: float
