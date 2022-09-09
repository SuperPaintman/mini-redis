'use strict';
/* Imports */
import * as Prism from 'prismjs';

Prism.languages.redis = {
  number: {
    pattern: /(\$|\*|:)\-?\d+(?:\\r\\n)/,
    inside: {
      keyword: /^(\$|\*|:)/,
      punctuation: /\\r\\n$/
    }
  },
  string: [
    {
      pattern: /(\+|-).+(?:\\r\\n)/,
      inside: {
        keyword: /^(\+|-)/,
        punctuation: /\\r\\n$/
      }
    },
    {
      pattern: /.+(?:\\r\\n)/,
      inside: {
        punctuation: /\\r\\n$/
      }
    }
  ],
  punctuation: /\\r\\n/
};
