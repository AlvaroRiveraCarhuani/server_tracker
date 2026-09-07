import time
import hmac
import hashlib
import pytest
from auth.hmac_auth import sign_payload, verify_hmac_signature, is_timestamp_valid

def test_sign_and_verify_payload():
    secret = "super_secret_agent_key_123"
    raw_body = b'{"host_id":"node-1","timestamp":1700000000}'

    sig = sign_payload(raw_body, secret)
    assert isinstance(sig, str)
    assert len(sig) == 64  # SHA-256 hex string

    # Verificación válida
    assert verify_hmac_signature(raw_body, secret, sig) is True

    # Clave errónea
    assert verify_hmac_signature(raw_body, "wrong_secret", sig) is False

    # Payload alterado
    tampered_body = b'{"host_id":"node-1","timestamp":1700000999}'
    assert verify_hmac_signature(tampered_body, secret, sig) is False

def test_timestamp_window_validation():
    now = int(time.time())

    # Timestamp actual es válido
    assert is_timestamp_valid(now, max_skew_seconds=300) is True

    # Timestamp de hace 2 minutos es válido
    assert is_timestamp_valid(now - 120, max_skew_seconds=300) is True

    # Timestamp de hace 10 minutos (600s) está expirado
    assert is_timestamp_valid(now - 600, max_skew_seconds=300) is False

    # Timestamp del futuro lejano (+600s) es inválido
    assert is_timestamp_valid(now + 600, max_skew_seconds=300) is False
