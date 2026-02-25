Output docs/assets/demo.gif

Require ./bin/agents

Set Margin 20
Set MarginFill "#6B50FF"
Set BorderRadius 10
Set WindowBar Colorful
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
Type "/help"
Sleep 500ms
Enter

Sleep 3s
Type "/clear"
Sleep 500ms
Enter

Sleep 1s
Type "I need to build a new Python LangGraph agent."
Sleep 1s
Enter

Sleep 2s
Type "Great. Now please deploy it to Google Agent Engine using GitHub Actions."
Sleep 1s
Enter

Sleep 3s
