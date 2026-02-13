import os
from typing import List
from .base import BaseNotifier
from .discord import DiscordNotifier


class NotificationManager:
    def __init__(self):
        self.channels: List[BaseNotifier] = []
        self._load_providers()

    def _load_providers(self):
        """Lee el .env y activa los canales disponibles"""
        
        # 1. Configurar Discord
        discord_url = os.getenv("DISCORD_WEBHOOK_URL")
        if discord_url:
            self.channels.append(DiscordNotifier(discord_url))
            print("📢 Notificaciones: Discord activado.")
        
        # 2. Telegram en el futuro:
        # if os.getenv("TELEGRAM_TOKEN"): ...

        if not self.channels:
            print("⚠️ Notificaciones: Ningún canal configurado (Solo logs).")

    def send_alert(self, server_name, status, url):
        """
        El método público que usa el monitor.
        Formatea el mensaje y lo manda a TODOS los canales activos.
        """
        # Creamos un mensaje bonito y genérico
        emoji = "🚨" if status == "down" else "✅"
        message = f"{emoji} **{server_name}** ({url}) está **{status.upper()}**"

        # Iteramos sobre todos los canales (Discord, Telegram, etc.)
        for channel in self.channels:
            channel.send(message)

# Instancia única (Singleton) para exportar
notifier = NotificationManager()