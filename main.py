from fastapi import FastAPI
from routers import server,target
from database import Base, engine
import models

app = FastAPI()

Base.metadata.create_all(bind=engine)
        
app.include_router(server.router)
app.include_router(target.router)

@app.get("/")
def hello():
    return {"message":"Hello "}
