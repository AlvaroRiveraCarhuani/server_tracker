import os
import time
import json
import pytest
from unittest.mock import AsyncMock
from fastapi.testclient import TestClient
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from database import Base, get_db
from main import app
from models.metrics import Host, AuditLog
from notifications.telegram import generate_signed_callback, verify_and_parse_callback
from services.connection_manager import manager

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

@pytest.fixture(autouse=True)
def setup_database():
    Base.metadata.create_all(bind=test_engine)
    app.dependency_overrides[get_db] = override_get_db
    db = TestingSessionLocal()
    host = Host(id="srv-node-01", name="Primary Node", secret_key="telegram_secret_pass")
    db.add(host)
    db.commit()
    db.close()
    yield
    app.dependency_overrides.pop(get_db, None)
    Base.metadata.drop_all(bind=test_engine)

client = TestClient(app)

def test_telegram_callback_valid():
    secret = "telegram_secret_pass"
    allowed_users = {111222333, 444555666}
    sender_id = 111222333

    cb_data = generate_signed_callback(
        action="restart",
        host_id="srv-1",
        container_id="cont-abc",
        secret=secret,
    )

    valid, action, host_id, container_id, reason = verify_and_parse_callback(
        callback_data=cb_data,
        secret=secret,
        allowed_user_ids=allowed_users,
        sender_user_id=sender_id,
        max_ttl=60,
    )

    assert valid is True
    assert action == "restart"
    assert host_id == "srv-1"
    assert container_id == "cont-abc"

def test_telegram_callback_unauthorized_user():
    secret = "telegram_secret_pass"
    allowed_users = {111222333}
    intruder_id = 999999999

    cb_data = generate_signed_callback("restart", "srv-1", "cont-abc", secret)
    valid, _, _, _, reason = verify_and_parse_callback(
        cb_data, secret, allowed_users, intruder_id
    )

    assert valid is False
    assert "no autorizado" in reason

def test_telegram_callback_expired_ttl():
    secret = "telegram_secret_pass"
    allowed_users = {111222333}
    sender_id = 111222333

    old_ts = int(time.time()) - 100
    cb_data = f"act:restart:srv-1:cont-abc:{old_ts}:fake_sig"

    valid, _, _, _, reason = verify_and_parse_callback(
        cb_data, secret, allowed_users, sender_id, max_ttl=60
    )

    assert valid is False
    assert "expirado" in reason.lower()

def test_telegram_webhook_dispatch_and_audit_logging():
    secret = "default_telegram_secret_999"
    os.environ["SOLV_TELEGRAM_SECRET"] = secret
    os.environ["TELEGRAM_ALLOWED_USER_IDS"] = "12345678"

    cb_data = generate_signed_callback(
        action="restart",
        host_id="srv-node-01",
        container_id="c-redis-01",
        secret=secret,
    )

    webhook_payload = {
        "update_id": 1001,
        "callback_query": {
            "id": "cbq-001",
            "from": {"id": 12345678, "first_name": "Admin"},
            "data": cb_data,
        },
    }

    response = client.post("/api/v1/chatops/telegram/webhook", json=webhook_payload)
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "execution_failed"
    assert "no tiene una conexión de control WebSocket activa" in data["message"]

    # Verify audit log recorded in database
    db = TestingSessionLocal()
    logs = db.query(AuditLog).all()
    assert len(logs) == 1
    assert logs[0].host_id == "srv-node-01"
    assert logs[0].container_id == "c-redis-01"
    assert logs[0].action == "restart"
    assert logs[0].source == "telegram"
    assert logs[0].status == "failed"
    db.close()
