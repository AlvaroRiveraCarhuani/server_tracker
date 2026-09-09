import pytest
from unittest.mock import AsyncMock, patch
import httpx
from services.ai_triage import AITriageService

@pytest.mark.anyio
async def test_ai_triage_fallback_without_api_key():
    service = AITriageService(api_key=None)
    diagnosis = await service.diagnose_incident(
        container_name="web_api",
        image="python:3.12",
        status="exited",
        logs="Error: database connection timeout",
    )
    assert "no disponible" in diagnosis
    assert "OPENROUTER_API_KEY" in diagnosis

@pytest.mark.anyio
async def test_ai_triage_successful_openrouter_response():
    service = AITriageService(api_key="fake-key-test")

    mock_response = {
        "choices": [
            {
                "message": {
                    "content": "[Causa probable]: Conexión a base de datos agotada por pool de conexiones saturado.\n[Acción recomendada]: Reiniciar el contenedor e incrementar max_connections."
                }
            }
        ]
    }

    from unittest.mock import MagicMock

    mock_client = AsyncMock()
    mock_post = MagicMock()
    mock_post.status_code = 200
    mock_post.json.return_value = mock_response
    mock_client.post.return_value = mock_post
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("httpx.AsyncClient", return_value=mock_client):
        diagnosis = await service.diagnose_incident(
            container_name="web_api",
            image="python:3.12",
            status="exited",
            logs="Error: too many connections to postgres",
        )

    assert "[Causa probable]" in diagnosis
    assert "[Acción recomendada]" in diagnosis
    assert "Reiniciar el contenedor" in diagnosis

@pytest.mark.anyio
async def test_ai_triage_timeout_fallback():
    service = AITriageService(api_key="fake-key-test")

    mock_client = AsyncMock()
    mock_client.post.side_effect = httpx.TimeoutException("Timeout after 8s")
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    with patch("httpx.AsyncClient", return_value=mock_client):
        diagnosis = await service.diagnose_incident(
            container_name="web_api",
            image="python:3.12",
            status="exited",
        )

    assert "timeout" in diagnosis.lower()
