import time
import pytest
from notifications.telegram import generate_signed_callback, verify_and_parse_callback

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

    # Timestamp de hace 100 segundos
    old_ts = int(time.time()) - 100
    cb_data = f"act:restart:srv-1:cont-abc:{old_ts}:fake_sig"

    valid, _, _, _, reason = verify_and_parse_callback(
        cb_data, secret, allowed_users, sender_id, max_ttl=60
    )

    assert valid is False
    assert "expirado" in reason.lower()
