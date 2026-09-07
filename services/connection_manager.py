import asyncio
import json
import logging
import uuid
import time
from typing import Dict, Optional, Any
from fastapi import WebSocket

logger = logging.getLogger(__name__)

class ConnectionManager:
    """
    Manages reverse WebSocket connections from Go agents (Data Plane).
    Enables sending remediation commands to connected hosts and awaiting typed acknowledgements.
    """

    def __init__(self):
        # host_id -> WebSocket
        self.active_connections: Dict[str, WebSocket] = {}
        # command_id -> asyncio.Future
        self._pending_commands: Dict[str, asyncio.Future] = {}

    async def connect(self, host_id: str, websocket: WebSocket) -> None:
        """Registers a newly authenticated host agent WebSocket."""
        await websocket.accept()
        # If an existing socket exists for this host, cleanly close or replace it
        if host_id in self.active_connections:
            try:
                await self.active_connections[host_id].close(code=1000, reason="Replaced by new connection")
            except Exception:
                pass
        self.active_connections[host_id] = websocket
        logger.info(f"Host agent '{host_id}' connected via reverse WebSocket.")

    def disconnect(self, host_id: str, websocket: Optional[WebSocket] = None) -> None:
        """Removes the host agent socket when disconnected."""
        current = self.active_connections.get(host_id)
        if current and (websocket is None or current == websocket):
            del self.active_connections[host_id]
            logger.info(f"Host agent '{host_id}' disconnected from reverse WebSocket.")

    def is_connected(self, host_id: str) -> bool:
        """Checks whether a specific host agent is actively connected."""
        return host_id in self.active_connections

    async def send_command(
        self,
        host_id: str,
        action: str,
        container_id: str,
        timeout: float = 10.0,
    ) -> Dict[str, Any]:
        """
        Sends an authorized remediation action to the host agent and waits for an execution ACK.
        Permitted actions: 'restart', 'stop', 'isolate_network'.
        """
        allowed_actions = {"restart", "stop", "isolate_network"}
        if action not in allowed_actions:
            raise ValueError(f"Action '{action}' is not permitted. Whitelist: {allowed_actions}")

        websocket = self.active_connections.get(host_id)
        if not websocket:
            raise ConnectionError(f"Host agent '{host_id}' is not currently connected.")

        command_id = f"cmd-{uuid.uuid4().hex[:8]}"
        payload = {
            "id": command_id,
            "action": action,
            "container_id": container_id,
            "timestamp": int(time.time()),
        }

        loop = asyncio.get_running_loop()
        future: asyncio.Future = loop.create_future()
        self._pending_commands[command_id] = future

        try:
            await websocket.send_text(json.dumps(payload))
            # Await response from agent with timeout
            response = await asyncio.wait_for(future, timeout=timeout)
            return response
        except asyncio.TimeoutError:
            logger.warning(f"Command {command_id} to host {host_id} timed out after {timeout}s.")
            return {
                "id": command_id,
                "success": False,
                "message": f"Execution timed out after {timeout} seconds",
                "error": "timeout",
                "timestamp": int(time.time()),
            }
        finally:
            self._pending_commands.pop(command_id, None)

    def handle_message(self, host_id: str, raw_message: str) -> None:
        """
        Processes incoming messages from the host agent (e.g. command execution ACKs, pong).
        """
        try:
            data = json.loads(raw_message)
            cmd_id = data.get("id")
            if cmd_id and cmd_id in self._pending_commands:
                future = self._pending_commands[cmd_id]
                if not future.done():
                    loop = future.get_loop()
                    loop.call_soon_threadsafe(future.set_result, data)
        except Exception as e:
            logger.error(f"Failed to parse incoming WebSocket message from {host_id}: {e}")


# Global singleton instance
manager = ConnectionManager()
