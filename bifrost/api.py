import argparse
import json
import signal
import subprocess
import sys
import time
import httpx
import uvicorn
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse
from contextlib import asynccontextmanager

# Global variables
bifrost_process = None
openai_api_key = None
bifrost_port = 3039  # Static port for Bifrost Go server

@asynccontextmanager
async def lifespan(app: FastAPI):
    global bifrost_process, openai_api_key
    
    # Startup
    try:
        bifrost_process = subprocess.Popen(
            [
                "go", "run", "main.go",
                "--openai-key", openai_api_key,
                "--port", str(bifrost_port),
                # "--proxy", "http://localhost:8080"
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        
        # Wait for server to start up
        print("Starting Bifrost server...")
        time.sleep(5)
        
        # Check if process is still running
        if bifrost_process.poll() is not None:
            stdout, stderr = bifrost_process.communicate()
            print(f"Bifrost failed to start: {stderr}")
            sys.exit(1)
            
        print(f"Bifrost server started on port {bifrost_port}")
    except Exception as e:
        print(f"Failed to start Bifrost server: {e}")
        sys.exit(1)

    yield

    # Shutdown
    if bifrost_process:
        print("Shutting down Bifrost server...")
        bifrost_process.terminate()
        try:
            bifrost_process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            bifrost_process.kill()
        print("Bifrost server shut down")

app = FastAPI(lifespan=lifespan)

@app.post("/v1/chat/completions")
async def chat_completions(request: Request):
    # Forward request to Bifrost
    try:
        # Get request body
        body = await request.json()
        
        # Create client with appropriate timeout
        async with httpx.AsyncClient(timeout=60.0) as client:
            response = await client.post(
                f"http://localhost:{bifrost_port}/v1/chat/completions",
                json=body,
                headers={"Content-Type": "application/json"}
            )
            
            # Return response from Bifrost
            return JSONResponse(
                content=response.json(),
                status_code=response.status_code
            )
    except httpx.RequestError as e:
        raise HTTPException(status_code=500, detail=f"Error communicating with Bifrost: {str(e)}")
    except json.JSONDecodeError:
        raise HTTPException(status_code=500, detail="Invalid response from Bifrost server")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Unexpected error: {str(e)}")

def handle_sigterm(signum, frame):
    # Handle termination gracefully
    print("Received termination signal, shutting down...")
    if bifrost_process:
        bifrost_process.terminate()
    sys.exit(0)

if __name__ == "__main__":
    # Set up argument parser
    parser = argparse.ArgumentParser(description="Bifrost API Wrapper")
    parser.add_argument("--openai-key", required=True, help="OpenAI API key")
    parser.add_argument("--port", type=int, default=3001, help="Port to run the FastAPI server on")
    
    # Parse arguments
    args = parser.parse_args()
    openai_api_key = args.openai_key
    
    # Register signal handlers
    signal.signal(signal.SIGTERM, handle_sigterm)
    signal.signal(signal.SIGINT, handle_sigterm)
    
    # Start FastAPI server
    uvicorn.run(app, host="0.0.0.0", port=args.port) 