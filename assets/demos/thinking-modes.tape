# brain thinking modes — the adaptive prompt system in action.
#
# Asks the same topic four different ways, once per mode, so the
# structural difference between modes (DIRECT ANSWER vs KEY FINDINGS
# vs RELEVANT FRAMEWORKS vs BUILDING BLOCKS) is visible side by side.
# Uses haiku so every response finishes in a few seconds.

Output "assets/demos/thinking-modes.gif"

Set Shell "zsh"
Set FontSize 18
Set Width 1280
Set Height 760
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set Padding 40
Set Framerate 24
Set PlaybackSpeed 1.2

Hide
Type "cd /Users/ugurcanaytar/Desktop/Projects/BrainGo/brain"
Enter
Type "export PATH=$PWD:$PATH"
Enter
Type "clear"
Enter
Show

Sleep 400ms
Type "brain chat -c brain-demo -m haiku"
Sleep 400ms
Enter
Sleep 2s

# --- recall: direct answer + related context ---
Type "/mode recall"
Sleep 300ms
Enter
Sleep 1500ms
Type "What is a CRDT?"
Sleep 400ms
Enter
Sleep 15s

# --- analysis: findings + connections + gaps + synthesis ---
Type "/mode analysis"
Sleep 300ms
Enter
Sleep 1500ms
Type "How do CRDTs relate to CAP theorem?"
Sleep 400ms
Enter
Sleep 16s

# --- decision: frameworks + arguments + recommendation ---
Type "/mode decision"
Sleep 300ms
Enter
Sleep 1500ms
Type "Should I pick AP or CP for a chat app?"
Sleep 400ms
Enter
Sleep 16s

# --- synthesis: building blocks + action plan ---
Type "/mode synthesis"
Sleep 300ms
Enter
Sleep 1500ms
Type "How would I build a partition-tolerant system?"
Sleep 400ms
Enter
Sleep 16s

Type "/quit"
Sleep 300ms
Enter
Sleep 500ms
