# brain TUI pickers — huh collection multi-select + /model picker.
#
# Pure TUI showcase, no LLM calls. Shows the two interactive pickers
# that back the "it's a TUI, not a thin CLI wrapper" claim:
#   1. the huh multi-select that pops when `brain chat` starts
#   2. the /model picker available mid-session

Output "assets/demos/pickers.gif"

Set Shell "zsh"
Set FontSize 18
Set Width 1280
Set Height 760
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set Padding 40
Set Framerate 24

Hide
Type "cd /Users/ugurcanaytar/Desktop/Projects/BrainGo/brain"
Enter
Type "export PATH=$PWD:$PATH"
Enter
Type "clear"
Enter
Show

Sleep 500ms
Type "brain chat"
Sleep 400ms
Enter
Sleep 2s

# Collection picker: scroll down a bit to show it's interactive,
# then select "All collections" (the default entry) and confirm.
Down@100ms 3
Sleep 800ms
Up@100ms 3
Sleep 600ms
Space
Sleep 700ms
Enter
Sleep 2s

# Model picker: bare /model opens the huh select.
Type "/model"
Sleep 400ms
Enter
Sleep 1500ms

# Walk through the three model options so the viewer sees them.
Down@150ms 2
Sleep 700ms
Up@150ms 1
Sleep 700ms
Enter
Sleep 1500ms

Type "/quit"
Sleep 300ms
Enter
Sleep 500ms
