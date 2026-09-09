from datetime import datetime
import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from database import Base
from models.metrics import Host, ContainerMetric
from mcp_server import get_infrastructure_overview, detect_anomalies_and_egress_spikes

@pytest.fixture
def db_session():
    test_engine = create_engine("sqlite:///:memory:", connect_args={"check_same_thread": False}, poolclass=StaticPool)
    Base.metadata.create_all(bind=test_engine)
    Session = sessionmaker(bind=test_engine)
    session = Session()

    # Seed data
    host = Host(id="host-alpha", name="Alpha Server", secret_key="k1")
    session.add(host)

    now = datetime.utcnow()
    c1 = ContainerMetric(
        host_id="host-alpha",
        container_id="c1",
        container_name="db-prod",
        image="postgres:16",
        status="running",
        cpu_percent=25.0,
        ram_bytes=980 * 1024 * 1024,
        ram_limit_bytes=1024 * 1024 * 1024, # 95.7% RAM -> OOM risk
        egress_bytes_sec=15 * 1024 * 1024,   # 15 MB/s -> Egress spike
        timestamp=now,
    )
    c2 = ContainerMetric(
        host_id="host-alpha",
        container_id="c2",
        container_name="web-frontend",
        image="nginx:alpine",
        status="running",
        cpu_percent=2.0,
        ram_bytes=30 * 1024 * 1024,
        ram_limit_bytes=512 * 1024 * 1024,
        egress_bytes_sec=1024,
        timestamp=now,
    )
    session.add(c1)
    session.add(c2)
    session.commit()

    yield session
    session.close()

def test_get_infrastructure_overview(db_session):
    overview = get_infrastructure_overview(db_session)
    assert overview["total_hosts"] == 1
    assert overview["total_containers_running"] == 2
    assert overview["hosts"][0]["host_id"] == "host-alpha"
    assert overview["hosts"][0]["running_count"] == 2

def test_detect_anomalies_and_egress_spikes(db_session):
    anomalies = detect_anomalies_and_egress_spikes(
        db_session,
        egress_threshold_mb_s=10.0,
        ram_percent_threshold=90.0,
    )

    assert len(anomalies) == 2
    types = [a["type"] for a in anomalies]
    assert "FINANCIAL_CIRCUIT_BREAKER_EGRESS_SPIKE" in types
    assert "MEMORY_OOM_RISK" in types
