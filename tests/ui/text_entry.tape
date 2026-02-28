Output tests/ui/text_entry.gif
Set Width 1000
Set Height 600
Set TypingSpeed 75ms
Hide
Type "./bin/swarm"
Enter
Sleep 1s
Show

# Test 1: Layout stability and submission
Type "Test 1: Submission layout jump check"
Sleep 500ms
Enter
Sleep 2s

# Test 2: Submission leakage (Prompt placement)
Type "Test 2: Ensure prompt cursor starts on line 1"
Sleep 500ms
Enter
Sleep 2s

# Test 3: Multiline support
Type "Test 3: Line 1"
Alt+Enter
Type "Line 2"
Sleep 500ms
Enter
Sleep 2s

# Test 4: History navigation
Up
Sleep 1s
Up
Sleep 1s
Down
Sleep 2s
