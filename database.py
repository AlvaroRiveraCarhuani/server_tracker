import os
from sqlalchemy import create_engine, URL
from sqlalchemy.orm import sessionmaker, declarative_base

database_url = os.getenv("DATABASE_URL")

if database_url:
    engine = create_engine(database_url)
elif os.getenv("POSTGRES_HOST"):
    url_object = URL.create(
        "postgresql+pg8000",
        username=os.getenv("POSTGRES_USER", "postgres"),
        password=os.getenv("POSTGRES_PASSWORD", "postgres"),
        host=os.getenv("POSTGRES_HOST", "localhost"),
        database=os.getenv("POSTGRES_DB", "server_tracker"),
    )
    engine = create_engine(url_object)
else:
    # Fallback para pruebas locales / SQLite
    engine = create_engine("sqlite:///./server_tracker.db", connect_args={"check_same_thread": False})

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
