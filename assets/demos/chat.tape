# brain chat — REPL demo.
#
# Shows the interactive multi-turn REPL: slash command help, mode
# switching, a grounded answer to a short question, clean exit. Uses
# the -c flag (added for this demo) to skip the collection picker so
# the tape is reproducible regardless of what collections the recorder
# has registered.

Output "assets/demos/chat.gif"

Set Shell "zsh"
Set FontSize 18
Set Width 1280
Set Height 760
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set Padding 40

Hide
Type "cd /Users/ugurcanaytar/Desktop/Projects/BrainGo/brain"
Enter
Type "clear"
Enter
Show

Sleep 500ms
Type "./brain chat -c brain-demo"
Sleep 500ms
Enter
Sleep 2s

Type "/help"
Sleep 400ms
Enter
Sleep 2500ms

Type "/mode recall"
Sleep 300ms
Enter
Sleep 1500ms

Type "What are CRDTs good for?"
Sleep 500ms
Enter
Sleep 22s

Type "/quit"
Sleep 300ms
Enter
Sleep 500ms
