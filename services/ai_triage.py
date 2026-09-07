import os
import logging
from typing import Optional
import httpx

logger = logging.getLogger(__name__)

OPENROUTER_API_URL = "https://openrouter.ai/api/v1/chat/completions"
DEFAULT_MODEL = "meta-llama/llama-3.3-70b-instruct:free"

class AITriageService:
    """
    Automated incident triage service powered by OpenRouter LLMs.
    Synthesizes container failure logs into a concise 2-line root cause diagnosis and recommendation.
    """

    def __init__(self, api_key: Optional[str] = None, model: Optional[str] = None):
        self.api_key = api_key or os.getenv("OPENROUTER_API_KEY")
        self.model = model or os.getenv("OPENROUTER_MODEL", DEFAULT_MODEL)

    async def diagnose_incident(
        self,
        container_name: str,
        image: str,
        status: str,
        logs: str = "",
        timeout: float = 8.0,
    ) -> str:
        """
        Queries OpenRouter for a fast 2-line diagnosis of a container incident.
        Returns a fallback diagnostic string if OpenRouter is unreachable or unconfigured.
        """
        if not self.api_key:
            return (
                "Diagnóstico IA no disponible (OPENROUTER_API_KEY no configurada).\n"
                "Revisar logs locales del contenedor para causa raíz."
            )

        system_prompt = (
            "Eres el motor de diagnóstico AIOps para SOLV Server Tracker. "
            "Analiza el fallo del contenedor y responde ESTRICTAMENTE en exactamente 2 líneas breves sin formato extra ni markdown:\n"
            "Línea 1: [Causa raíz probable]\n"
            "Línea 2: [Acción recomendada]"
        )

        user_content = (
            f"Contenedor: {container_name}\n"
            f"Imagen: {image}\n"
            f"Estado: {status}\n"
            f"Logs recientes:\n{logs[-1500:] if logs else 'Sin logs disponibles'}"
        )

        payload = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_content},
            ],
            "max_tokens": 120,
            "temperature": 0.2,
        }

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "HTTP-Referer": "https://github.com/alvaroriverac/server_tracker",
            "X-Title": "SOLV Server Tracker",
            "Content-Type": "application/json",
        }

        try:
            async with httpx.AsyncClient(timeout=timeout) as client:
                response = await client.post(OPENROUTER_API_URL, json=payload, headers=headers)
                if response.status_code == 200:
                    data = response.json()
                    diagnosis = data["choices"][0]["message"]["content"].strip()
                    return diagnosis
                else:
                    logger.warning(f"OpenRouter returned HTTP {response.status_code}: {response.text}")
                    return (
                        f"Diagnóstico no disponible (OpenRouter HTTP {response.status_code}).\n"
                        "Verificar cuota o modelo en OpenRouter."
                    )
        except httpx.TimeoutException:
            logger.warning("OpenRouter diagnosis timed out.")
            return "Diagnóstico no disponible (timeout en consulta OpenRouter).\nVerificar conectividad de red."
        except Exception as e:
            logger.error(f"Error during OpenRouter triage: {e}")
            return f"Diagnóstico no disponible ({str(e)}).\nRevisar logs del sistema."

# Singleton instance
ai_triage = AITriageService()
