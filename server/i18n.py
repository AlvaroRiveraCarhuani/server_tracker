"""
Módulo i18n para el servidor FastAPI y notificaciones de Telegram (Ola 8).
C1: Placeholders nombrados ({name}, {host}, etc.).
C2: Config key único o detección; clave desconocida devuelve la clave misma sin crash.
"""

from typing import Dict, Any

CATALOG: Dict[str, Dict[str, str]] = {
    "es": {
        "telegram.alert_title": "Alerta de contenedor anómalo en {host}",
        "telegram.btn_action": "[{action}] Contenedor ({ttl}s)",
        "telegram.unauthorized": "Usuario no autorizado para ejecutar remediaciones",
        "telegram.invalid_format": "Estructura de callback inválida",
        "telegram.invalid_timestamp": "Timestamp de callback no es numérico",
        "telegram.expired": "Botón expirado (TTL de {ttl}s superado)",
        "telegram.invalid_sig": "Firma de callback inválida o alterada",
        "telegram.valid": "Válido",
    },
    "en": {
        "telegram.alert_title": "Anomalous container alert on {host}",
        "telegram.btn_action": "[{action}] Container ({ttl}s)",
        "telegram.unauthorized": "Unauthorized user for remediations",
        "telegram.invalid_format": "Invalid callback structure",
        "telegram.invalid_timestamp": "Callback timestamp is not numeric",
        "telegram.expired": "Expired button (TTL of {ttl}s exceeded)",
        "telegram.invalid_sig": "Invalid or tampered callback signature",
        "telegram.valid": "Valid",
    },
}

def normalize_language(lang: str) -> str:
    clean = (lang or "").lower().strip()
    if clean.startswith("es"):
        return "es"
    return "en"

def t(lang: str, key: str, **kwargs: Any) -> str:
    """
    Traduce una clave i18n usando placeholders nombrados.
    Si la clave no existe, retorna la clave misma como fallback visible (C1).
    """
    normalized = normalize_language(lang)
    lang_dict = CATALOG.get(normalized, CATALOG["en"])
    template = lang_dict.get(key)
    if template is None:
        return key

    if not kwargs:
        return template

    try:
        return template.format(**kwargs)
    except KeyError:
        return template
