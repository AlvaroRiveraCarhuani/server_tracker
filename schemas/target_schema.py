from pydantic import BaseModel, HttpUrl, Field
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

class TargetUpdate(BaseModel):
    name: str | None = Field(default=None, min_length=1)
    url: HttpUrl | None = None