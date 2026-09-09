from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session
from sqlalchemy.exc import IntegrityError
from typing import List
from database import get_db
from models.target import Target
from schemas.target_schema import TargetCreate, TargetResponse, TargetUpdate
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

@router.delete("/{id}", dependencies=[Depends(get_api_key)])
def delete_target(id: int, db: Session = Depends(get_db)):
    target = db.get(Target, id)
    
    if not target:
        raise HTTPException(status_code=404, detail=f"Target {id} not found")
    try:
        db.delete(target)
        db.commit()
        return {"message": f"Target {id} deleted successfully"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error to delete target")

router.patch("/{id}", dependencies=[Depends(get_api_key)])
def update_target(id: int, target_data: TargetUpdate, db: Session= Depends(get_db)):
    target = db.get(Target, id)

    if not target:
        raise HTTPException(status_code=404, detail=f"Target {id} not found")
    update_dict = target_data.model_dump(exclude_unset=True)
    
    if not update_dict:
        raise HTTPException(status_code=400, detail=f"No fields provided for update")
    try:
        for key, value in update_dict.items():
            setattr(target,key,value)
        db.commit()
        db.refresh(target)
        return {"message": f"Successful update", "data": target}
    except Exception as e:
        db.rollback()
        raise HTTPException(status_code=500, detail=f"Error to update, {id}")
