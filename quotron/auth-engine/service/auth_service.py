import os
import logging
from datetime import datetime, timedelta
from typing import Dict, Optional

from fastapi import FastAPI, Depends, HTTPException, status
from fastapi.security import OAuth2PasswordBearer, OAuth2PasswordRequestForm
from jose import JWTError, jwt
from passlib.context import CryptContext
from pydantic import BaseModel

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI(title="Quotron Auth Engine")

# Security configurations
SECRET_KEY = os.getenv("AUTH_SECRET_KEY", "dev_secret_key_replace_in_production")
ALGORITHM = "HS256"
ACCESS_TOKEN_EXPIRE_MINUTES = 30

# Password context for hashing
pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")

# OAuth2 password bearer token
oauth2_scheme = OAuth2PasswordBearer(tokenUrl="token")

# Models
class User(BaseModel):
    username: str
    email: Optional[str] = None
    disabled: Optional[bool] = None

class UserInDB(User):
    hashed_password: str

class Token(BaseModel):
    access_token: str
    token_type: str

class TokenData(BaseModel):
    username: Optional[str] = None

class SessionData(BaseModel):
    cookies: Dict[str, str]
    headers: Dict[str, str]
    expiration: datetime

# Mock database - would be replaced with a real database in production
fake_users_db = {
    "testuser": {
        "username": "testuser",
        "email": "test@example.com",
        "hashed_password": pwd_context.hash("password123"),
        "disabled": False,
    }
}

# Session storage - would be replaced with a database in production
sessions = {}

def verify_password(plain_password, hashed_password):
    return pwd_context.verify(plain_password, hashed_password)

def get_user(db, username: str):
    if username in db:
        user_dict = db[username]
        return UserInDB(**user_dict)
    return None

def authenticate_user(db, username: str, password: str):
    user = get_user(db, username)
    if not user:
        return False
    if not verify_password(password, user.hashed_password):
        return False
    return user

def create_access_token(data: dict, expires_delta: Optional[timedelta] = None):
    to_encode = data.copy()
    
    if expires_delta:
        expire = datetime.utcnow() + expires_delta
    else:
        expire = datetime.utcnow() + timedelta(minutes=15)
    
    to_encode.update({"exp": expire})
    encoded_jwt = jwt.encode(to_encode, SECRET_KEY, algorithm=ALGORITHM)
    return encoded_jwt

async def get_current_user(token: str = Depends(oauth2_scheme)):
    credentials_exception = HTTPException(
        status_code=status.HTTP_401_UNAUTHORIZED,
        detail="Could not validate credentials",
        headers={"WWW-Authenticate": "Bearer"},
    )
    
    try:
        payload = jwt.decode(token, SECRET_KEY, algorithms=[ALGORITHM])
        username: str = payload.get("sub")
        if username is None:
            raise credentials_exception
        token_data = TokenData(username=username)
    except JWTError:
        raise credentials_exception
    
    user = get_user(fake_users_db, username=token_data.username)
    if user is None:
        raise credentials_exception
    return user

async def get_current_active_user(current_user: User = Depends(get_current_user)):
    if current_user.disabled:
        raise HTTPException(status_code=400, detail="Inactive user")
    return current_user

@app.post("/token", response_model=Token)
async def login_for_access_token(form_data: OAuth2PasswordRequestForm = Depends()):
    user = authenticate_user(fake_users_db, form_data.username, form_data.password)
    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Incorrect username or password",
            headers={"WWW-Authenticate": "Bearer"},
        )
    
    access_token_expires = timedelta(minutes=ACCESS_TOKEN_EXPIRE_MINUTES)
    access_token = create_access_token(
        data={"sub": user.username}, expires_delta=access_token_expires
    )
    
    return {"access_token": access_token, "token_type": "bearer"}

@app.post("/sessions/{site_name}")
async def store_session(
    site_name: str, 
    cookies: Dict[str, str], 
    headers: Dict[str, str] = {},
    current_user: User = Depends(get_current_active_user)
):
    """Store a browser session for a specific site."""
    
    session_id = f"{current_user.username}_{site_name}"
    session_data = SessionData(
        cookies=cookies,
        headers=headers,
        expiration=datetime.utcnow() + timedelta(hours=1)
    )
    
    sessions[session_id] = session_data
    logger.info(f"Session stored for {site_name}")
    
    return {"status": "success", "message": f"Session for {site_name} stored successfully"}

@app.get("/sessions/{site_name}")
async def get_session(
    site_name: str, 
    current_user: User = Depends(get_current_active_user)
):
    """Get a stored browser session for a specific site."""
    
    session_id = f"{current_user.username}_{site_name}"
    
    if session_id not in sessions:
        raise HTTPException(
            status_code=404,
            detail=f"No session found for {site_name}"
        )
    
    session = sessions[session_id]
    
    # Check if session is expired
    if session.expiration < datetime.utcnow():
        del sessions[session_id]
        raise HTTPException(
            status_code=410,
            detail=f"Session for {site_name} has expired"
        )
    
    return {
        "cookies": session.cookies,
        "headers": session.headers,
        "expiration": session.expiration
    }

@app.delete("/sessions/{site_name}")
async def delete_session(
    site_name: str, 
    current_user: User = Depends(get_current_active_user)
):
    """Delete a stored browser session for a specific site."""
    
    session_id = f"{current_user.username}_{site_name}"
    
    if session_id in sessions:
        del sessions[session_id]
        logger.info(f"Session for {site_name} deleted")
        return {"status": "success", "message": f"Session for {site_name} deleted successfully"}
    
    return {"status": "success", "message": f"No session found for {site_name}"}

@app.get("/users/me", response_model=User)
async def read_users_me(current_user: User = Depends(get_current_active_user)):
    """Get current user information."""
    return current_user

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)