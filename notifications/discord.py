import httpx
import logging
from .base import BaseNotifier

logger = logging.getLogger(__name__)

class DiscordNotifier(BaseNotifier):
    def __init__(self, webhook_url: str):
        self.webhook_url = webhook_url

    def send(self, message: str):
        if not self.webhook_url:
            return

        try:
            payload = {"content": message}
            response = httpx.post(self.webhook_url, json=payload)
            
            if response.status_code == 204:
                logger.info(" Discord: Notificación enviada.")
            else:
                logger.error(f" Discord Error: {response.status_code}")
        except Exception as e:
            logger.error(f" Discord Fallo: {e}")