'use strict';
/* Imports */
import * as Prism from 'prismjs';
import './';

/* Types */
type Token = string | { type: string; content: Token | Token[] };

/* Helpers */
const raw = String.raw;
const token = (type: string, content: Token | Token[]): Token => ({
  type,
  content
});

describe('Prism', () => {
  describe('Redis', () => {
    it('should work', () => {
      Prism.tokenize(raw`+OK\r\n`, Prism.languages.redis);
    });

    it('should parse a simple string', () => {
      const tokens = Prism.tokenize(raw`+OK\r\n`, Prism.languages.redis);

      expect(tokens).toMatchObject([
        token('string', [
          token('keyword', '+'),
          'OK',
          token('punctuation', '\\r\\n')
        ])
      ]);
    });

    it('should parse an error', () => {
      const tokens = Prism.tokenize(
        raw`-ERR Protocol error: expected '$', got ' '\r\n`,
        Prism.languages.redis
      );

      expect(tokens).toMatchObject([
        token('string', [
          token('keyword', '-'),
          `ERR Protocol error: expected '$', got ' '`,
          token('punctuation', '\\r\\n')
        ])
      ]);
    });

    it('should parse an integer', () => {
      const tokens = Prism.tokenize(raw`:1337\r\n`, Prism.languages.redis);

      expect(tokens).toMatchObject([
        token('number', [
          token('keyword', ':'),
          '1337',
          token('punctuation', '\\r\\n')
        ])
      ]);
    });

    it('should parse a negative integer', () => {
      const tokens = Prism.tokenize(raw`:-1337\r\n`, Prism.languages.redis);

      expect(tokens).toMatchObject([
        token('number', [
          token('keyword', ':'),
          '-1337',
          token('punctuation', '\\r\\n')
        ])
      ]);
    });

    it('should parse a bulk string', () => {
      const tokens = Prism.tokenize(raw`$3\r\nGET\r\n`, Prism.languages.redis);

      expect(tokens).toMatchObject([
        token('number', [
          token('keyword', '$'),
          '3',
          token('punctuation', '\\r\\n')
        ]),
        token('string', ['GET', token('punctuation', '\\r\\n')])
      ]);
    });

    it('should parse an array', () => {
      const tokens = Prism.tokenize(
        raw`*2\r\n:10\r\n:20\r\n`,
        Prism.languages.redis
      );

      expect(tokens).toMatchObject([
        token('number', [
          token('keyword', '*'),
          '2',
          token('punctuation', '\\r\\n')
        ]),
        token('number', [
          token('keyword', ':'),
          '10',
          token('punctuation', '\\r\\n')
        ]),
        token('number', [
          token('keyword', ':'),
          '20',
          token('punctuation', '\\r\\n')
        ])
      ]);
    });

    it('should parse multiple lines', () => {
      const tokens = Prism.tokenize(
        [
          raw`*3\r\n`,
          raw`$3\r\n`,
          raw`SET\r\n`,
          raw`$5\r\n`,
          raw`mykey\r\n`,
          raw`$7\r\n`,
          raw`myvalue\r\n`
        ].join('\n'),
        Prism.languages.redis
      );

      expect(tokens).toMatchObject([
        token('number', [
          token('keyword', '*'),
          '3',
          token('punctuation', '\\r\\n')
        ]),
        '\n',
        token('number', [
          token('keyword', '$'),
          '3',
          token('punctuation', '\\r\\n')
        ]),
        '\n',
        token('string', ['SET', token('punctuation', '\\r\\n')]),
        '\n',
        token('number', [
          token('keyword', '$'),
          '5',
          token('punctuation', '\\r\\n')
        ]),
        '\n',
        token('string', ['mykey', token('punctuation', '\\r\\n')]),
        '\n',
        token('number', [
          token('keyword', '$'),
          '7',
          token('punctuation', '\\r\\n')
        ]),
        '\n',
        token('string', ['myvalue', token('punctuation', '\\r\\n')])
      ]);
    });
  });
});
