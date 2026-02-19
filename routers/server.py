from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session
from typing import List

from database import get_db
from models.server_log import ServerLog
from schemas.server_schema import ServerList, ServerLogResponse
from auth.authApiKey import get_api_key

router = APIRouter(prefix="/servers", tags=["Server Logs"])

@router.get("/history", response_model=List[ServerLogResponse])
def get_history(limit: int = 10, db: Session = Depends(get_db)):
    return db.query(ServerLog).order_by(ServerLog.timestamp.desc()).limit(limit).all()

@router.post("/report", dependencies=[Depends(get_api_key)])
def report_status(data: ServerList, db: Session = Depends(get_db)):
    failed_servers = []

    for server in data.servers:
        new_log = ServerLog(
            target_name=server.name,
            status=server.status
        )
        db.add(new_log)
        
        if server.status == "down":
            failed_servers.append(server)

    db.commit() 
    
    if failed_servers:
        return {"message": f"Recorded status. {len(failed_servers)} servers are DOWN."}
    
    return {"message": "All systems operational. Log updated."}
    