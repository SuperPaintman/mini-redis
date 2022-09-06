'use strict';
/* Imports */
import { MagicLine, MagicLineParser, MagicMarker } from './parser';

/* Tests */
describe('splitter', () => {
  describe('MagicLineParser', () => {
    it('should work', () => {
      new MagicLineParser(`//> snippet test`).parse();
    });

    it('should parser a snippet', () => {
      const line = new MagicLineParser(`//> snippet test`).parse();

      expect(line).toEqual<MagicLine>({
        marker: MagicMarker.Start,
        kind: 'snippet',
        args: ['test'],
        options: {}
      });
    });

    it('should parser a snippet with multiple args', () => {
      const line = new MagicLineParser(
        `//> snippet test2 replaces test1`
      ).parse();

      expect(line).toEqual<MagicLine>({
        marker: MagicMarker.Start,
        kind: 'snippet',
        args: ['test2', 'replaces', 'test1'],
        options: {}
      });
    });

    it('should parser a snippet with options', () => {
      const line = new MagicLineParser(
        `//^ remove-lines: before=1, after=2, test`
      ).parse();

      expect(line).toEqual<MagicLine>({
        marker: MagicMarker.Line,
        kind: 'remove-lines',
        args: [],
        options: { before: '1', after: '2', test: null }
      });
    });

    it('should ignore extra whitespaces', () => {
      const line = new MagicLineParser(
        `   //>snippet   test-2    replaces   test-1 :uncomment-lines  ,  test   =  true `
      ).parse();

      expect(line).toEqual<MagicLine>({
        marker: MagicMarker.Start,
        kind: 'snippet',
        args: ['test-2', 'replaces', 'test-1'],
        options: { 'uncomment-lines': null, test: 'true' }
      });
    });

    it('should parse quoted strings', () => {
      const line = new MagicLineParser(
        `//> snippet "  Test 2  " replaces "Test@1": test-1="  =  \\"true\\"  =  ", test-2="", test-3="  "`
      ).parse();

      expect(line).toEqual<MagicLine>({
        marker: MagicMarker.Start,
        kind: 'snippet',
        args: ['  Test 2  ', 'replaces', 'Test@1'],
        options: {
          'test-1': '  =  "true"  =  ',
          'test-2': '',
          'test-3': '  '
        }
      });
    });

    it('should parse a line with a provided marker', () => {
      const line = new MagicLineParser(`snippet test`).parse(MagicMarker.Line);

      expect(line).toEqual<MagicLine>({
        marker: MagicMarker.Line,
        kind: 'snippet',
        args: ['test'],
        options: {}
      });
    });

    it('should ignore regular lines', () => {
      const line = new MagicLineParser(`const Test = true`).parse();

      expect(line).toBeNull();
    });

    it('should ignore non-magic comment lines', () => {
      const line = new MagicLineParser(`// > hello there`).parse();

      expect(line).toBeNull();
    });
  });
});
