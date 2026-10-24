<script setup>
defineProps({
  size: { type: [Number, String], default: 28 },
})
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 64 64" fill="currentColor" aria-hidden="true">
    <path d="M33 8.2c.8 7.6 3.3 11.2 8.4 13.6-5.1 2.3-7.6 6-8.4 13.6-.8-7.6-3.3-11.3-8.4-13.6 5.1-2.4 7.6-6 8.4-13.6z" />
    <path d="M20.2 36.2c.5 4.6 2 6.8 5.1 8.2-3.1 1.4-4.6 3.6-5.1 8.2-.5-4.6-2-6.8-5.1-8.2 3.1-1.4 4.6-3.6 5.1-8.2z" />
    <path d="M45.6 38.4c.4 3.8 1.6 5.6 4.1 6.8-2.5 1.2-3.7 3-4.1 6.8-.4-3.8-1.6-5.6-4.1-6.8 2.5-1.2 3.7-3 4.1-6.8z" />
  </svg>
</template>
