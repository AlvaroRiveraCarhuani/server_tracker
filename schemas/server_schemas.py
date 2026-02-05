from pydantic import BaseModel
from datetime import datetime
from typing import List

class Server(BaseModel):
    nombre: str
    status: str  
    
class ServerList(BaseModel):
    servers: List[Server]


class ServerCheckResponse(BaseModel):
    alerta: bool
    mensaje: str
    lista: List[Server] = []

    
class ServerLogOut(BaseModel):
    id: int
    nombre_servidor: str
    status: str
    fecha: datetime
    
    model_config={
        "from_attributes": True
    }