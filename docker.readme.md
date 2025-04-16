docker build -t bifrost-env .

🛠️ Step-by-Step: Run & Curl Inside the Same Container

✅ 1. Start the container in detached mode (so it stays running)
Run the container with interactive mode and a name:

docker run -dit --name bifrost-container bifrost-env
-d = detached (run in background)
-i = interactive
-t = pseudo-TTY
--name gives the container a name for easy access
✅ 2. Open a shell in the container (1st terminal)
docker exec -it bifrost-container bash
Inside that shell, run your Go server:

go run bifrost-benchmarks/main.go
# or whatever your main file is
(Make sure it binds to 0.0.0.0:3001 or :3001)

✅ 3. Open a second terminal into the same container
In a new terminal window/tab, run:

docker exec -it bifrost-container bash
Now you're in the same container, different shell session.

✅ 4. Curl the API
Inside that second terminal, you can now:

curl http://localhost:3001
It should hit the Go server running in the first terminal.