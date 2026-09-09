from typing import Dict, List, Any
from sqlalchemy.orm import Session
from sqlalchemy import func
from models.metrics import Host, ContainerMetric

def get_infrastructure_overview(db: Session) -> Dict[str, Any]:
    """Herramienta MCP: Retorna el resumen consolidado de la infraestructura para agentes de IA."""
    hosts = db.query(Host).all()
    overview = {
        "total_hosts": len(hosts),
        "hosts": [],
        "total_containers_running": 0,
        "total_cpu_allocated_percent": 0.0,
        "total_ram_allocated_bytes": 0,
    }

    for h in hosts:
        latest_ts = (
            db.query(func.max(ContainerMetric.timestamp))
            .filter(ContainerMetric.host_id == h.id)
            .scalar()
        )

        containers = []
        if latest_ts:
            containers = (
                db.query(ContainerMetric)
                .filter(ContainerMetric.host_id == h.id, ContainerMetric.timestamp == latest_ts)
                .all()
            )

        running = [c for c in containers if c.status == "running"]
        cpu_sum = sum(c.cpu_percent for c in containers)
        ram_sum = sum(c.ram_bytes for c in containers)

        overview["total_containers_running"] += len(running)
        overview["total_cpu_allocated_percent"] += cpu_sum
        overview["total_ram_allocated_bytes"] += ram_sum

        overview["hosts"].append({
            "host_id": h.id,
            "last_seen_at": h.last_seen_at.isoformat() if h.last_seen_at else None,
            "containers_count": len(containers),
            "running_count": len(running),
            "cpu_percent": round(cpu_sum, 2),
            "ram_mb": round(ram_sum / (1024 * 1024), 2),
        })

    return overview

def detect_anomalies_and_egress_spikes(
    db: Session,
    egress_threshold_mb_s: float = 10.0,
    ram_percent_threshold: float = 90.0,
) -> List[Dict[str, Any]]:
    """Herramienta MCP: Detecta anomalías críticas de consumo de RAM (riesgo de OOM) y picos de salida de red."""
    anomalies = []
    hosts = db.query(Host).all()

    for h in hosts:
        latest_ts = (
            db.query(func.max(ContainerMetric.timestamp))
            .filter(ContainerMetric.host_id == h.id)
            .scalar()
        )
        if not latest_ts:
            continue

        containers = (
            db.query(ContainerMetric)
            .filter(ContainerMetric.host_id == h.id, ContainerMetric.timestamp == latest_ts)
            .all()
        )

        for c in containers:
            egress_mb_s = c.egress_bytes_sec / (1024 * 1024)
            ram_pct = 0.0
            if c.ram_limit_bytes > 0:
                ram_pct = (c.ram_bytes / c.ram_limit_bytes) * 100.0

            # 1. Alerta de Cortocircuito Financiero (Egress Spike)
            if egress_mb_s >= egress_threshold_mb_s:
                anomalies.append({
                    "host_id": h.id,
                    "container_id": c.container_id,
                    "container_name": c.container_name,
                    "type": "FINANCIAL_CIRCUIT_BREAKER_EGRESS_SPIKE",
                    "severity": "CRITICAL",
                    "current_value": f"{egress_mb_s:.2f} MB/s",
                    "threshold": f"{egress_threshold_mb_s:.2f} MB/s",
                    "suggested_action": "isolate_network",
                })

            # 2. Alerta de Riesgo OOM (Memoria al límite)
            if ram_pct >= ram_percent_threshold:
                anomalies.append({
                    "host_id": h.id,
                    "container_id": c.container_id,
                    "container_name": c.container_name,
                    "type": "MEMORY_OOM_RISK",
                    "severity": "HIGH",
                    "current_value": f"{ram_pct:.1f}% ({c.ram_bytes / (1024*1024):.1f} MB)",
                    "threshold": f"{ram_percent_threshold:.1f}%",
                    "suggested_action": "restart",
                })

    return anomalies
