from database import Base
from sqlalchemy import Column, Integer, String, DateTime
from datetime import datetime

class ServerLog(Base):
    __tablename__ = "server_logs" 

    id = Column(Integer, primary_key=True, index=True)
    target_name = Column(String, index=True) 
    status = Column(String)
    timestamp = Column(DateTime, default=datetime.utcnow) 