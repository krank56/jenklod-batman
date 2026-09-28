# Pipeline inputs

Many pipelines stop on an `input` step before the production deploy. jenklod-batman shows where a build is waiting and lets you answer.

- **Watched jobs** show `⏸ INPUT` and the question.
- **The builds screen** shows the paused build as `⏸ INPUT`, with a banner: `#6 is waiting: Deploy to prod? — press i`.
- **The log** shows the same banner while you follow the console.

![The input prompt: Deploy to prod?](/media/input-builds.png)

<kbd>i</kbd> opens the prompt, with any fields the step defines. <kbd>enter</kbd> runs its proceed action (for example *Deploy*), and <kbd>ctrl+x</kbd> (or <kbd>x</kbd>) aborts it. Both ask y/n first. <kbd>esc</kbd> leaves it for later.

<Demo name="input" caption="A watched deploy pauses on 'Deploy to prod?'; approve it and follow the log to the end." />

## Requirements

Listing pending inputs uses the API of the **Pipeline: Stage View** plugin (`wfapi`). Without that plugin, <kbd>i</kbd> says so, and <kbd>o</kbd> opens the build in the browser to answer it there.

Jenkins still enforces the step's `submitter` restriction. If you aren't allowed to approve, you'll see a "forbidden" error.
