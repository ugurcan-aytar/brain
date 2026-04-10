# brain /challenge — the cross-reference power move.
#
# Ask a question against one collection, then /challenge to re-score
# the same question against a DIFFERENT collection (with a different
# perspective). brain rebuilds the system prompt around the new
# sources and streams a re-grounded answer. Great way to stress-test
# a conclusion before you commit to it.
#
# The tape uses two demo collections:
#   - brain-demo       — academic/textbook notes on Raft, CAP, CRDTs
#   - brain-demo-v2    — practitioner notes (Paxos tradeoffs, eventual
#                        consistency in production)

Output "assets/demos/challenge.gif"

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

Sleep 400ms
Type "brain chat -c brain-demo -m haiku"
Sleep 400ms
Enter
Sleep 2s

Type "/mode recall"
Sleep 300ms
Enter
Sleep 1500ms

Type "Should I use Raft for my system?"
Sleep 500ms
Enter
Sleep 22s

Type "/challenge"
Sleep 400ms
Enter
Sleep 2500ms

# brain-demo-v2 is the last collection added, so it sits at the
# bottom of the picker. 19 Down presses walk past "All collections"
# and every registered collection to reach it.
# huh clamps at the end of the list, so Down 30 reliably lands on
# the very last item (brain-demo-v2) no matter the exact count.
Down@60ms 30
Sleep 800ms
Space
Sleep 800ms
Enter
Sleep 30s

Type "/quit"
Sleep 300ms
Enter
Sleep 500ms
