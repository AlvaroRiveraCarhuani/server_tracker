from pydantic import BaseModel
from datetime import datetime
from typing import List

class ServerStatus(BaseModel):
    name: str
    status: str

class ServerList(BaseModel):
    servers: List[ServerStatus]

class ServerLogResponse(BaseModel):
    id: int
    target_name: str
    status: str
    timestamp: datetime

    model_config = {
        "from_attributes": True
    }