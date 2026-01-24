from database import  Base
from sqlalchemy import Column, Integer, String, DateTime
from datetime import datetime
from database import SessionLocal
class ServerLog(Base):
    __tablename__ = "registro_caidas" 

    id = Column(Integer, primary_key=True, index=True)
    nombre_servidor = Column(String)
    status = Column(String)
    fecha = Column(DateTime, default=datetime.utcnow)
