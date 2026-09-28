# Macros from a shell: list them, run one with a parameter override.
Source docs/tapes/settings.tape
Set Height 620
Output docs/public/media/cli.mp4

Hide
Type "clear" Enter
Show
Sleep 500ms
Type "jenklod-batman run --list" Enter
Sleep 2s
Type "jenklod-batman run ship ENV=prod -y" Enter
Sleep 26s
Screenshot docs/public/media/cli.png
Sleep 2s
