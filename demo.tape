Output docs/assets/demo.gif

Require ./bin/agents

Set FontSize 18
Set Width 1500
Set Height 800
Set TypingSpeed 75ms

Hide
Type "export AGENTS_DRY_RUN=true && ./bin/agents"
Enter
Sleep 1s
Show

Sleep 2s
Type "/skills"
Sleep 500ms
Enter

Sleep 3s
Type "/model list"
Sleep 500ms
Enter

Sleep 2s
Down 2
Sleep 500ms
Enter

Sleep 3s
Type "I want to build a local python ADK agent."
Sleep 500ms
Enter

Sleep 2s
Type "Great. Now run the tests for it."
Sleep 500ms
Enter

Sleep 2s
Type "Looks good! Please deploy it to Google Agent Engine."
Sleep 500ms
Enter

Sleep 5s
