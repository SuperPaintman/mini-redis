<script lang="ts">
  import { link } from 'svelte-routing';

  export let logoURL = '/';
</script>

<aside class="sidebar">
  <div class="content">
    <a class="logo" href={logoURL} use:link>Mini Redis</a>

    <nav class="nav">
      <slot />
      <!-- <h3 class="title">Introduction</h3>

      <div class="divider" />

      <ul class="list">
        <li>
          <a href="#Why Learn This Stuff?">Why Learn This Stuff?</a>
        </li>
        <li>
          <a href="#How the Book Is Organized">How the Book Is Organized</a>
        </li>
        <li>
          <a href="#The First Interpreter">The First Interpreter</a>
        </li>
        <li>
          <a href="#The Second Interpreter">The Second Interpreter</a>
        </li>
      </ul>

      <div class="divider" /> -->
    </nav>
  </div>
</aside>

<style lang="stylus">
  @import "../constants.styl";

  .sidebar {
    position: fixed;

    width: $sidebar-widht;
    height: 100%;

    background: #29313d;
    color: #7aa0b8;

    font-family: "Source Sans Pro", sans-serif, serif;

    z-index: $sidebar-z-index;
  }

  .sidebar :global(a) {
    color: inherit;
    text-decoration: none;
  }

  // .sidebar h3 {
  //   margin: 0;
  //   font-size: inherit;
  //   font-weight: inherit;
  // }

  // .sidebar ul {
  //   margin: 0;
  //   padding: 0;
  //   list-style: none;
  // }

  .content {
    margin: 52px 48px;
  }

  .logo {
    display: block;

    margin: 52px 0;

    color: inherit;
    font-size: 42px;
    font-weight: 700;
    text-decoration: none;
  }

  .nav {
    margin: 24px 0;

    font-size: 16px;
    line-height: 24px;
  }

  // .nav > .divider {
  //   margin: 7px 0;

  //   border-top: solid 1px #3b4b5e;
  // }

  // .nav > .title {
  //   margin: 0;

  //   font-size: 18px;
  //   font-weight: 400;
  // }

  // .nav > .list {
  //   font-weight: 300;
  // }
</style>
