<script lang="ts">
  import { writable } from 'svelte/store';

  type Chapter = {
    meta: {
      title: string;
      slug: string;
      draft: boolean;
      headings: Array<{ text: string; slug: string }>;
    };
    html: string;
  };

  type Result =
    | { type: 'none' }
    | { type: 'ok'; value: Chapter }
    | { type: 'error'; value: unknown };

  export let slug: string;

  let loading = false;
  // We keep track of the request sequence to prevent the page blinking
  // and breaking the order.
  let requestID = 0;
  const result = writable<Result>({
    type: 'none'
  });

  async function loadChapter(slug: string): Promise<void> {
    const rid = ++requestID;
    loading = true;

    try {
      const response = await fetch(`/api/chapters/${slug}.json`);
      if (response.status >= 400 && response.status <= 500) {
        throw response;
      }

      const value = await response.json();

      if (rid === requestID) {
        result.set({
          type: 'ok',
          value
        });
      }
    } catch (err) {
      if (rid === requestID) {
        result.set({
          type: 'error',
          value: err
        });
      }
    } finally {
      if (rid === requestID) {
        loading = false;
      }
    }
  }

  $: loadChapter(slug);
</script>

<slot {loading} result={$result} />
