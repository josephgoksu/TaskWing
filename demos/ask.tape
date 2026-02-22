# Demo: taskwing ask --answer
# Search project knowledge with AI-generated answer

Require taskwing

Output demos/ask.gif
Output demos/ask.mp4

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

Type@60ms 'taskwing ask "why did we choose SQLite over Postgres?" --answer'
Enter
Sleep 20s

Sleep 3s
