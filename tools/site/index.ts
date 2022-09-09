'use strict';
/* Imports */
import 'normalize.css';
import './index.styl';
import App from './App.svelte';

if (!module.parent && typeof global.document !== 'undefined') {
  new App({
    target: document.getElementById('root')!,
    hydrate: IS_SSR || false,
    props: {}
  });
}

export default App;
