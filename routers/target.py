from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session
from typing import List

from database import get_db
from models.target import Target
from schemas.target_schema import TargetCreate, TargetResponse

router = APIRouter(prefix="/targets", tags=["Targets Configuration"])

@router.post("/", response_model=TargetResponse)
def create_target(target: TargetCreate, db: Session = Depends(get_db)):
    new_target = Target(name=target.name, url=str(target.url))
    db.add(new_target)
    db.commit()
    db.refresh(new_target)
    return new_target

@router.get("/", response_model=List[TargetResponse])
def list_targets(db: Session = Depends(get_db)):
    return db.query(Target).all()