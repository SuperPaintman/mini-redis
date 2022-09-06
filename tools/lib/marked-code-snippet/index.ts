'use strict';
/* Imports */
import { Workspace, MagicLineParser, MagicMarker } from '../splitter';
import type { marked } from 'marked';

/* Types */
declare module 'marked' {
  namespace marked {
    namespace Tokens {
      type TokenCodeSnippet = {
        type: 'codeSnippet';
        raw: string;
        marker: MagicMarker;
        kind: string;
        args: string[];
        options: Record<string, string | null>;
      };
    }
  }
}

export type Options = {
  format?(code: string, lang: string): string;
  highlight?(code: string, lang: string): string;
};

/* Helpers */
const quote = (s: string): string => JSON.stringify(s);
const defaultFormat = (code: string, _lang: string) => code;
const defaultHighlighter = (code: string, _lang: string) => code;

export function codeSnippet(
  workspace: Workspace,
  options: Options = {}
): marked.TokenizerExtension & marked.RendererExtension {
  const { format = defaultFormat, highlight = defaultHighlighter } = options;

  return {
    name: 'codeSnippet',
    level: 'block',
    start: (src) => src.match(/\^snippet\s/)?.index,
    tokenizer(src, _tokens): marked.Tokens.TokenCodeSnippet | void {
      const match = src.match(/^(\^snippet\s+.+)(?:\n|$)/);
      if (!match) {
        return;
      }

      const magicLine = new MagicLineParser(match[1].slice(1)).parse(
        MagicMarker.Line
      );
      if (!magicLine) {
        return;
      }

      return {
        type: 'codeSnippet',
        raw: match[1],
        ...magicLine
      };
    },
    renderer(rawToken: marked.Tokens.Generic | marked.Tokens.TokenCodeSnippet) {
      if (rawToken.type !== 'codeSnippet') {
        return false;
      }

      const token: marked.Tokens.TokenCodeSnippet =
        rawToken as marked.Tokens.TokenCodeSnippet;

      if (token.kind !== 'snippet') {
        throw new Error(`Unknown magic line kind: ${quote(token.kind)}`);
      }

      if (token.args.length !== 1) {
        throw new Error(
          `Wrong arity for ${quote(token.kind)}: got ${
            token.args.length
          }, want 1`
        );
      }

      const name = token.args[0];

      const file = workspace.file(name);
      if (!file) {
        throw new Error(`File for the ${quote(name)} snippet is not found`);
      }

      file.enable(name);

      const pos = file.snippetLines(name);
      if (!pos) {
        throw new Error(`Position for the ${quote(name)} snippet is not found`);
      }

      const [start, end] = pos;

      const before = token.options.before
        ? parseInt(token.options.before, 10)
        : 0;
      if (isNaN(before)) {
        throw new Error(
          `Broken before option: ${quote(token.options.before || '<null>')}`
        );
      }

      const after = token.options.after ? parseInt(token.options.after, 10) : 0;
      if (isNaN(after)) {
        throw new Error(
          `Broken after option: ${quote(token.options.after || '<null>')}`
        );
      }

      const language = file.language();

      const lines = format(file.lines().join('\n'), language);

      const snippet = lines
        .split('\n')
        .slice(start - before, end + after)
        .join('\n');

      const code = highlight(snippet, language);

      // Build a HTML string.
      let res: string = '';

      if (before === 0 && after === 0) {
        res += `<pre><code class="language-${language}">${code}</code></pre>`;
      } else {
        const codeLines = code.split('\n');
        const linesBefore = codeLines.slice(0, before);
        const linesHighlighted = codeLines.slice(before, -1 * after);
        const linesAfter = codeLines.slice(-1 * after);

        res += `<pre><code class="language-${language}">`;
        if (linesBefore.length > 0) {
          res += `<div class="dimmed">${linesBefore.join('\n')}\n</div>`;
        }

        res += `<div class="highlighted">${linesHighlighted.join(
          '\n'
        )}\n</div>`;

        if (linesAfter.length > 0) {
          res += `<div class="dimmed">${linesAfter.join('\n')}\n</div>`;
        }

        res += '</pre></code>';
      }

      res += `<div><em>${file.path()}</em></div>`;

      return res;
    }
  };
}
