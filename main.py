from fastapi import FastAPI
from database import Base, engine
from routers import server, target, telemetry, telegram_webhook, websocket_channel
import models.metrics

Base.metadata.create_all(bind=engine)

app = FastAPI(
    title="SOLV Server Tracker — Control Plane",
    description="Active Observability, ChatOps and AIOps Server for Docker Infrastructure",
    version="0.1.0",
)

app.include_router(telemetry.router)
app.include_router(websocket_channel.router)
app.include_router(telegram_webhook.router)
app.include_router(server.router)
app.include_router(target.router)

@app.get("/")
def root():
    return {
        "system": "SOLV Server Tracker",
        "status": "operational",
        "version": "0.1.0",
    }