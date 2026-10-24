<script setup>
defineProps({
  name: { type: String, required: true },
  size: { type: [Number, String], default: 18 },
})
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <template v-if="name === 'pencil'">
      <path d="M4 20h4l10.5-10.5a2.1 2.1 0 0 0-3-3L5 17v3z" />
      <path d="M13.5 6.5l3 3" />
    </template>
    <template v-else-if="name === 'search'">
      <circle cx="11" cy="11" r="6.5" />
      <path d="M16 16l4 4" />
    </template>
    <template v-else-if="name === 'home'">
      <path d="M4 11.5 12 5l8 6.5" />
      <path d="M7 10.5V19h10v-8.5" />
    </template>
    <template v-else-if="name === 'more'">
      <circle cx="6" cy="12" r="1" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1" fill="currentColor" stroke="none" />
      <circle cx="18" cy="12" r="1" fill="currentColor" stroke="none" />
    </template>
    <template v-else-if="name === 'chevron'">
      <path d="M7 10l5 5 5-5" />
    </template>
    <template v-else-if="name === 'moon'">
      <path d="M16.5 14.5A6.5 6.5 0 0 1 10 6a6.8 6.8 0 1 0 6.5 8.5z" />
    </template>
    <template v-else-if="name === 'sun'">
      <circle cx="12" cy="12" r="3.2" />
      <path d="M12 3.5v2.2M12 18.3v2.2M3.5 12h2.2M18.3 12h2.2M6 6l1.6 1.6M16.4 16.4 18 18M18 6l-1.6 1.6M7.6 16.4 6 18" />
    </template>
    <template v-else-if="name === 'download'">
      <path d="M12 4v10" />
      <path d="M8 10.5 12 14.5 16 10.5" />
      <path d="M5 19h14" />
    </template>
    <template v-else-if="name === 'gear'">
      <circle cx="12" cy="12" r="3" />
      <path d="M12 3.5v2.2M12 18.3v2.2M3.5 12h2.2M18.3 12h2.2M6.1 6.1l1.6 1.6M16.3 16.3l1.6 1.6M17.9 6.1l-1.6 1.6M7.7 16.3 6.1 17.9" />
    </template>
    <template v-else-if="name === 'paperclip'">
      <path d="M8 12.5 14.8 5.7a3 3 0 0 1 4.2 4.2l-8.2 8.2a4.2 4.2 0 0 1-6-6l7.4-7.4" />
    </template>
    <template v-else-if="name === 'globe'">
      <circle cx="12" cy="12" r="8" />
      <path d="M4 12h16M12 4c2.2 2.4 3.3 5.1 3.3 8S14.2 17.6 12 20c-2.2-2.4-3.3-5.1-3.3-8S9.8 6.4 12 4z" />
    </template>
    <template v-else-if="name === 'spark'">
      <path d="M12 3.5 13.2 9 18.5 10.2 13.2 11.4 12 16.8 10.8 11.4 5.5 10.2 10.8 9 12 3.5z" />
      <path d="M18 14.5 18.6 16.4 20.5 17 18.6 17.6 18 19.5 17.4 17.6 15.5 17 17.4 16.4 18 14.5z" />
    </template>
    <template v-else-if="name === 'mic'">
      <rect x="9" y="3.5" width="6" height="10" rx="3" />
      <path d="M6.5 11a5.5 5.5 0 0 0 11 0M12 16.5V20" />
    </template>
    <template v-else-if="name === 'arrow-up'">
      <path d="M12 18V6" />
      <path d="M7 10.5 12 5.5l5 5" />
    </template>
    <template v-else-if="name === 'stop'">
      <rect x="7" y="7" width="10" height="10" rx="2" fill="currentColor" stroke="none" />
    </template>
    <template v-else-if="name === 'copy'">
      <rect x="8" y="8" width="10" height="12" rx="2" />
      <path d="M6 15V6.5A1.5 1.5 0 0 1 7.5 5H15" />
    </template>
    <template v-else-if="name === 'refresh'">
      <path d="M19 12a7 7 0 1 1-2-4.9" />
      <path d="M19 5v4h-4" />
    </template>
    <template v-else-if="name === 'check'">
      <path d="M5 12.5 9.2 17 19 7" />
    </template>
    <template v-else-if="name === 'close'">
      <path d="M7 7l10 10M17 7 7 17" />
    </template>
    <template v-else-if="name === 'menu'">
      <path d="M4 7h16M4 12h16M4 17h16" />
    </template>
  </svg>
</template>
