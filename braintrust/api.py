from fastapi import FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
from typing import List, Optional
from openai import OpenAI
import argparse

# Parse command line arguments
parser = argparse.ArgumentParser(description='Braintrust API Server')
parser.add_argument('--braintrust-key', required=True, help='Braintrust API key')
parser.add_argument('--port', type=int, default=8000, help='Port to run the server on')
args = parser.parse_args()

# Initialize FastAPI app
app = FastAPI()

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Initialize OpenAI client for Braintrust
client = OpenAI(
    base_url="https://api.braintrust.dev/v1/proxy",
    api_key=args.braintrust_key,
)

# Models
class Message(BaseModel):
    content: str = Field(..., min_length=1)
    role: str = Field(..., pattern="^(user|assistant|system)$")

class CompletionRequest(BaseModel):
    messages: List[Message] = Field(..., min_items=1)
    model: str = Field(..., min_length=1)

class ErrorResponse(BaseModel):
    error: str
    detail: Optional[str] = None

# Completion endpoint
@app.post("/v1/chat/completions")
async def create_completion(
    request: Request,
    completion_request: CompletionRequest,
):
    try:
        # Create completion
        response = client.chat.completions.create(
            model=completion_request.model,
            messages=[msg.dict() for msg in completion_request.messages],
        )
        
        return response
            
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=args.port)
