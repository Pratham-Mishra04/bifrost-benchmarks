# LLM Lite API

A FastAPI-based service for LLM completions using LiteLLM.

## Features

- Rate limiting (60 requests per minute)
- Proper error handling and validation
- Async support for better performance
- CORS enabled
- Comprehensive logging
- Health check endpoint
- API key verification
- Support for both OpenAI and Anthropic models

## Setup

1. Install dependencies:
```bash
pip install -r requirements.txt
```

2. Configure your API keys in the `.env` file:
```
OPENAI_API_KEY=your-openai-key
ANTHROPIC_API_KEY=your-anthropic-key
```

3. Run the server:
```bash
uvicorn api:app --reload
```

The API will be available at `http://localhost:8000`

## API Endpoints

### POST /v1/completions
Send a completion request to the LLM.

Request body:
```json
{
    "messages": [
        {
            "content": "Hello, how are you?",
            "role": "user"
        }
    ],
    "model": "openai/gpt-4",
    "temperature": 0.7,
    "max_tokens": 1000
}
```

Response:
```json
{
    "id": "chatcmpl-123",
    "model": "gpt-4",
    "choices": [
        {
            "message": {
                "role": "assistant",
                "content": "Hello! I'm doing well, thank you for asking."
            }
        }
    ],
    "usage": {
        "prompt_tokens": 9,
        "completion_tokens": 12,
        "total_tokens": 21
    }
}
```

### GET /health
Check the health status of the API.

Response:
```json
{
    "status": "healthy",
    "version": "1.0.0",
    "supported_models": [
        "openai/gpt-4",
        "anthropic/claude-3-sonnet-20240229"
    ]
}
```

## Rate Limiting

- Completion endpoint: 60 requests per minute
- Health check endpoint: 5 requests per minute

## Error Handling

The API returns appropriate HTTP status codes and error messages:

- 400: Bad Request (invalid input)
- 429: Too Many Requests (rate limit exceeded)
- 500: Internal Server Error
- 504: Gateway Timeout

## Documentation

Once the server is running, you can access:
- Interactive API documentation: `http://localhost:8000/docs`
- Alternative API documentation: `http://localhost:8000/redoc`

## Production Deployment

For production deployment, consider:
1. Using a proper WSGI server like Gunicorn
2. Setting up proper logging
3. Implementing API key rotation
4. Setting up monitoring and alerting
5. Using a reverse proxy like Nginx
6. Implementing proper security measures 