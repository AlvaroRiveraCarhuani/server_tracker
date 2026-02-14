import os
from typing import List
from .base import BaseNotifier
from .discord import DiscordNotifier

class NotificationManager:
    def __init__(self):
        self.channels: List[BaseNotifier] = []
        self._load_providers()

    def _load_providers(self):
        discord_url = os.getenv("DISCORD_WEBHOOK_URL")
        if discord_url:
            self.channels.append(DiscordNotifier(discord_url))
            print("📢 Manager: Discord channel activated.", flush=True)
        
        if not self.channels:
            print("⚠️ Manager: No notification channels configured.", flush=True)

    def send_alert(self, server_name, status, url):
        emoji = "🚨" if status == "down" else "✅"
        message = f"{emoji} **ALERT**: Server **{server_name}** ({url}) is **{status.upper()}**"

        for channel in self.channels:
            channel.send(message)

notifier = NotificationManager()