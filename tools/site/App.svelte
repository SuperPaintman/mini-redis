<script lang="ts">
  import { writable } from 'svelte/store';
  import { Router, Route, navigate } from 'svelte-routing';
  import ProgressBar from './components/ProgressBar.svelte';
  import Page from './components/Page.svelte';
  import Sidebar from './components/Sidebar.svelte';
  import SidebarTitle from './components/SidebarTitle.svelte';
  import SidebarList from './components/SidebarList.svelte';
  import SidebarItem from './components/SidebarItem.svelte';
  import SidebarDivider from './components/SidebarDivider.svelte';
  import ChapterLoader from './components/ChapterLoader.svelte';

  export let url = '';

  const progressBarActive = writable(false);

  const sidebar = writable<{
    title?: string;
    headings?: Array<{
      text: string;
      slug: string;
    }>;
  }>({});

  function handleSidebarTitleClick(e: MouseEvent): void {
    e.preventDefault();

    window.scrollTo(0, 0);

    if (location.hash === '' || location.hash === '#') {
      return;
    }

    navigate(location.pathname + location.search); // Remove the hash.
  }
</script>

<div class="root">
  <div class="progress-bar">
    <ProgressBar active={$progressBarActive} />
  </div>

  <Router {url}>
    <Sidebar logoURL="table-of-contents">
      {#if $sidebar.title}
        <SidebarTitle>
          <a href="#top" on:click={handleSidebarTitleClick}>{$sidebar.title}</a>
        </SidebarTitle>

        <SidebarDivider />
      {/if}

      {#if $sidebar.headings && $sidebar.headings.length > 0}
        <SidebarList>
          {#each $sidebar.headings as heading (heading.slug)}
            <SidebarItem>
              <a href="#{heading.slug}">{heading.text}</a>
            </SidebarItem>
          {/each}
        </SidebarList>

        <SidebarDivider />
      {/if}
    </Sidebar>

    <Page>
      <Route path="/">
        {@const _ = progressBarActive.set(false)}
        <div>Index</div>
      </Route>
      <Route path="/:slug" let:params>
        <ChapterLoader slug={params.slug} let:loading let:result>
          {@const _1 = progressBarActive.set(loading)}

          {#if result.type === 'none'}
            <!-- Loading... -->
          {:else if result.type === 'ok'}
            {@const _2 = sidebar.set(result.value.meta)}
            <!-- svelte-ignore a11y-missing-content -->
            <article>
              <h1 href="#top">{result.value.meta.title}</h1>

              {@html result.value.html}
            </article>
          {:else}
            <pre><code>{'' + result.value}</code></pre>
          {/if}
        </ChapterLoader>
      </Route>
      <Route path="*">
        {@const _ = progressBarActive.set(false)}
        <div>404</div>
      </Route>
    </Page>
  </Router>
</div>

<style lang="stylus">
  @import "./constants.styl";

  .root :global {
    @import "./prism-theme.styl";

    pre > code .highlighted {
      margin: -2px -12px;
      padding: 2px 10px;

      border-left: solid 2px #dad8d6;
      border-right: solid 2px #dad8d6;
    }
  }

  .progress-bar {
    position: fixed;

    top: 0;
    left: 0;

    width: 100%;
    height: 2px;

    z-index: $progress-bar-z-index;
  }
</style>
