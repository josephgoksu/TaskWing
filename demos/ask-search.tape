# Demo: taskwing ask (search only)
# Fast architecture search without LLM

Require taskwing

Output demos/ask-search.gif
Output demos/ask-search.mp4

Set Shell "zsh"
Set FontSize 18
Set Width 1200
Set Height 800
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 50ms
Set WindowBar Colorful
Set WindowBarSize 40
Set Framerate 24
Set PlaybackSpeed 1
Set LoopOffset 0%

Sleep 1s

Type@60ms 'taskwing ask "why did we choose SQLite over Postgres?"'
Enter
Sleep 8s

Sleep 3s
