import logging
from fastapi import APIRouter, WebSocket, WebSocketDisconnect, status, Depends
from sqlalchemy.orm import Session
from database import get_db
from models.metrics import Host
from auth.hmac_auth import verify_hmac_signature, is_timestamp_valid
from services.connection_manager import manager

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/v1/ws", tags=["Reverse WebSocket Channel"])

@router.websocket("/agent/{host_id}")
async def agent_websocket_endpoint(
    websocket: WebSocket,
    host_id: str,
    db: Session = Depends(get_db),
):
    """
    Reverse WebSocket control channel endpoint.
    Agents dial this endpoint to receive remediation actions without opening host ports.
    Authenticates the handshake via HMAC-SHA256 signature and timestamp.
    """
    # Extract signature and timestamp from headers (or query params as fallback)
    signature = websocket.headers.get("x-solv-signature") or websocket.query_params.get("signature")
    timestamp_str = websocket.headers.get("x-solv-timestamp") or websocket.query_params.get("timestamp")

    if not signature or not timestamp_str:
        logger.warning(f"WebSocket rejected for host '{host_id}': missing HMAC credentials")
        await websocket.close(code=status.WS_1008_POLICY_VIOLATION, reason="Missing cryptographic authentication headers")
        return

    try:
        timestamp = int(timestamp_str)
    except ValueError:
        logger.warning(f"WebSocket rejected for host '{host_id}': invalid timestamp format")
        await websocket.close(code=status.WS_1008_POLICY_VIOLATION, reason="Invalid timestamp format")
        return

    if not is_timestamp_valid(timestamp, max_skew_seconds=300):
        logger.warning(f"WebSocket rejected for host '{host_id}': expired timestamp")
        await websocket.close(code=status.WS_1008_POLICY_VIOLATION, reason="Timestamp expired (replay protection)")
        return

    host = db.query(Host).filter(Host.id == host_id).first()
    if not host:
        logger.warning(f"WebSocket rejected: host '{host_id}' not registered in database")
        await websocket.close(code=status.WS_1008_POLICY_VIOLATION, reason="Host not found")
        return

    # Handshake payload canonical string: f"{host_id}:{timestamp}"
    handshake_payload = f"{host_id}:{timestamp}".encode("utf-8")
    if not verify_hmac_signature(handshake_payload, host.secret_key, signature):
        logger.warning(f"WebSocket rejected for host '{host_id}': invalid HMAC signature")
        await websocket.close(code=status.WS_1008_POLICY_VIOLATION, reason="Invalid HMAC signature")
        return

    # Handshake valid: register connection
    await manager.connect(host_id, websocket)

    try:
        while True:
            raw_text = await websocket.receive_text()
            manager.handle_message(host_id, raw_text)
    except WebSocketDisconnect:
        manager.disconnect(host_id, websocket)
    except Exception as e:
        logger.error(f"Error on WebSocket connection with host '{host_id}': {e}")
        manager.disconnect(host_id, websocket)
