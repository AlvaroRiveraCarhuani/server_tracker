import time
import json
import pytest
from fastapi.testclient import TestClient
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from database import Base, get_db
from main import app
from models.metrics import Host
from auth.hmac_auth import sign_payload

# Base de datos en memoria para pruebas compartida con StaticPool
TEST_DB_URL = "sqlite:///:memory:"
test_engine = create_engine(
    TEST_DB_URL,
    connect_args={"check_same_thread": False},
    poolclass=StaticPool,
)
TestingSessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=test_engine)

def override_get_db():
    db = TestingSessionLocal()
    try:
        yield db
    finally:
        db.close()

app.dependency_overrides[get_db] = override_get_db

@pytest.fixture(autouse=True)
def setup_database():
    Base.metadata.create_all(bind=test_engine)
    db = TestingSessionLocal()
    # Crear un host preexistente
    host = Host(id="srv-node-01", name="Primary Node", secret_key="secret_key_prod_abc")
    db.add(host)
    db.commit()
    db.close()
    yield
    Base.metadata.drop_all(bind=test_engine)

client = TestClient(app)

def test_ingest_telemetry_valid_signature():
    secret = "secret_key_prod_abc"
    now_ts = int(time.time())

    payload_dict = {
        "host_id": "srv-node-01",
        "timestamp": now_ts,
        "containers": [
            {
                "id": "c1234567890a",
                "name": "redis-cache",
                "image": "redis:7-alpine",
                "status": "running",
                "cpu_percent": 15.5,
                "ram_bytes": 64000000,
                "ram_limit_bytes": 512000000,
                "egress_bytes_sec": 2048.0,
                "ingress_bytes_sec": 4096.0,
                "pids": 4
            }
        ]
    }
    raw_body = json.dumps(payload_dict).encode("utf-8")
    sig = sign_payload(raw_body, secret)

    headers = {
        "X-Solv-Signature": sig,
        "X-Solv-Timestamp": str(now_ts),
        "Content-Type": "application/json"
    }

    response = client.post("/api/v1/telemetry/ingest", content=raw_body, headers=headers)
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "success"
    assert data["containers_ingested"] == 1

def test_ingest_telemetry_invalid_signature_rejected():
    now_ts = int(time.time())
    payload_dict = {
        "host_id": "srv-node-01",
        "timestamp": now_ts,
        "containers": []
    }
    raw_body = json.dumps(payload_dict).encode("utf-8")

    headers = {
        "X-Solv-Signature": "invalid_hex_signature_00000000000000000000000000000000000000000",
        "X-Solv-Timestamp": str(now_ts),
        "Content-Type": "application/json"
    }

    response = client.post("/api/v1/telemetry/ingest", content=raw_body, headers=headers)
    assert response.status_code == 401

def test_get_live_metrics():
    # Primero insertamos métricas válidas
    test_ingest_telemetry_valid_signature()

    response = client.get("/api/v1/telemetry/srv-node-01/live")
    assert response.status_code == 200
    containers = response.json()
    assert len(containers) == 1
    assert containers[0]["name"] == "redis-cache"
    assert containers[0]["cpu_percent"] == 15.5
