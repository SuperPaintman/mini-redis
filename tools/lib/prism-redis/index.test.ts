'use strict';
/* Imports */
import * as Prism from 'prismjs';
import './';

/* Types */
type Token = string | { type: string; content: Token | Token[] };

/* Helpers */
const raw = String.raw;
const tokenize = (text: string) => Prism.tokenize(text, Prism.languages.redis);
const token = (type: string, content: Token | Token[]): Token => ({
  type,
  content
});
const string = (content: Token | Token[]) => token('string', content);
const number = (content: Token | Token[]) => token('number', content);
const keyword = (content: Token | Token[]) => token('keyword', content);
const punctuation = (content: Token | Token[]) => token('punctuation', content);
const crlf = punctuation(raw`\r\n`);

describe('Prism', () => {
  describe('Redis', () => {
    it('should work', () => {
      Prism.tokenize(raw`+OK\r\n`, Prism.languages.redis);
    });

    it('should tokenize a simple string', () => {
      const tokens = tokenize(raw`+OK\r\n`);

      expect(tokens).toMatchObject([string([keyword('+'), 'OK', crlf])]);
    });

    it('should tokenize an error', () => {
      const tokens = tokenize(
        raw`-ERR Protocol error: expected '$', got ' '\r\n`
      );

      expect(tokens).toMatchObject([
        string([
          keyword('-'),
          `ERR Protocol error: expected '$', got ' '`,
          crlf
        ])
      ]);
    });

    it('should tokenize an integer', () => {
      const tokens = tokenize(raw`:1337\r\n`);

      expect(tokens).toMatchObject([number([keyword(':'), '1337', crlf])]);
    });

    it('should tokenize a negative integer', () => {
      const tokens = tokenize(raw`:-1337\r\n`);

      expect(tokens).toMatchObject([number([keyword(':'), '-1337', crlf])]);
    });

    it('should tokenize a bulk string', () => {
      const tokens = tokenize(raw`$3\r\nGET\r\n`);

      expect(tokens).toMatchObject([
        number([keyword('$'), '3', crlf]),
        string(['GET', crlf])
      ]);
    });

    it('should tokenize an array', () => {
      const tokens = tokenize(raw`*2\r\n:10\r\n:20\r\n`);

      expect(tokens).toMatchObject([
        number([keyword('*'), '2', crlf]),
        number([keyword(':'), '10', crlf]),
        number([keyword(':'), '20', crlf])
      ]);
    });

    it('should tokenize multiple lines', () => {
      const tokens = tokenize(
        [
          raw`*3\r\n`,
          raw`$3\r\n`,
          raw`SET\r\n`,
          raw`$5\r\n`,
          raw`mykey\r\n`,
          raw`$7\r\n`,
          raw`myvalue\r\n`
        ].join('\n')
      );

      expect(tokens).toMatchObject([
        number([keyword('*'), '3', crlf]),
        '\n',
        number([keyword('$'), '3', crlf]),
        '\n',
        string(['SET', crlf]),
        '\n',
        number([keyword('$'), '5', crlf]),
        '\n',
        string(['mykey', crlf]),
        '\n',
        number([keyword('$'), '7', crlf]),
        '\n',
        string(['myvalue', crlf])
      ]);
    });
  });
});
