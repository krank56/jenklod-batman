<script setup lang="ts">
import { withBase } from 'vitepress'

// A terminal recording from docs/public/media, made with `make docs-media`.
const props = defineProps<{ name: string; caption?: string }>()
const src = withBase(`/media/${props.name}.mp4`)
</script>

<template>
  <figure class="demo">
    <video :src="src" autoplay loop muted playsinline preload="metadata" :aria-label="caption || name" />
    <figcaption v-if="caption">{{ caption }}</figcaption>
  </figure>
</template>

<style scoped>
.demo {
  margin: 24px 0;
}
.demo video {
  display: block;
  width: 100%;
  height: auto;
  border-radius: 10px;
  border: 1px solid var(--vp-c-divider);
  background: #11131b;
}
.demo figcaption {
  margin-top: 8px;
  font-size: 14px;
  color: var(--vp-c-text-2);
  text-align: center;
}
</style>
