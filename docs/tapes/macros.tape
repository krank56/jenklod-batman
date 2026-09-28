# Build a macro in the editor (a dry-run deploy, then wait), run it, watch it.
Source docs/tapes/settings.tape
Output docs/public/media/macros.mp4

Hide
Type "clear && jenklod-batman" Enter
Sleep 1.5s
Space
Sleep 1s
Show
Sleep 1s
Type "m"
Sleep 1.2s
Type "n"
Sleep 500ms
Type "dry-deploy"
Sleep 400ms
Enter
Sleep 600ms
Type "a"
Sleep 1s
Type "b"
Sleep 400ms
Type "gdep"
Sleep 800ms
Enter
Sleep 1.2s
Tab
Sleep 400ms
Space
Sleep 1s
Enter
Sleep 800ms
Type "a"
Sleep 400ms
Type "w"
Sleep 400ms
Type "gdep"
Sleep 600ms
Enter
Sleep 1.5s
Screenshot docs/public/media/macro-editor.png
Ctrl+S
Sleep 1.5s
Down
Sleep 400ms
Enter
Sleep 1.5s
Type "y"
Sleep 8s
Screenshot docs/public/media/macro-running.png
Sleep 21s
Screenshot docs/public/media/macro-done.png
Sleep 1s
Type "q"
Sleep 300ms
Type "q"
Sleep 500ms
