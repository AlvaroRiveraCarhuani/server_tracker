from fastapi import FastAPI
from routers import server 
app = FastAPI()

@app.get("/")
def hello():
    return {"message":"Hello "}

app.include_router(server.router)