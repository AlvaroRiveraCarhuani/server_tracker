from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session
from sqlalchemy.exc import IntegrityError
from typing import List
from database import get_db
from models.target import Target
from schemas.target_schema import TargetCreate, TargetResponse
from auth.authApiKey import get_api_key 

router = APIRouter(prefix="/targets", tags=["Targets Configuration"])

@router.get("/", response_model=List[TargetResponse])
def list_targets(db: Session = Depends(get_db)):
    return db.query(Target).all()

@router.post("/", response_model=TargetResponse, dependencies=[Depends(get_api_key)])
def create_target(target: TargetCreate, db: Session = Depends(get_db)):
    try:
        new_target = Target(name=target.name, url=str(target.url))
        db.add(new_target)
        db.commit()
        db.refresh(new_target)
        return new_target
    except IntegrityError:
        db.rollback()
        raise HTTPException(status_code=400, detail="A target with this name already exists.")