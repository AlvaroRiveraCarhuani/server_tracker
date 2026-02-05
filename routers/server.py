from fastapi import Depends
from sqlalchemy.orm import Session
from models.serverlogs import ServerLog
from database import get_db
from typing import List
from schemas import ServerLogOut,ServerCheckResponse,ServerList
from fastapi import APIRouter

router=APIRouter(
    prefix="/server",
    tags=["Servers"]
)

    
@router.get("/", response_model=List[ServerLogOut])
def list_logs(limit: int = 10, db: Session = Depends(get_db)):
    logs = db.query(ServerLog).order_by(ServerLog.fecha.desc()).limit(limit).all()
    return logs


@router.post("/", response_model=ServerCheckResponse)
def check_servers(data: ServerList, db: Session = Depends(get_db)):
    servidores_caidos = []
    
    for servidor in data.servers:
        if servidor.status == "down":
            nuevo_registro = ServerLog(
                nombre_servidor=servidor.nombre,
                status=servidor.status
            )
                    
            db.add(nuevo_registro)
            
            servidores_caidos.append(servidor)

    if len(servidores_caidos) > 0:
        db.commit() 
        
        return {
            "alerta": True, 
            "mensaje": f"Se guardaron {len(servidores_caidos)} servidores en la base de datos.",
            "lista": servidores_caidos
        }
    else:
        return {"alerta": False, "mensaje": "Todos los sistemas operativos"}