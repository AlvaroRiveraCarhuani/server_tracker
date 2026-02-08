from fastapi import FastAPI
from database import Base, engine
from routers import server, target
import models

Base.metadata.create_all(bind=engine)

app = FastAPI(title="Server Tracker API")

app.include_router(server.router)
app.include_router(target.router)

@app.get("/")
def root():
    return {"message": "Server Tracker API is running"}