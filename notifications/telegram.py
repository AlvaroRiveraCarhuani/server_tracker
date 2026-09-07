import os
import time
import hmac
import hashlib
import logging
from typing import Optional, Tuple, Set
import httpx

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

CALLBACK_TTL_SECONDS = 60

def generate_signed_callback(action: str, host_id: str, container_id: str, secret: str) -> str:
    """
    Genera un callback_data con formato seguro:
    'act:{action}:{host_id}:{container_id}:{timestamp}:{signature_prefix}'
    """
    now = int(time.time())
    data_to_sign = f"{action}:{host_id}:{container_id}:{now}"
    sig = hmac.new(secret.encode("utf-8"), data_to_sign.encode("utf-8"), hashlib.sha256).hexdigest()[:8]
    return f"act:{action}:{host_id}:{container_id}:{now}:{sig}"

def verify_and_parse_callback(
    callback_data: str,
    secret: str,
    allowed_user_ids: Set[int],
    sender_user_id: int,
    max_ttl: int = CALLBACK_TTL_SECONDS,
) -> Tuple[bool, Optional[str], Optional[str], Optional[str], str]:
    """
    Valida el callback de Telegram:
    1. Lista blanca de usuarios autorizados (D5).
    2. Formato del payload.
    3. Ventana de tiempo (TTL <= 60s).
    4. Firma criptográfica HMAC.
    """
    if sender_user_id not in allowed_user_ids:
        return False, None, None, None, "Usuario no autorizado para ejecutar remediaciones"

    parts = callback_data.split(":")
    if len(parts) != 6 or parts[0] != "act":
        return False, None, None, None, "Estructura de callback inválida"

    _, action, host_id, container_id, ts_str, received_sig = parts

    try:
        ts = int(ts_str)
    except ValueError:
        return False, None, None, None, "Timestamp de callback no es numérico"

    now = int(time.time())
    if now - ts > max_ttl or ts - now > 10:
        return False, None, None, None, f"Botón expirado (TTL de {max_ttl}s superado)"

    data_to_sign = f"{action}:{host_id}:{container_id}:{ts}"
    expected_sig = hmac.new(secret.encode("utf-8"), data_to_sign.encode("utf-8"), hashlib.sha256).hexdigest()[:8]

    if not hmac.compare_digest(expected_sig, received_sig):
        return False, None, None, None, "Firma de callback inválida o alterada"

    return True, action, host_id, container_id, "Válido"

async def send_interactive_telegram_alert(
    message: str,
    host_id: str,
    container_id: str,
    action: str = "restart",
    chat_id: Optional[str] = None,
    secret: Optional[str] = None,
    ai_diagnosis: Optional[str] = None,
) -> bool:
    """Envía un mensaje interactivo con botón firmado a Telegram, opcionalmente con triaje de IA."""
    bot_token = os.getenv("TELEGRAM_BOT_TOKEN")
    target_chat_id = chat_id or os.getenv("TELEGRAM_CHAT_ID")
    shared_secret = secret or os.getenv("SOLV_TELEGRAM_SECRET", "default_telegram_secret_999")

    if not bot_token or not target_chat_id:
        logger.warning("Credenciales de Telegram no configuradas")
        return False

    full_message = message
    if ai_diagnosis:
        full_message += f"\n\n*Diagnóstico AIOps:*\n{ai_diagnosis}"

    callback_data = generate_signed_callback(action, host_id, container_id, shared_secret)

    url = f"https://api.telegram.org/bot{bot_token}/sendMessage"
    payload = {
        "chat_id": target_chat_id,
        "text": full_message,
        "parse_mode": "Markdown",
        "reply_markup": {
            "inline_keyboard": [
                [
                    {
                        "text": f"[{action.upper()}] Contenedor (60s)",
                        "callback_data": callback_data,
                    }
                ]
            ]
        },
    }

    async with httpx.AsyncClient() as client:
        try:
            resp = await client.post(url, json=payload, timeout=5.0)
            resp.raise_for_status()
            return True
        except Exception as e:
            logger.error(f"Error enviando mensaje a Telegram: {e}")
            return False