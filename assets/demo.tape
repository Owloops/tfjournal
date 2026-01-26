# VHS Demo for tfjournal

# https://github.com/charmbracelet/vhs

Output demo.gif
Set FontSize 16
Set Width 1280
Set Height 720
Set Theme "Catppuccin Mocha"

# Start with a clean terminal
Type "# tfjournal - Record Terraform runs with git context"
Sleep 1s
Enter
Sleep 500ms

# Launch tfjournal TUI
Type "./tfjournal"
Sleep 500ms
Enter
Sleep 3s

# Navigate through runs
Down
Sleep 400ms

# Select a run to view details
Enter
Sleep 2s

# Show Events tab
Type "e"
Sleep 2s

# Show Timeline tab
Type "t"
Sleep 2s

# Show Output tab
Type "o"
Sleep 2s

# Scroll through output
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms

# Go back to list
Escape
Sleep 1s

# Show filtering with /
Type "/"
Sleep 500ms
Type "failed"
Sleep 1s
Enter
Sleep 2s

# Clear filter
Escape
Sleep 1s

# Exit
Type "q"
Sleep 1s
