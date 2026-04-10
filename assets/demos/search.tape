# brain search — instant retrieval demo.
#
# The "no LLM, just grounded retrieval" pitch. Shows that brain's
# value starts before any model call — qmd returns scored chunks from
# your notes in milliseconds, with inline previews. This is what
# powers the "no context → no LLM → no hallucination" core principle.

Output "assets/demos/search.gif"

Set Shell "zsh"
Set FontSize 18
Set Width 1280
Set Height 680
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set Padding 40

Hide
Type "cd /Users/ugurcanaytar/Desktop/Projects/BrainGo/brain"
Enter
Type "export PATH=$PWD:$PATH"
Enter
Type "clear"
Enter
Show

Sleep 500ms
Type `brain search -c brain-demo "CRDTs and CAP theorem tradeoffs"`
Sleep 700ms
Enter
Sleep 5s
