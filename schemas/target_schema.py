from pydantic import BaseModel, HttpUrl

class TargetCreate(BaseModel):
    name: str
    url: HttpUrl 

class TargetResponse(BaseModel):
    id: int
    name: str
    url: str

    model_config = {
        "from_attributes": True
    }