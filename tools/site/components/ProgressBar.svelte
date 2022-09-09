<script lang="ts">
  import { tweened } from 'svelte/motion';
  import { cubicOut, linear } from 'svelte/easing';

  export let active = false;
  export let duration = 5000;

  const opacity = tweened(0, {
    duration: 500,
    easing: linear
  });
  const progress = tweened(0, {
    duration: 0,
    easing: cubicOut
  });

  function activate(active: boolean): void {
    if (active) {
      opacity.set(1, { duration: 0 });
      progress.set(0, { duration: 0 });
      progress.set(0.9, { duration });
    } else {
      opacity.set(0);
      progress.set(1, { duration: 300, easing: linear });
    }
  }

  $: activate(active);
</script>

<div class="root" class:active={$opacity !== 0} style="opacity: {$opacity};">
  <div class="progress" style="width: {$progress * 100}%;" />
</div>

<style lang="stylus">
  .root {
    display: none;

    position: relative;

    width: 100%;
    height: 100%;
  }

  .active {
    display: block;
  }

  .progress {
    position: absolute;

    top: 0;
    left: 0;

    width: 0%;
    height: 100%;

    background: #79b8ff; // TODO
  }
</style>
