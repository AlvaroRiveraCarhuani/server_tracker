from datetime import datetime
from sqlalchemy import Column, Integer, String, Float, BigInteger, DateTime, ForeignKey, Index
from sqlalchemy.orm import relationship
from database import Base

class Host(Base):
    __tablename__ = "hosts"

    id = Column(String(64), primary_key=True, index=True)
    name = Column(String(128), nullable=True)
    secret_key = Column(String(128), nullable=False)  # Pre-shared key para validación HMAC
    last_seen_at = Column(DateTime, default=datetime.utcnow, nullable=False)
    created_at = Column(DateTime, default=datetime.utcnow, nullable=False)

    metrics = relationship("ContainerMetric", back_populates="host", cascade="all, delete-orphan")

class ContainerMetric(Base):
    __tablename__ = "container_metrics"

    id = Column(Integer, primary_key=True, autoincrement=True)
    host_id = Column(String(64), ForeignKey("hosts.id", ondelete="CASCADE"), nullable=False, index=True)
    container_id = Column(String(64), nullable=False, index=True)
    container_name = Column(String(128), nullable=False)
    image = Column(String(256), nullable=False)
    status = Column(String(32), nullable=False)
    cpu_percent = Column(Float, default=0.0, nullable=False)
    ram_bytes = Column(BigInteger, default=0, nullable=False)
    ram_limit_bytes = Column(BigInteger, default=0, nullable=False)
    egress_bytes_sec = Column(Float, default=0.0, nullable=False)
    ingress_bytes_sec = Column(Float, default=0.0, nullable=False)
    pids = Column(Integer, default=0, nullable=False)
    timestamp = Column(DateTime, default=datetime.utcnow, nullable=False, index=True)

    host = relationship("Host", back_populates="metrics")

    __table_args__ = (
        Index("ix_host_container_time", "host_id", "container_id", "timestamp"),
    )

class AuditLog(Base):
    __tablename__ = "audit_logs"

    id = Column(Integer, primary_key=True, autoincrement=True)
    host_id = Column(String(64), ForeignKey("hosts.id", ondelete="CASCADE"), nullable=False, index=True)
    container_id = Column(String(64), nullable=False, index=True)
    action = Column(String(32), nullable=False)
    source = Column(String(32), nullable=False)  # 'telegram', 'mcp', 'automated'
    status = Column(String(32), nullable=False)  # 'success', 'failed'
    message = Column(String(512), nullable=True)
    created_at = Column(DateTime, default=datetime.utcnow, nullable=False, index=True)

