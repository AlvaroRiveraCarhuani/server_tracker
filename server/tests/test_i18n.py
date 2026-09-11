from i18n import t, normalize_language
from notifications.telegram import verify_and_parse_callback, generate_signed_callback

def test_i18n_translation():
    # Español por defecto
    assert t("es", "telegram.unauthorized") == "Usuario no autorizado para ejecutar remediaciones"
    assert t("es", "telegram.btn_action", action="RESTART", ttl=60) == "[RESTART] Contenedor (60s)"
    
    # Inglés
    assert t("en", "telegram.unauthorized") == "Unauthorized user for remediations"
    assert t("en", "telegram.btn_action", action="RESTART", ttl=60) == "[RESTART] Container (60s)"

    # Clave no encontrada devuelve la clave misma
    assert t("es", "unknown.key") == "unknown.key"

def test_telegram_callback_bilingual_messages():
    secret = "test_secret_key"
    allowed_uids = {1001}
    cb = generate_signed_callback("restart", "host-1", "cont-1", secret)

    # Unauthorized en español
    ok, act, h, c, msg_es = verify_and_parse_callback(cb, secret, allowed_uids, 9999, lang="es")
    assert not ok
    assert msg_es == "Usuario no autorizado para ejecutar remediaciones"

    # Unauthorized en inglés
    ok, act, h, c, msg_en = verify_and_parse_callback(cb, secret, allowed_uids, 9999, lang="en")
    assert not ok
    assert msg_en == "Unauthorized user for remediations"

    # Válido en español
    ok, act, h, c, msg_valid_es = verify_and_parse_callback(cb, secret, allowed_uids, 1001, lang="es")
    assert ok
    assert msg_valid_es == "Válido"

    # Válido en inglés
    ok, act, h, c, msg_valid_en = verify_and_parse_callback(cb, secret, allowed_uids, 1001, lang="en")
    assert ok
    assert msg_valid_en == "Valid"
