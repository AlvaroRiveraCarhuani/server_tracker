from fastapi import FastAPI
from routers import server 
from database import Base, engine
import models

app = FastAPI()

Base.metadata.create_all(bind=engine)
        
app.include_router(server.router)


@app.get("/")
def hello():
    return {"message":"Hello "}
