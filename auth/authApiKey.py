import os
from fastapi import HTTPException, Security, status
from fastapi.security import APIKeyHeader

api_key_header = APIKeyHeader(name="x-api-key", auto_error=False)

def get_api_key(api_key_header: str = Security(api_key_header)):
    CORRECT_KEY = os.getenv("API_SECRET_KEY")
    
    if api_key_header != CORRECT_KEY:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail=" Acceso Denegado: API Key inválida o faltante."
        )
    
    return api_key_header