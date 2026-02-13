import os
import httpx
import logging

# Configurar logs para ver qué pasa
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class Notifier:
    def __init__(self):
        # Leemos la configuración del entorno
        self.discord_url = os.getenv("DISCORD_WEBHOOK_URL")

    def send_alert(self, server_name, status, url):
        """
        Envía una alerta a los canales configurados.
        """
        message = f"🚨 **ALERTA CRÍTICA** 🚨\n\nEl servidor **{server_name}** ({url}) está **{status.upper()}**."
        
        # 1. Intentar enviar a Discord si existe la URL
        if self.discord_url:
            self._send_discord(message)
        else:
            logger.warning("Discord URL no configurada. Saltando notificación.")

    def _send_discord(self, message):
        try:
            payload = {"content": message}
            response = httpx.post(self.discord_url, json=payload)
            if response.status_code == 204: # Discord devuelve 204 No Content si todo salió bien
                logger.info("✅ Notificación enviada a Discord.")
            else:
                logger.error(f"❌ Error Discord: {response.status_code} - {response.text}")
        except Exception as e:
            logger.error(f"❌ Fallo al conectar con Discord: {e}")

# Instancia global para usar en otros lados
notifier = Notifier()