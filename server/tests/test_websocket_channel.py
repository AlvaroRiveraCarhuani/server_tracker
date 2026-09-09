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
from models.metrics import Host
from auth.hmac_auth import sign_payload
from services.connection_manager import ConnectionManager, manager

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
    host = Host(id="srv-node-01", name="Primary Node", secret_key="super_secret_ws_key")
    db.add(host)
    db.commit()
    db.close()
    yield
    app.dependency_overrides.pop(get_db, None)
    Base.metadata.drop_all(bind=test_engine)

client = TestClient(app)

def test_websocket_rejected_without_auth():
    with pytest.raises(Exception):
        with client.websocket_connect("/api/v1/ws/agent/srv-node-01"):
            pass

def test_websocket_rejected_with_invalid_signature():
    ts = str(int(time.time()))
    headers = {
        "x-solv-signature": "bad_hex_signature_12345",
        "x-solv-timestamp": ts,
    }
    with pytest.raises(Exception):
        with client.websocket_connect("/api/v1/ws/agent/srv-node-01", headers=headers):
            pass

def test_websocket_rejected_with_expired_timestamp():
    expired_ts = str(int(time.time()) - 400)
    sig = sign_payload(f"srv-node-01:{expired_ts}".encode(), "super_secret_ws_key")
    headers = {
        "x-solv-signature": sig,
        "x-solv-timestamp": expired_ts,
    }
    with pytest.raises(Exception):
        with client.websocket_connect("/api/v1/ws/agent/srv-node-01", headers=headers):
            pass

def test_websocket_successful_handshake_lifecycle():
    ts = str(int(time.time()))
    sig = sign_payload(f"srv-node-01:{ts}".encode(), "super_secret_ws_key")
    headers = {
        "x-solv-signature": sig,
        "x-solv-timestamp": ts,
    }

    with client.websocket_connect("/api/v1/ws/agent/srv-node-01", headers=headers) as ws:
        assert manager.is_connected("srv-node-01")
        # Send heartbeat/ping from agent
        ws.send_text(json.dumps({"type": "ping", "timestamp": int(time.time())}))

    # Once exited context, connection is cleanly removed
    assert not manager.is_connected("srv-node-01")

@pytest.mark.anyio
async def test_connection_manager_dispatch_and_ack():
    test_manager = ConnectionManager()
    mock_ws = AsyncMock()

    await test_manager.connect("srv-test", mock_ws)
    assert test_manager.is_connected("srv-test")

    # Hook send_text on mock_ws to simulate agent responding with ACK
    async def simulate_agent_ack(msg_json):
        data = json.loads(msg_json)
        cmd_id = data["id"]
        ack = {
            "id": cmd_id,
            "success": True,
            "message": "Container restarted successfully",
            "error": None,
            "timestamp": int(time.time()),
        }
        test_manager.handle_message("srv-test", json.dumps(ack))

    mock_ws.send_text.side_effect = simulate_agent_ack

    result = await test_manager.send_command("srv-test", "restart", "cont-99", timeout=2.0)
    assert result["success"] is True
    assert result["message"] == "Container restarted successfully"

@pytest.mark.anyio
async def test_connection_manager_action_whitelist_guard():
    test_manager = ConnectionManager()
    mock_ws = AsyncMock()
    await test_manager.connect("srv-test", mock_ws)

    # Prohibited action (RCE attempt like exec or rm)
    with pytest.raises(ValueError, match="is not permitted"):
        await test_manager.send_command("srv-test", "exec_bash", "cont-99")
