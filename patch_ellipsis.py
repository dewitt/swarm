import re

files = ['cmd/agents/demo_swarm.go', 'cmd/agents/interactive.go', 'pkg/sdk/manager.go']

for file_path in files:
    with open(file_path, 'r') as f:
        content = f.read()
    
    # Replace displayed ellipses. Avoiding Go variadics like "args...".
    # We look for "..." inside double quotes.
    
    # 1. Matches "something..."
    content = re.sub(r'(\w)\.\.\."', r'\1…"', content)
    # 2. Matches "...something" (less common in UI but possible)
    content = re.sub(r'"\.\.\.(\w)', r'"…\1', content)
    # 3. Matches just "..."
    content = content.replace('"... "', '"… "')
    content = content.replace('"..."', '"…"')
    # 4. Matches concat + "..."
    content = content.replace('+ "..."', '+ "…"')
    
    with open(file_path, 'w') as f:
        f.write(content)
