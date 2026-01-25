from fastapi import FastAPI
from routers import server 
from database import Base, engine
import models

app = FastAPI()

@app.on_event("startup")
def startup():
    Base.metadata.create_all(bind=engine)
        
@app.get("/")
def hello():
    return {"message":"Hello "}

app.include_router(server.router)