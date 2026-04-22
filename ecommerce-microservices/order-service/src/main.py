from fastapi import FastAPI, HTTPException, Depends
from fastapi.middleware.cors import CORSMiddleware
from sqlalchemy import create_engine, Column, Integer, String, Float, DateTime, Enum
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, Session
from pydantic import BaseModel
from typing import List, Optional
from datetime import datetime
import os
import enum

# App setup
app = FastAPI(title="Order Service", description="Order microservice for ECommerce", version="1.0.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

# Database
DATABASE_URL = os.getenv("DATABASE_URL", "sqlite:///./orders.db")
engine = create_engine(DATABASE_URL, connect_args={"check_same_thread": False} if "sqlite" in DATABASE_URL else {})
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()


# Enums
class OrderStatus(str, enum.Enum):
    PENDING = "PENDING"
    CONFIRMED = "CONFIRMED"
    PROCESSING = "PROCESSING"
    SHIPPED = "SHIPPED"
    DELIVERED = "DELIVERED"
    CANCELLED = "CANCELLED"


# Models
class Order(Base):
    __tablename__ = "orders"

    id = Column(Integer, primary_key=True, index=True)
    user_id = Column(String(100), nullable=False)
    product_id = Column(String(100), nullable=False)
    product_name = Column(String(255), nullable=False)
    quantity = Column(Integer, nullable=False)
    unit_price = Column(Float, nullable=False)
    total_price = Column(Float, nullable=False)
    status = Column(Enum(OrderStatus), default=OrderStatus.PENDING)
    shipping_address = Column(String(500))
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


Base.metadata.create_all(bind=engine)


# Pydantic schemas
class OrderCreate(BaseModel):
    user_id: str
    product_id: str
    product_name: str
    quantity: int
    unit_price: float
    shipping_address: Optional[str] = None


class OrderUpdate(BaseModel):
    status: Optional[OrderStatus] = None
    shipping_address: Optional[str] = None


class OrderResponse(BaseModel):
    id: int
    user_id: str
    product_id: str
    product_name: str
    quantity: int
    unit_price: float
    total_price: float
    status: OrderStatus
    shipping_address: Optional[str]
    created_at: datetime
    updated_at: Optional[datetime]

    class Config:
        from_attributes = True


# Dependency
def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


# Routes
@app.get("/health")
def health_check():
    return {"status": "UP", "service": "order-service"}


@app.post("/api/orders", response_model=OrderResponse, status_code=201)
def create_order(order: OrderCreate, db: Session = Depends(get_db)):
    total = order.quantity * order.unit_price
    db_order = Order(
        user_id=order.user_id,
        product_id=order.product_id,
        product_name=order.product_name,
        quantity=order.quantity,
        unit_price=order.unit_price,
        total_price=total,
        shipping_address=order.shipping_address
    )
    db.add(db_order)
    db.commit()
    db.refresh(db_order)
    return db_order


@app.get("/api/orders", response_model=List[OrderResponse])
def get_all_orders(skip: int = 0, limit: int = 100, db: Session = Depends(get_db)):
    return db.query(Order).offset(skip).limit(limit).all()


@app.get("/api/orders/{order_id}", response_model=OrderResponse)
def get_order(order_id: int, db: Session = Depends(get_db)):
    order = db.query(Order).filter(Order.id == order_id).first()
    if not order:
        raise HTTPException(status_code=404, detail="Order not found")
    return order


@app.get("/api/orders/user/{user_id}", response_model=List[OrderResponse])
def get_orders_by_user(user_id: str, db: Session = Depends(get_db)):
    return db.query(Order).filter(Order.user_id == user_id).all()


@app.put("/api/orders/{order_id}", response_model=OrderResponse)
def update_order(order_id: int, order_update: OrderUpdate, db: Session = Depends(get_db)):
    order = db.query(Order).filter(Order.id == order_id).first()
    if not order:
        raise HTTPException(status_code=404, detail="Order not found")
    if order_update.status:
        order.status = order_update.status
    if order_update.shipping_address:
        order.shipping_address = order_update.shipping_address
    order.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(order)
    return order


@app.delete("/api/orders/{order_id}")
def cancel_order(order_id: int, db: Session = Depends(get_db)):
    order = db.query(Order).filter(Order.id == order_id).first()
    if not order:
        raise HTTPException(status_code=404, detail="Order not found")
    order.status = OrderStatus.CANCELLED
    db.commit()
    return {"message": "Order cancelled successfully"}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=int(os.getenv("PORT", 8000)))
