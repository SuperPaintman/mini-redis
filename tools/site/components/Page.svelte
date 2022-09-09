<div class="page">
  <div class="content">
    <slot />
  </div>
</div>

<style lang="stylus">
  @import "../constants.styl";

  .page {
    position: relative;

    margin-left: $sidebar-widht + 48px;
    margin-right: @margin-left;
  }

  .content {
    width: 912px;

    margin: auto;

    overflow: hidden;
  }
</style>
