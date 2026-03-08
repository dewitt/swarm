import subprocess
import os

# Build latest
print("Building Swarm...")
subprocess.run(["go", "build", "-o", "bin/swarm", "./cmd/swarm"])

# Run the query
print("\n--- Running agent query ---")
result = subprocess.run(
    ["./bin/swarm", "-p", "What's the biggest file in this repository?"],
    capture_output=True,
    text=True
)

print(result.stdout)
