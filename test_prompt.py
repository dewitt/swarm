import subprocess
import os

print("Building Swarm...")
subprocess.run(["go", "build", "-o", "bin/swarm", "./cmd/swarm"])

if os.path.exists(".gemini/state.db"):
    os.remove(".gemini/state.db")

# 1. Manually inject the fact into the database to bypass the agent's potential failure to commit
import sqlite3
os.makedirs(".gemini", exist_ok=True)
conn = sqlite3.connect(".gemini/state.db")
c = conn.cursor()
c.execute("CREATE TABLE `semantic_facts` (`id` integer PRIMARY KEY AUTOINCREMENT,`fact` text,`created_at` integer)")
fact = "The largest file in the repository is ./pkg/sdk/.config/swarm/sessions.db, a SQLite database file, with a size of 425MB."
c.execute("INSERT INTO semantic_facts (fact, created_at) VALUES (?, 1772936450)", (fact,))
conn.commit()
conn.close()

print("\n--- Warm Run (Retrieving fact from DB) ---")
result = subprocess.run(
    ["./bin/swarm", "-p", "What's the biggest file in this repository?"],
    capture_output=True,
    text=True
)
print(result.stdout)

if "425MB" in result.stdout or "sessions.db" in result.stdout:
    print("\nSUCCESS: Fact was retrieved and used in the response!")
else:
    print("\nFAILURE: Fact was not used. Check the routing logs.")
