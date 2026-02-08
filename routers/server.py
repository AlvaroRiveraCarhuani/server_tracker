from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session
from typing import List

from database import get_db
from models.server_log import ServerLog
from schemas.server_schema import ServerList, ServerLogResponse

router = APIRouter(prefix="/servers", tags=["Server Logs"])

@router.get("/history", response_model=List[ServerLogResponse])
def get_history(limit: int = 10, db: Session = Depends(get_db)):
    return db.query(ServerLog).order_by(ServerLog.timestamp.desc()).limit(limit).all()

@router.post("/report")
def report_status(data: ServerList, db: Session = Depends(get_db)):
    failed_servers = []
    
    for server in data.servers:
        if server.status == "down":
            new_log = ServerLog(
                target_name=server.name,
                status=server.status
            )
            db.add(new_log)
            failed_servers.append(server)

    if failed_servers:
        db.commit()
        return {"message": f"Recorded {len(failed_servers)} failed servers."}
    
    return {"message": "All systems operational (or no failures reported)."}