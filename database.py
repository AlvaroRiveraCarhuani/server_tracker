from sqlalchemy import create_engine, URL
from sqlalchemy.orm import sessionmaker, declarative_base
import os 

url_object = URL.create(
    "postgresql+pg8000",
    username=os.getenv("POSTGRES_USER"),
    password=os.getenv("POSTGRES_PASSWORD"),  
    host=os.getenv("POSTGRES_HOST"),
    database=os.getenv("POSTGRES_DB"),
)

engine = create_engine(url_object)

Base = declarative_base()

SessionLocal = sessionmaker(
    autocommit=False,
    autoflush=False,
    bind=engine,
)

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
