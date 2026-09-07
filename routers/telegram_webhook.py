import os
from datetime import datetime
from typing import Set
from fastapi import APIRouter, HTTPException, Request, Depends, status
from sqlalchemy.orm import Session
from database import get_db
from models.metrics import AuditLog
from notifications.telegram import verify_and_parse_callback
from services.connection_manager import manager

router = APIRouter(prefix="/api/v1/chatops/telegram", tags=["ChatOps Telegram"])

def get_allowed_user_ids() -> Set[int]:
    raw = os.getenv("TELEGRAM_ALLOWED_USER_IDS", "12345678,98765432")
    return {int(uid.strip()) for uid in raw.split(",") if uid.strip().isdigit()}

@router.post("/webhook")
async def handle_telegram_webhook(request: Request, db: Session = Depends(get_db)):
    """Receptor seguro de webhooks de Telegram para interacción por botones."""
    try:
        body = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Cuerpo JSON inválido")

    callback_query = body.get("callback_query")
    if not callback_query:
        return {"status": "ignored", "reason": "No es un callback_query"}

    sender_user_id = callback_query.get("from", {}).get("id")
    callback_data = callback_query.get("data", "")
    secret = os.getenv("SOLV_TELEGRAM_SECRET", "default_telegram_secret_999")
    allowed_users = get_allowed_user_ids()

    valid, action, host_id, container_id, reason = verify_and_parse_callback(
        callback_data=callback_data,
        secret=secret,
        allowed_user_ids=allowed_users,
        sender_user_id=sender_user_id,
    )

    if not valid:
        return {
            "status": "rejected",
            "reason": reason,
        }

    # Despachar orden al canal de WebSocket reverso del host correspondiente
    exec_status = "failed"
    exec_msg = ""

    if not manager.is_connected(host_id):
        exec_msg = f"Host '{host_id}' no tiene una conexión de control WebSocket activa."
    else:
        try:
            res = await manager.send_command(host_id, action, container_id, timeout=10.0)
            if res.get("success"):
                exec_status = "success"
                exec_msg = res.get("message", "Acción completada")
            else:
                exec_msg = res.get("message") or res.get("error", "Error desconocido en ejecución")
        except Exception as e:
            exec_msg = f"Error despachando comando: {str(e)}"

    # Registrar en auditoría inmutable
    audit = AuditLog(
        host_id=host_id,
        container_id=container_id,
        action=action,
        source="telegram",
        status=exec_status,
        message=exec_msg[:500] if exec_msg else None,
        created_at=datetime.utcnow(),
    )
    db.add(audit)
    db.commit()

    return {
        "status": "accepted" if exec_status == "success" else "execution_failed",
        "action": action,
        "host_id": host_id,
        "container_id": container_id,
        "execution_status": exec_status,
        "message": exec_msg,
    }

