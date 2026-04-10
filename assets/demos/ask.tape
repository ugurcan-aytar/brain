# brain ask — hero demo.
#
# Shows the one-shot Q&A flow: retrieval spinner, streaming markdown
# answer, cited sources, closing logo. Uses the pre-seeded `brain-demo`
# collection (see assets/demo-notes/) and the claude CLI fallback so the
# recording runs without an API key.
#
# Regenerate with:
#   cd assets/demos && vhs ask.tape
#
# Prereqs (see README > Contributing):
#   - brew install vhs ttyd
#   - go build -o brain ./cmd/brain    (at repo root)
#   - ./brain add assets/demo-notes --name brain-demo

Output assets/demos/ask.gif

Set Shell "zsh"
Set FontSize 18
Set Width 1280
Set Height 720
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms
Set Padding 40

Hide
Type "cd /Users/ugurcanaytar/Desktop/Projects/BrainGo/brain"
Enter
Type "clear"
Enter
Show

Sleep 500ms
Type `./brain ask -c brain-demo -M recall "What's the difference between CP and AP systems?"`
Sleep 800ms
Enter
Sleep 25s
