from database import Base
from sqlalchemy import Column, Integer, String

class Target(Base):
    __tablename__ = "targets"

    id = Column(Integer, primary_key=True, index=True)
    name = Column(String, unique=True, index=True)
    url = Column(String, nullable=False)