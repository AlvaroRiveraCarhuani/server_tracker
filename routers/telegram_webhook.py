import os
import json
from typing import Set
from fastapi import APIRouter, HTTPException, Request, status
from notifications.telegram import verify_and_parse_callback

router = APIRouter(prefix="/api/v1/chatops/telegram", tags=["ChatOps Telegram"])

def get_allowed_user_ids() -> Set[int]:
    raw = os.getenv("TELEGRAM_ALLOWED_USER_IDS", "12345678,98765432")
    return {int(uid.strip()) for uid in raw.split(",") if uid.strip().isdigit()}

@router.post("/webhook")
async def handle_telegram_webhook(request: Request):
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

    # Aquí se despacha la orden al canal de WebSocket reverso del host correspondiente
    return {
        "status": "accepted",
        "action": action,
        "host_id": host_id,
        "container_id": container_id,
        "message": f"Orden de {action} autorizada y despachada hacia el host {host_id}",
    }
