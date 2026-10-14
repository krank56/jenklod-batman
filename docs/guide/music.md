# Theme music

*Gotham by Night*, a keygen-style chiptune, plays from launch and loops while you watch your builds. <kbd>M</kbd> stops it, and <kbd>M</kbd> again starts it from the top.

<audio controls preload="none" src="/media/gotham.ogg"></audio>

It's an original tune in D minor at 136 BPM, about a minute and a half before it loops. A rising minor sixth over an octave-jumping bass, then a slower rooftop section. jenklod-batman synthesizes it as it plays, the way a Commodore 64's SID chip would: pulse and sawtooth voices, a resonant filter sweep on the bass, noise drums and a touch of echo. Nothing is downloaded or stored.

While it plays, the header shows `♫` and the cat in the status bar stops to dance to the beat (unless <kbd>a</kbd> has stilled it).

## Volume

<kbd>+</kbd> (or <kbd>=</kbd>) and <kbd>-</kbd> change the volume in steps of 10%, while the music plays. It starts at 50%; your system volume does the rest.

## Saved

Music is on until you turn it off. <kbd>M</kbd> and the volume keys save their setting as `music` and `music_volume` in the [config](/reference/config), so music you stop stays stopped at the next launch.

If there's no sound device, as on a headless server, the app carries on silently. If you turned the music on yourself, the status bar also says *Music unavailable* and why. The setting stays as it was, ready for a machine with speakers.

On Linux the music plays through ALSA, which the binary needs installed. See [Install](./install#linux-alsa).
