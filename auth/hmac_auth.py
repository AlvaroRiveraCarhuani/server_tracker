import hmac
import hashlib
import time
from typing import Optional
from fastapi import Header, HTTPException, Request, status

def sign_payload(payload: bytes, secret: str) -> str:
    """Calcula la firma HMAC-SHA256 en formato hexadecimal."""
    return hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest()

def verify_hmac_signature(payload: bytes, secret: str, signature: str) -> bool:
    """Valida la firma HMAC-SHA256 de forma resistente a ataques de temporización."""
    expected = sign_payload(payload, secret)
    return hmac.compare_digest(expected, signature)

def is_timestamp_valid(req_timestamp: int, max_skew_seconds: int = 300) -> bool:
    """Verifica que el timestamp no esté fuera de la ventana de tolerancia (replay protection)."""
    current_time = int(time.time())
    return abs(current_time - req_timestamp) <= max_skew_seconds

async def verify_hmac_header(
    request: Request,
    x_solv_signature: Optional[str] = Header(None, alias="X-Solv-Signature"),
    x_solv_timestamp: Optional[str] = Header(None, alias="X-Solv-Timestamp"),
) -> bytes:
    """
    Dependencia FastAPI que valida la cabecera HMAC y el timestamp antes de permitir la ingestión.
    """
    if not x_solv_signature or not x_solv_timestamp:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Cabeceras criptográficas X-Solv-Signature y X-Solv-Timestamp requeridas",
        )

    try:
        ts = int(x_solv_timestamp)
    except ValueError:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Cabecera X-Solv-Timestamp no es un entero válido",
        )

    if not is_timestamp_valid(ts):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Timestamp expirado o fuera de ventana permitida (posible ataque de replay)",
        )

    body = await request.body()
    return body
