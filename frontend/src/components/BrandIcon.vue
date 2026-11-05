<template>
  <img
    class="brand-mark"
    src="/brand/icon-32.png"
    srcset="/brand/icon-32.png 1x, /brand/icon-64.png 2x, /brand/icon-96.png 3x"
    width="32"
    height="32"
    alt=""
    aria-hidden="true"
    draggable="false"
  />
</template>
<style scoped>
/* Preserve source cascade and contextual sizing; shared tokens remain global. */
.brand-mark {
  display: block;
  object-fit: contain;
  width: 30px;
  height: 30px;
  flex: none;
}
.brand-mark {
  display: block;
  object-fit: contain;
  background: transparent;
  border-radius: 0;
  clip-path: none;
}
.nav-wordmark .brand-mark {
  margin-right: 8px;
  width: 26px;
  height: 26px;
}
.overlay-title .brand-mark {
  width: 25px;
  height: 25px;
}
@media (max-width: 999px) {
  .nav-wordmark .brand-mark {
    width: 22px;
    height: 22px;
    font-size: 18px;
  }
}
.sidebar-header .brand-mark {width:23px;height:23px;}
.sidebar-rail .brand-mark {width:21px;height:21px;}
</style>
