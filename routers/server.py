from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session
from typing import List

from database import get_db
from models.server_log import ServerLog
from schemas.server_schema import ServerList, ServerLogResponse, ServerUpdate
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
    
@router.delete("/report", dependencies=[Depends(get_api_key)])
def delete_report(id:int, db: Session = Depends(get_db)):
    report = db.get(ServerLog, id)
    if not report:
        raise HTTPException(status_code=404, detail="Report not found")
    
    try:
        db.delete(report)
        db.commit()
        return {"message": f"Report {id} was successfully deleted"}
    except Exception as e:
        db.rollback()
        raise HTTPException(status_code=500, detail=f"Error deleting error: {str(e)}")

@router.patch("/report/{id}", dependencies=[Depends(get_api_key)])
def modify_report(id: int, update_data: ServerUpdate, db: Session = Depends(get_db)):
    report = db.get(ServerLog, id)
    if not report:
        raise HTTPException(status_code=404, detail=f"Report {id} not found")

    update_dict = update_data.model_dump(exclude_unset=True)
    
    if not update_dict:
        raise HTTPException(status_code=400, detail="No fields provided for update")

    try:
        for key, value in update_dict.items():
            setattr(report, key, value)
            
        db.commit()
        db.refresh(report)
        return {"message": "Successful update", "data": report}
        
    except Exception as e:
        db.rollback()
        raise HTTPException(status_code=500, detail=f"Error updating report {id}: {str(e)}")