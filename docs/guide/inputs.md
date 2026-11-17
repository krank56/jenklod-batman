# Pipeline inputs

Many pipelines stop on an `input` step before the production deploy. jenklod-batman shows where a build is waiting and lets you answer.

- **Watched jobs** show `⏸ INPUT` and the question.
- **The builds screen** shows the paused build as `⏸ INPUT`, with a banner: `#6 is waiting: Deploy to prod? — press i`.
- **The log** shows the same banner while you follow the console.

![The input prompt: Deploy to prod?](/media/input-builds.png)

<kbd>i</kbd> opens the prompt, with any fields the step defines. <kbd>enter</kbd> runs its proceed action (for example *Deploy*), and <kbd>ctrl+x</kbd> (or <kbd>x</kbd>) aborts it. Both ask y/n first. <kbd>esc</kbd> leaves it for later.

<Demo name="input" caption="A watched deploy pauses on 'Deploy to prod?'; approve it and follow the log to the end." />

## Answering ahead

A deploy can take a while to reach its question. <kbd>i</kbd> on a build that is running but not waiting yet (in the builds list, the log, or a watched job's row) answers it ahead of time:

- The app reads the consoles of the job's last 3 finished builds and proposes the answer given last time, for example *Last time (#41): Deploy to prod? → Deploy, by Bruce Wayne*. If it finds none, it proposes Proceed for whatever the build asks first.
- <kbd>←</kbd>/<kbd>→</kbd> switches between Proceed and Abort, <kbd>enter</kbd> arms it, and <kbd>esc</kbd> cancels. Nothing is sent to Jenkins yet, so there's no y/n.
- The build is polled on the watch interval, even if its job isn't watched. Its row and its log show `⏵ armed`.
- When it pauses, the app answers its **first** input step and disarms. A later input waits for you as usual.

The input waits for you instead, with a notification, when:

- it asks something other than the proposed question;
- Proceed needs a field that has no default, or a password. Otherwise the step's defaults are sent.

You get a desktop notification when the answer is sent, and when the build finishes without asking. <kbd>i</kbd> on an armed build shows it again, and <kbd>d</kbd> disarms it.

Pre-answers last only while the app is open. <kbd>q</kbd> asks before quitting when some are armed.

## Requirements

Listing pending inputs uses the API of the **Pipeline: Stage View** plugin (`wfapi`). Without that plugin, <kbd>i</kbd> says so, and <kbd>O</kbd> opens the build in the browser to answer it there.

Jenkins still enforces the step's `submitter` restriction. If you aren't allowed to approve, you'll see a "forbidden" error.
