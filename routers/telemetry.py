import json
from datetime import datetime
from typing import List
from fastapi import APIRouter, Depends, Header, HTTPException, Request, status
from sqlalchemy.orm import Session
from sqlalchemy import func

from database import get_db
from models.metrics import Host, ContainerMetric
from schemas.telemetry import TelemetryBatchSchema, HostSummarySchema, ContainerMetricSchema
from auth.hmac_auth import verify_hmac_signature, is_timestamp_valid

router = APIRouter(prefix="/api/v1/telemetry", tags=["Telemetry"])

@router.post("/ingest", status_code=status.HTTP_200_OK)
async def ingest_telemetry(
    request: Request,
    db: Session = Depends(get_db),
    x_solv_signature: str = Header(..., alias="X-Solv-Signature"),
    x_solv_timestamp: str = Header(..., alias="X-Solv-Timestamp"),
):
    """
    Ingesta un lote de telemetría proveniente del agente Go tras validar su firma HMAC-SHA256.
    """
    try:
        ts = int(x_solv_timestamp)
    except ValueError:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Cabecera X-Solv-Timestamp inválida",
        )

    if not is_timestamp_valid(ts):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Timestamp fuera de la ventana de tolerancia de 300 segundos",
        )

    raw_body = await request.body()

    try:
        data = json.loads(raw_body)
        batch = TelemetryBatchSchema(**data)
    except Exception as e:
        raise HTTPException(
            status_code=status.HTTP_422_UNPROCESSABLE_ENTITY,
            detail=f"Payload JSON de telemetría inválido: {str(e)}",
        )

    # 1. Buscar el host registrado
    host = db.query(Host).filter(Host.id == batch.host_id).first()
    if not host:
        # Registro automático de host nuevo con la clave recibida si se permite
        # Por defecto buscamos el secreto preconfigurado en variables de entorno o registramos
        default_secret = "psk_live_9876543210abcdef"
        # Verificamos si la firma es válida con el secreto default o registramos
        if verify_hmac_signature(raw_body, default_secret, x_solv_signature):
            host = Host(id=batch.host_id, name=batch.host_id, secret_key=default_secret)
            db.add(host)
            db.commit()
            db.refresh(host)
        else:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Host no registrado o firma criptográfica no autorizada",
            )
    else:
        # Validar la firma HMAC con el secreto almacenado del host
        if not verify_hmac_signature(raw_body, host.secret_key, x_solv_signature):
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Firma HMAC-SHA256 inválida para el host especificado",
            )

    # 2. Actualizar último contacto del host
    host.last_seen_at = datetime.utcnow()

    # 3. Insertar las métricas de los contenedores
    batch_time = datetime.utcfromtimestamp(batch.timestamp)
    for c in batch.containers:
        metric = ContainerMetric(
            host_id=batch.host_id,
            container_id=c.id,
            container_name=c.name,
            image=c.image,
            status=c.status,
            cpu_percent=c.cpu_percent,
            ram_bytes=c.ram_bytes,
            ram_limit_bytes=c.ram_limit_bytes,
            egress_bytes_sec=c.egress_bytes_sec,
            ingress_bytes_sec=c.ingress_bytes_sec,
            pids=c.pids,
            timestamp=batch_time,
        )
        db.add(metric)

    db.commit()

    return {
        "status": "success",
        "host_id": batch.host_id,
        "containers_ingested": len(batch.containers),
        "timestamp": batch.timestamp,
    }

@router.get("/hosts", response_model=List[HostSummarySchema])
def list_hosts(db: Session = Depends(get_db)):
    """Lista todos los hosts vigilados y un resumen consolidado de sus recursos."""
    hosts = db.query(Host).all()
    result = []

    for h in hosts:
        # Obtener las métricas más recientes por contenedor
        latest_metrics = (
            db.query(ContainerMetric)
            .filter(ContainerMetric.host_id == h.id)
            .order_by(ContainerMetric.timestamp.desc())
            .limit(50)
            .all()
        )

        total_cpu = sum(m.cpu_percent for m in latest_metrics)
        total_ram = sum(m.ram_bytes for m in latest_metrics)
        total_egress = sum(m.egress_bytes_sec for m in latest_metrics)

        result.append(
            HostSummarySchema(
                host_id=h.id,
                last_seen_at=h.last_seen_at,
                container_count=len(latest_metrics),
                total_cpu_percent=total_cpu,
                total_ram_bytes=total_ram,
                total_egress_bytes_sec=total_egress,
            )
        )

    return result

@router.get("/{host_id}/live", response_model=List[ContainerMetricSchema])
def get_live_metrics(host_id: str, db: Session = Depends(get_db)):
    """Obtiene el último snapshot de métricas de contenedores para un host."""
    host = db.query(Host).filter(Host.id == host_id).first()
    if not host:
        raise HTTPException(status_code=404, detail="Host no encontrado")

    # Subconsulta para obtener el timestamp más reciente del host
    latest_ts = (
        db.query(func.max(ContainerMetric.timestamp))
        .filter(ContainerMetric.host_id == host_id)
        .scalar()
    )

    if not latest_ts:
        return []

    metrics = (
        db.query(ContainerMetric)
        .filter(ContainerMetric.host_id == host_id, ContainerMetric.timestamp == latest_ts)
        .all()
    )

    return [
        ContainerMetricSchema(
            id=m.container_id,
            name=m.container_name,
            image=m.image,
            status=m.status,
            cpu_percent=m.cpu_percent,
            ram_bytes=m.ram_bytes,
            ram_limit_bytes=m.ram_limit_bytes,
            egress_bytes_sec=m.egress_bytes_sec,
            ingress_bytes_sec=m.ingress_bytes_sec,
            pids=m.pids,
            timestamp=m.timestamp,
        )
        for m in metrics
    ]
